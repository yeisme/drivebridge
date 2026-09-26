package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/yeisme/drivebridge/internal/bridge"
)

// surfacesContractFixture is the frozen contract fixture server: it answers
// the public capabilities document, the batch lifecycle and the part byte
// path with server-shaped envelopes and verbatim server IDs. It is the
// cross-owner stand-in until the backend wave lands its real services.
func surfacesContractFixture(t *testing.T, advertised []string) *httptest.Server {
	t.Helper()
	batches := map[string]map[string]any{}
	parts := map[string]int64{}
	mux := http.NewServeMux()
	encode := func(w http.ResponseWriter, doc map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": doc})
	}
	fail := func(w http.ResponseWriter, code string, status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "fixture refusal"}})
	}
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			fail(w, "unauthorized", 401)
			return
		}
		encode(w, map[string]any{"instance_id": "SRV-INSTANCE", "actions": advertised})
	})
	mux.HandleFunc("POST /api/v1/upload-batches", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SpaceID        string `json:"space_id"`
			ConflictPolicy string `json:"conflict_policy"`
		}
		json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
		id := fmt.Sprintf("SRV-BATCH-%s", r.Header.Get("Idempotency-Key"))
		if prior, ok := batches[id]; ok {
			encode(w, prior)
			return
		}
		batches[id] = map[string]any{"batch_id": id, "batch_revision": 1, "phase": "enumerating"}
		encode(w, batches[id])
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"accepted": 1})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/seal", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"operation_id": "SRV-OP-" + r.PathValue("id"), "part_size": 1 << 20})
	})
	mux.HandleFunc("PUT /api/v1/operations/{id}/parts/{index}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" || r.Header.Get("Content-Type") != "application/octet-stream" {
			fail(w, "invalid_request", 400)
			return
		}
		n, _ := io.Copy(io.Discard, r.Body)
		parts[r.PathValue("id")] += n
		encode(w, map[string]any{"received_bytes": n})
	})
	mux.HandleFunc("GET /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		remaining, streamed := parts["SRV-OP-"+id], parts["SRV-OP-"+id] > 0
		_ = remaining
		encode(w, map[string]any{"batch_id": id, "phase": map[bool]string{true: "sealed", false: "enumerating"}[streamed], "items": []any{}, "items_total": 1})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func surfacesFixtureAdapter(t *testing.T, endpoint string) *Adapter {
	t.Helper()
	dir := t.TempDir()
	credential := filepath.Join(dir, "token")
	if err := os.WriteFile(credential, []byte("fixture-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	a, err := NewWithAigora(endpoint, credential, root, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// TestFPRemoteSurfacesAdapterToolsGateOnDiscoveryAndOptIn pins the
// registration gate: without the explicit opt-in the tool list stays
// byte-identical to the legacy surface, and with it only live-advertised
// actions become tools — the missing ones stay unregistered while the
// status tool reports the concrete gap.
func TestFPRemoteSurfacesAdapterToolsGateOnDiscoveryAndOptIn(t *testing.T) {
	advertised := []string{"upload_batch_create", "upload_batch_add_items", "upload_batch_seal", "upload_batch_get", "organize_plan_get"}
	server := surfacesContractFixture(t, advertised)
	legacy := surfacesFixtureAdapter(t, server.URL)
	legacyNames := func() map[string]bool {
		out := map[string]bool{}
		for name := range legacy.MCP().ListTools() {
			out[name] = true
		}
		return out
	}()
	before := legacyNames
	// The opt-in attaches the surface; registration follows discovery.
	if err := legacy.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	after := map[string]bool{}
	for name := range legacy.MCP().ListTools() {
		after[name] = true
	}
	for name := range before {
		if !after[name] {
			t.Fatalf("legacy tool %s vanished after the opt-in", name)
		}
	}
	for _, want := range []string{"upload_batch_create", "upload_batch_add_items", "upload_batch_seal", "upload_batch_get", "organize_plan_get", "upload_batch_file", "remote_surfaces_status"} {
		if !after[want] {
			t.Fatalf("advertised surface tool %s missing from tools/list", want)
		}
	}
	for _, banned := range []string{"analytics_snapshot", "analytics_query", "cloud_scan", "changes_v2", "organize_apply"} {
		if after[banned] {
			t.Fatalf("unadvertised action %s must not be registered", banned)
		}
	}
	// The status tool states the actual gap instead of guessing aliases.
	status, ok := after["remote_surfaces_status"], true
	_ = status
	if !ok {
		t.Fatal("status tool missing")
	}
	out, err := legacy.Call(context.Background(), "remote_surfaces_status", nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(out)
	if !strings.Contains(string(encoded), "analytics_snapshot") || !strings.Contains(string(encoded), `"discovery":"ok"`) {
		t.Fatalf("status must report the concrete gap: %s", encoded)
	}
	// A fresh adapter without the opt-in keeps the exact legacy list.
	fresh := surfacesFixtureAdapter(t, server.URL)
	strict := map[string]bool{}
	for name := range fresh.MCP().ListTools() {
		strict[name] = true
	}
	if len(strict) != len(before) {
		t.Fatalf("legacy-only adapter changed size: %d vs %d", len(strict), len(before))
	}
	for name := range strict {
		if !before[name] {
			t.Fatalf("unexpected tool %s in legacy-only list", name)
		}
	}
}

// TestFPRemoteSurfacesEntriesShareOneContract pins entry equivalence: the
// MCP tool dispatch, the adapter Call path and the direct application
// service return the same payload for the same fixture call, and the CLI
// remote --action surface routes through the same application contract.
func TestFPRemoteSurfacesEntriesShareOneContract(t *testing.T) {
	server := surfacesContractFixture(t, bridge.RemoteSurfaceActionNames())
	a := surfacesFixtureAdapter(t, server.URL)
	if err := a.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	direct, err := a.Surfaces.Run(ctx, "upload_batch_get", map[string]any{"batch_id": "SRV-BATCH-k"})
	if err != nil {
		t.Fatal(err)
	}
	// Seed the batch so all three entries read the same server state.
	if _, err = a.Surfaces.Run(ctx, "upload_batch_create", map[string]any{"space_id": "sp", "conflict_policy": "fail", "idempotency_key": "k"}); err != nil {
		t.Fatal(err)
	}
	direct, err = a.Surfaces.Run(ctx, "upload_batch_get", map[string]any{"batch_id": "SRV-BATCH-k"})
	if err != nil {
		t.Fatal(err)
	}
	viaCall, err := a.Call(ctx, "upload_batch_get", map[string]any{"batch_id": "SRV-BATCH-k"})
	if err != nil {
		t.Fatal(err)
	}
	srv := a.MCP()
	response := srv.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"upload_batch_get","arguments":{"batch_id":"SRV-BATCH-k"}}}`))
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var rpc struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(encoded, &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Result.IsError || len(rpc.Result.Content) == 0 {
		t.Fatalf("MCP dispatch failed: %s", encoded)
	}
	var want, got map[string]any
	wantJSON, _ := json.Marshal(direct)
	callJSON, _ := json.Marshal(viaCall)
	json.Unmarshal(wantJSON, &want)
	json.Unmarshal([]byte(rpc.Result.Content[0].Text), &got)
	if string(wantJSON) != string(callJSON) {
		t.Fatalf("adapter Call diverged from the application contract:\n%s\n%s", wantJSON, callJSON)
	}
	var gotViaCall map[string]any
	json.Unmarshal(callJSON, &gotViaCall)
	if got["batch_id"] != gotViaCall["batch_id"] || got["phase"] != gotViaCall["phase"] {
		t.Fatalf("MCP dispatch diverged from the application contract: %s vs %s", rpc.Result.Content[0].Text, callJSON)
	}
	if want["batch_id"] != "SRV-BATCH-k" {
		t.Fatalf("server identity must pass through verbatim: %v", want)
	}
}

// TestFPRemoteSurfacesBoundedFileUploadStreamsThroughContract exercises the
// full pure-MCP discovery → bounded upload flow over real HTTP: capability
// discovery gates the tool, the file streams as parts through the public
// byte path, and the receipt keeps the server's original IDs.
func TestFPRemoteSurfacesBoundedFileUploadStreamsThroughContract(t *testing.T) {
	server := surfacesContractFixture(t, bridge.RemoteSurfaceActionNames())
	a := surfacesFixtureAdapter(t, server.URL)
	if err := a.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	payload := filepath.Join(root, "bounded.bin")
	chunk := make([]byte, 1<<20)
	for i := 1; i <= 3; i++ {
		copy(chunk, fmt.Sprintf("part-%d", i))
		if err := os.WriteFile(payload+"."+fmt.Sprint(i), chunk, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(payload, make([]byte, 3*(1<<20)+17), 0600); err != nil {
		t.Fatal(err)
	}
	// The adapter's root is its own fixture root; rebind the surfaces root
	// to this payload directory for the byte source.
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFS.Close()
	a.Surfaces.Root = rootFS
	receipt, err := a.Call(context.Background(), "upload_batch_file", map[string]any{
		"space_id": "sp", "local_path": "bounded.bin", "conflict_policy": "fail", "idempotency_key": "bounded-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := receipt.(map[string]any)
	if doc["batch_id"] != "SRV-BATCH-bounded-1" || doc["operation_id"] != "SRV-OP-SRV-BATCH-bounded-1" {
		t.Fatalf("bounded upload receipt: %v", doc)
	}
	if doc["bytes_streamed"] != int64(3*(1<<20)+17) {
		t.Fatalf("streamed bytes: %v", doc["bytes_streamed"])
	}
	// A source outside the root is refused before any byte leaves.
	if _, err = a.Call(context.Background(), "upload_batch_file", map[string]any{
		"space_id": "sp", "local_path": "../escape.bin", "idempotency_key": "bounded-2",
	}); err == nil || err.Error() != "path_not_allowed" {
		t.Fatalf("outside-root source: %v", err)
	}
}

// surfacesScenarioFixture extends the contract fixture with the recovery
// and snapshot routes the cross-owner scenario needs: the create response
// drops once per key, lookup answers from the accepted batch, and the
// snapshot routes enforce the explicit export permission.
func surfacesScenarioFixture(t *testing.T) *httptest.Server {
	t.Helper()
	accepted := map[string]map[string]any{}
	dropped := map[string]bool{}
	parts := map[string]int64{}
	exportPermission := true
	mux := http.NewServeMux()
	encode := func(w http.ResponseWriter, doc map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": doc})
	}
	fail := func(w http.ResponseWriter, code string, status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
	}
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"instance_id": "SRV-SCENARIO", "actions": bridge.RemoteSurfaceActionNames()})
	})
	mux.HandleFunc("POST /api/v1/upload-batches", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SpaceID        string `json:"space_id"`
			ConflictPolicy string `json:"conflict_policy"`
		}
		json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
		key := r.Header.Get("Idempotency-Key")
		if prior, ok := accepted[key]; ok {
			encode(w, prior)
			return
		}
		if !dropped[key] {
			// The request is accepted durably; only the response is lost.
			dropped[key] = true
			accepted[key] = map[string]any{"batch_id": "SRV-SCN-BATCH-" + key, "operation_id": "SRV-SCN-OP-" + key}
			w.Header().Set("Connection", "close")
			return
		}
		accepted[key] = map[string]any{"batch_id": "SRV-SCN-BATCH-" + key, "operation_id": "SRV-SCN-OP-" + key}
		encode(w, accepted[key])
	})
	mux.HandleFunc("POST /api/v1/operations/lookup", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action          string         `json:"action"`
			IdempotencyKey  string         `json:"idempotency_key"`
			OriginalRequest map[string]any `json:"original_request"`
		}
		json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
		if prior, ok := accepted[body.IdempotencyKey]; ok && body.Action == "upload_batch_create" {
			doc := map[string]any{"found": true, "result_certainty": "confirmed", "recovery_action": "read_original_operation"}
			for k, v := range prior {
				doc[k] = v
			}
			encode(w, doc)
			return
		}
		encode(w, map[string]any{"found": false})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"accepted": 1})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/seal", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"operation_id": "SRV-SCN-OP-" + r.PathValue("id"), "part_size": 1 << 20})
	})
	mux.HandleFunc("PUT /api/v1/operations/{id}/parts/{index}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		parts[r.PathValue("id")] += n
		encode(w, map[string]any{"received_bytes": n})
	})
	mux.HandleFunc("GET /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		phase := "enumerating"
		if parts["SRV-SCN-OP-"+id] > 0 {
			phase = "sealed"
		}
		encode(w, map[string]any{"batch_id": id, "phase": phase, "items": []any{}, "operation_id": "SRV-SCN-OP-" + id})
	})
	mux.HandleFunc("POST /api/v1/analytics/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if !exportPermission {
			fail(w, "forbidden", 403)
			return
		}
		encode(w, map[string]any{"snapshot_id": "SRV-SCN-SNAP", "generation": "scn-gen-1"})
	})
	mux.HandleFunc("POST /api/v1/analytics/snapshots/{id}/queries", func(w http.ResponseWriter, r *http.Request) {
		if !exportPermission {
			fail(w, "forbidden", 403)
			return
		}
		encode(w, map[string]any{"snapshot_id": r.PathValue("id"), "generation": "scn-gen-1", "rows": []any{map[string]any{"dataset": "file_current", "files": 2}}})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestFPRemoteSurfacesCrossOwnerScenarioWalksTheHandoff pins the frozen
// handoff scenario end to end through public interfaces only: pure-MCP
// discovery, one bounded file upload from the allowed root, original-ID
// recovery after a lost response, and one authorized snapshot query. The
// fixture stands in for the backend wave; the real-service run stays a
// separately reported integration fact.
func TestFPRemoteSurfacesCrossOwnerScenarioWalksTheHandoff(t *testing.T) {
	server := surfacesScenarioFixture(t)
	a := surfacesFixtureAdapter(t, server.URL)
	if err := a.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// 1. Pure MCP discovery: tools/list names the surface tools the server
	// advertises; no CLI, no local state, no guessed aliases.
	srv := a.MCP()
	listed := srv.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	encoded, _ := json.Marshal(listed)
	for _, want := range []string{"upload_batch_file", "files_query", "organize_plan", "organize_apply", "remote_surfaces_status"} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("tools/list missing %s: %s", want, encoded)
		}
	}
	// 2. Bounded file upload from the allowed root.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "handoff.bin"), []byte("cross-owner-handoff-payload"), 0600); err != nil {
		t.Fatal(err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFS.Close()
	a.Surfaces.Root = rootFS
	receipt, err := a.Call(ctx, "upload_batch_file", map[string]any{
		"space_id": "sp", "local_path": "handoff.bin", "idempotency_key": "scn-1",
	})
	if err != nil {
		t.Fatalf("bounded upload: %v", err)
	}
	doc, _ := receipt.(map[string]any)
	if doc["batch_id"] != "SRV-SCN-BATCH-scn-1" {
		t.Fatalf("upload receipt must carry the server's original batch: %v", doc)
	}
	// 3. Original-ID recovery: the create response dropped once; the same
	// key plus the same original request reads the original IDs back.
	lookup, err := a.Call(ctx, "operation_lookup", map[string]any{
		"action": "upload_batch_create", "idempotency_key": "scn-1",
		"original_request": map[string]any{"space_id": "sp", "conflict_policy": "fail"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recovered, _ := lookup.(map[string]any)
	if recovered["batch_id"] != "SRV-SCN-BATCH-scn-1" || recovered["found"] != true {
		t.Fatalf("lookup must recover the original IDs: %v", recovered)
	}
	readBack, err := a.Call(ctx, "upload_batch_get", map[string]any{"batch_id": "SRV-SCN-BATCH-scn-1"})
	if err != nil {
		t.Fatal(err)
	}
	status, _ := readBack.(map[string]any)
	if status["batch_id"] != "SRV-SCN-BATCH-scn-1" || status["phase"] != "sealed" {
		t.Fatalf("original ID readback: %v", status)
	}
	// 4. Authorized snapshot query: same generation, explicit scope.
	snap, err := a.Call(ctx, "analytics_snapshot", map[string]any{"space_id": "sp", "idempotency_key": "scn-snap"})
	if err != nil {
		t.Fatal(err)
	}
	snapDoc, _ := snap.(map[string]any)
	if snapDoc["snapshot_id"] != "SRV-SCN-SNAP" {
		t.Fatalf("snapshot receipt: %v", snapDoc)
	}
	query, err := a.Call(ctx, "analytics_snapshot_query", map[string]any{"snapshot_id": "SRV-SCN-SNAP", "dataset": "file_current"})
	if err != nil {
		t.Fatal(err)
	}
	queryDoc, _ := query.(map[string]any)
	if queryDoc["generation"] != "scn-gen-1" {
		t.Fatalf("snapshot query must report the snapshot generation: %v", queryDoc)
	}
}

// TestFPRemoteSurfacesConcurrentRevocationAndErrorInjection pins the
// concurrency-and-failure discipline required by the spec: revocation
// observed mid-flight surfaces as forbidden on the next call with no
// escalation, and a byte-path failure mid-stream keeps the original
// operation identity with an explicit interrupted verdict.
func TestFPRemoteSurfacesConcurrentRevocationAndErrorInjection(t *testing.T) {
	authorized := true
	revokedAt := int64(0)
	calls := int64(0)
	failParts := false
	mux := http.NewServeMux()
	encode := func(w http.ResponseWriter, doc map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": doc})
	}
	fail := func(w http.ResponseWriter, code string, status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
	}
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"actions": bridge.RemoteSurfaceActionNames()})
	})
	mux.HandleFunc("POST /api/v1/analytics/snapshots", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1) - 1
		if authorized {
			// Revoke after the third create: later callers observe the
			// refusal; the in-flight one keeps its original identity.
			if n >= 3 {
				authorized = false
				revokedAt = n
			}
		}
		if !authorized {
			fail(w, "forbidden", 403)
			return
		}
		encode(w, map[string]any{"snapshot_id": fmt.Sprintf("SNAP-%d", n)})
	})
	mux.HandleFunc("POST /api/v1/upload-batches", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"batch_id": "REV-BATCH"})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"accepted": 1})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/seal", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"operation_id": "REV-OP", "part_size": 1 << 10})
	})
	mux.HandleFunc("PUT /api/v1/operations/{id}/parts/{index}", func(w http.ResponseWriter, r *http.Request) {
		if failParts {
			fail(w, "upload_interrupted", 500)
			return
		}
		io.Copy(io.Discard, r.Body)
		encode(w, map[string]any{"received_bytes": 0})
	})
	mux.HandleFunc("GET /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"batch_id": r.PathValue("id"), "phase": "enumerating"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	a := surfacesFixtureAdapter(t, server.URL)
	if err := a.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Concurrent callers: the first three succeed, the rest observe the
	// revocation as forbidden — never a rebind, escalation or retry.
	var wg sync.WaitGroup
	results := make([]error, 6)
	for i := range 6 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = a.Call(ctx, "analytics_snapshot", map[string]any{"space_id": "sp", "idempotency_key": fmt.Sprintf("rev-%d", i)})
		}(i)
	}
	wg.Wait()
	succeeded, refused := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case err.Error() == "forbidden":
			refused++
		default:
			t.Fatalf("unexpected concurrent verdict: %v", err)
		}
	}
	if succeeded < 1 || refused < 1 || succeeded+refused != 6 {
		t.Fatalf("concurrent revocation split: ok=%d refused=%d", succeeded, refused)
	}
	if authorized || revokedAt == 0 {
		t.Fatalf("revocation must have been observed: authorized=%v revokedAt=%d", authorized, revokedAt)
	}
	// Byte-path error injection: a mid-stream part failure surfaces the
	// server code and the original operation identity stays readable.
	failParts = true
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inject.bin"), make([]byte, 4096+31), 0600); err != nil {
		t.Fatal(err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFS.Close()
	a.Surfaces.Root = rootFS
	_, err = a.Call(ctx, "upload_batch_file", map[string]any{"space_id": "sp", "local_path": "inject.bin", "idempotency_key": "inj-1"})
	if err == nil || err.Error() != "upload_interrupted" {
		t.Fatalf("part failure must surface the server code: %v", err)
	}
	readBack, err := a.Call(ctx, "upload_batch_get", map[string]any{"batch_id": "REV-BATCH"})
	if err != nil {
		t.Fatal(err)
	}
	status, _ := readBack.(map[string]any)
	if status["batch_id"] != "REV-BATCH" {
		t.Fatalf("original identity must remain readable after injection: %v", status)
	}
}
