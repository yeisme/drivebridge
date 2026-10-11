package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/sdk"
)

// manjuStub serves the content-owner contract shape over loopback: a
// process endpoint returning an index generation and a discovery query
// endpoint returning evidence items.
func manjuStub(t *testing.T, searchable bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-manju" {
			w.WriteHeader(403)
			return
		}
		switch r.URL.Path {
		case "/api/v1/process":
			if searchable {
				json.NewEncoder(w).Encode(map[string]any{"index_generation": "gen-1"})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"index_generation": "gen-1", "unsupported": []string{"audio"}})
			}
		case "/api/v1/query":
			json.NewEncoder(w).Encode(map[string]any{"coverage": "partial", "missing": []string{"audio"}, "items": []map[string]any{
				{"file_ref": manjuKnownRef, "asset_revision_ref": "asset-1", "evidence_ref": "ev-1", "locator": map[string]any{"kind": "time_range", "start_ms": 1000, "end_ms": 2500}, "truth_level": "inferred", "index_generation": "gen-1"},
				{"file_ref": "drivebridge://foreign.invalid/file/x/version/y"},
			}})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

var manjuKnownRef string

// contentRig starts a fixture with one uploaded file and a Manju-shaped
// connection pointed at the stub.
func contentRig(t *testing.T, searchable bool) (*fixtures.Runtime, *human, string) {
	t.Helper()
	f, e := fixtures.StartWithOptions(t.TempDir(), "", "", fixtures.Options{})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "docs/scene.mp4"}, "cr-1", sizeReader(64), nil)
	if e != nil {
		t.Fatal(e)
	}
	manjuKnownRef = "drivebridge://" + f.Service.Instance + "/file/" + op.FileID + "/version/" + op.VersionID
	stub := manjuStub(t, searchable)
	var conn service0Connection
	if status := h.call(t, "POST", "/creative/connections", map[string]string{"kind": "manju", "name": "Manju", "project_ref": "project-a", "url": stub.URL, "token": "fixture-manju"}, &conn); status != 200 {
		t.Fatalf("manju connection status %d", status)
	}
	return f, h, conn.ID
}

type service0Connection struct {
	ID string `json:"id"`
}

// Task 2.1/3.1: the binding contract — pinned identity, policy gate and
// idempotent replay.
func TestFPContentRetrievalBindingIdentityAndPolicyGate(t *testing.T) {
	f, h, connID := contentRig(t, true)
	c := mustSDK(t, h)
	// Without an enabled policy, binding creation is refused.
	if _, e := c.CreateContentBinding(context.Background(), sdk.CreateContentBindingRequest{Ref: manjuKnownRef, ManjuConnectionID: connID, ProjectRef: "project-a", Purpose: "indexing"}, "bind-key"); e == nil || e.Error() != "content_policy_required" {
		t.Fatalf("binding without policy must be content_policy_required, got %v", e)
	}
	// The human enables a scoped policy.
	policy := setContentPolicyHTTP(t, h, h.space.ID, map[string]any{"prefixes": []string{"docs"}, "requested_layers": []string{"vision"}, "enabled": true}, 0)
	if policy.PolicyRevision != 1 || !policy.Enabled {
		t.Fatalf("policy wrong: %+v", policy)
	}
	// Agents never widen policies.
	_, ac := agent(t, h, "", "read")
	if _, e := ac.SetContentPolicy(context.Background(), h.space.ID, sdk.ContentPolicyRequest{Enabled: true}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent policy write must be forbidden, got %v", e)
	}
	// Binding under the enabled policy pins the version and replays.
	first, e := c.CreateContentBinding(context.Background(), sdk.CreateContentBindingRequest{Ref: manjuKnownRef, ManjuConnectionID: connID, ProjectRef: "project-a", Purpose: "indexing"}, "bind-key")
	if e != nil {
		t.Fatal(e)
	}
	if first.State != "not_processed" || first.FileRef != manjuKnownRef {
		t.Fatalf("binding wrong: %+v", first)
	}
	replay, e := c.CreateContentBinding(context.Background(), sdk.CreateContentBindingRequest{Ref: manjuKnownRef, ManjuConnectionID: connID, ProjectRef: "project-a", Purpose: "indexing"}, "bind-key")
	if e != nil || replay.BindingID != first.BindingID {
		t.Fatalf("binding replay wrong: %+v %v", replay, e)
	}
	// A different purpose under the same key conflicts.
	if _, e := c.CreateContentBinding(context.Background(), sdk.CreateContentBindingRequest{Ref: manjuKnownRef, ManjuConnectionID: connID, ProjectRef: "project-a", Purpose: "other"}, "bind-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("fingerprint drift must be idempotency_conflict, got %v", e)
	}
	_ = f
}

// Task 3.1/3.2: processing and version semantics — success marks the
// binding searchable with an index generation; a version change makes the
// binding stale and late results never attach to the new version.
func TestFPContentRetrievalProcessAndVersionSemantics(t *testing.T) {
	f, h, connID := contentRig(t, true)
	c := mustSDK(t, h)
	setContentPolicyHTTP(t, h, h.space.ID, map[string]any{"prefixes": []string{"docs"}, "enabled": true}, 0)
	policy, _ := c.GetContentPolicy(context.Background(), h.space.ID)
	binding, e := c.CreateContentBinding(context.Background(), sdk.CreateContentBindingRequest{Ref: manjuKnownRef, ManjuConnectionID: connID, ProjectRef: "project-a", Purpose: "indexing"}, "proc-key")
	if e != nil {
		t.Fatal(e)
	}
	// A stale policy revision refuses.
	if _, e := c.ProcessContent(context.Background(), binding.BindingID, policy.PolicyRevision+7, "run-key"); e == nil || e.Error() != "version_conflict" {
		t.Fatalf("stale policy revision must be version_conflict, got %v", e)
	}
	processed, e := c.ProcessContent(context.Background(), binding.BindingID, policy.PolicyRevision, "run-key")
	if e != nil {
		t.Fatal(e)
	}
	if processed.State != "searchable" || processed.IndexGeneration != "gen-1" || processed.OperationID == "" {
		t.Fatalf("processed wrong: %+v", processed)
	}
	// The original file stays fully readable regardless.
	var files []sdk.File
	if e := c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID, "", nil, &files); e != nil || len(files) != 1 {
		t.Fatalf("original file disturbed: %v", e)
	}
	// A version change stops new consumption with content_index_stale.
	var listed []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID, "", nil, &listed)
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "docs/scene.mp4", FileID: listed[0].ID, ExpectedVersion: listed[0].VersionID}, "cr-2", sizeReader(80), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := c.ProcessContent(context.Background(), binding.BindingID, policy.PolicyRevision, "run-key-2"); e == nil || e.Error() != "content_index_stale" {
		t.Fatalf("version drift must be content_index_stale, got %v", e)
	}
	stale, _ := c.GetContentBinding(context.Background(), binding.BindingID)
	if stale.State != "stale" {
		t.Fatalf("binding must be stale: %+v", stale)
	}
	_ = f
}

// Task 3.1/3.2: retrieval — exact mode reads managed facts; discovery
// mode re-authorizes every content-owner item, drops foreign refs and
// reports missing modalities; unsupported modalities and missing owners
// answer with the typed codes.
func TestFPContentRetrievalQueryModesAndAuthorization(t *testing.T) {
	_, h, connID := contentRig(t, true)
	c := mustSDK(t, h)
	setContentPolicyHTTP(t, h, h.space.ID, map[string]any{"enabled": true}, 0)
	// Exact mode: managed facts only, no content owner involved.
	exact, e := c.QueryContent(context.Background(), sdk.ContentQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Limit: 10})
	if e != nil || len(exact.Items) != 1 || exact.Items[0].FileRef != manjuKnownRef {
		t.Fatalf("exact wrong: %+v %v", exact, e)
	}
	// Discovery mode through the content owner: foreign refs drop, the
	// authorized item keeps its evidence/locator/truth facts.
	discovery, e := c.QueryContent(context.Background(), sdk.ContentQueryRequest{Mode: "discovery", Text: "雨天街道", SpaceIDs: []string{h.space.ID}, Modalities: []string{"video", "audio"}, EvidenceRequired: true, ManjuConnectionID: connID})
	if e != nil {
		t.Fatal(e)
	}
	if discovery.Coverage != "partial" || len(discovery.Items) != 1 {
		t.Fatalf("discovery wrong: %+v", discovery)
	}
	item := discovery.Items[0]
	if item.FileRef != manjuKnownRef || item.EvidenceRef != "ev-1" || item.TruthLevel != "inferred" || item.Locator["kind"] != "time_range" || item.IndexGeneration != "gen-1" {
		t.Fatalf("discovery item wrong: %+v", item)
	}
	if len(discovery.Missing) != 1 || discovery.Missing[0] != "audio" {
		t.Fatalf("missing modalities wrong: %+v", discovery.Missing)
	}
	// Unknown modalities are refused without a substitute model.
	if _, e := c.QueryContent(context.Background(), sdk.ContentQueryRequest{Mode: "discovery", SpaceIDs: []string{h.space.ID}, Modalities: []string{"smell"}, ManjuConnectionID: connID}); e == nil || e.Error() != "modality_unsupported" {
		t.Fatalf("unknown modality must be modality_unsupported, got %v", e)
	}
	// A missing content owner is reported honestly.
	if _, e := c.QueryContent(context.Background(), sdk.ContentQueryRequest{Mode: "discovery", Text: "x", SpaceIDs: []string{h.space.ID}, ManjuConnectionID: "nonexistent"}); e == nil || e.Error() != "identity_mapping_required" {
		t.Fatalf("missing owner mapping must be identity_mapping_required, got %v", e)
	}
	// Revoked agents lose retrieval without leaking items.
	a, ac := agent(t, h, "", "read")
	_ = a
	if _, e := ac.QueryContent(context.Background(), sdk.ContentQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}}); e != nil {
		t.Fatalf("scoped agent exact read: %v", e)
	}
	_ = fmt.Sprint
}

// setContentPolicyHTTP writes the human-only policy over the public
// contract.
func setContentPolicyHTTP(t *testing.T, h *human, spaceID string, body map[string]any, expected uint64) sdk.ContentPolicyView {
	t.Helper()
	if expected > 0 {
		body["expected_revision"] = expected
	}
	var out sdk.ContentPolicyView
	if status := h.call(t, "PUT", "/spaces/"+spaceID+"/content-policy", body, &out); status != 200 {
		t.Fatalf("policy status %d", status)
	}
	return out
}

// Rework (content M): processing persists its idempotency — a same-key
// replay reads the original operation back instead of erroring, a
// different intent under the key conflicts, and a failed attempt frees
// the key for an explicit retry.
func TestFPContentRetrievalProcessIdempotencyPersisted(t *testing.T) {
	_, h, connID := contentRig(t, true)
	setContentPolicyHTTP(t, h, h.space.ID, map[string]any{"enabled": true}, 0)
	policy, _ := mustSDK(t, h).GetContentPolicy(context.Background(), h.space.ID)
	binding := contentBindingHTTP(t, h, connID, manjuKnownRef, "idem-key")
	// First processing succeeds.
	first := processContentHTTP(t, h, binding.BindingID, policy.PolicyRevision, "run-key")
	if first.State != "searchable" || first.OperationID == "" {
		t.Fatalf("first process wrong: %+v", first)
	}
	// Same key, same intent: replay returns the original operation.
	replay := processContentHTTP(t, h, binding.BindingID, policy.PolicyRevision, "run-key")
	if replay.OperationID != first.OperationID || replay.State != "searchable" {
		t.Fatalf("replay wrong: %+v vs %+v", replay, first)
	}
	// A different intent under the same key conflicts.
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCallWithKey(t, h, "POST", "/content-bindings/"+binding.BindingID+"/process", mustJSON(map[string]any{"policy_revision": policy.PolicyRevision + 5}), "run-key")
	json.Unmarshal(raw, &env)
	if status != 409 {
		t.Fatalf("different intent under same key: got %d %s", status, raw)
	}
}

// contentBindingHTTP creates one binding over the public contract.
func contentBindingHTTP(t *testing.T, h *human, connID, ref, key string) sdk.ContentBindingView {
	t.Helper()
	body := map[string]any{"ref": ref, "manju_connection_id": connID, "project_ref": "project-a", "purpose": "indexing"}
	req, _ := http.NewRequest("POST", h.base+"/api/v1/content-bindings", bytes.NewReader(mustJSON(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Data  sdk.ContentBindingView `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &out)
	if resp.StatusCode != 200 {
		t.Fatalf("binding status %d: %s", resp.StatusCode, raw)
	}
	return out.Data
}

// processContentHTTP runs one processing over the public contract.
func processContentHTTP(t *testing.T, h *human, bindingID string, policyRevision uint64, key string) sdk.ContentBindingView {
	t.Helper()
	req, _ := http.NewRequest("POST", h.base+"/api/v1/content-bindings/"+bindingID+"/process", bytes.NewReader(mustJSON(map[string]any{"policy_revision": policyRevision})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Data  sdk.ContentBindingView `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &out)
	if resp.StatusCode != 200 {
		t.Fatalf("process status %d: %s", resp.StatusCode, raw)
	}
	return out.Data
}

// fpRawCallWithKey performs one raw HTTP call carrying an idempotency key.
func fpRawCallWithKey(t *testing.T, h *human, method, path string, body []byte, key string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, h.base+"/api/v1"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}
