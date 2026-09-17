package remote

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	aigoraFixtureKey   = "fixture-aigora-key"
	aigoraFixtureProof = "header.payload.signature"
)

func aigoraFixtureGateway(t *testing.T, exchanges *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/service-access/exchange" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+aigoraFixtureKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		exchanges.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"principal_context": aigoraFixtureProof, "expires_at": time.Now().Add(5 * time.Minute).Unix()}})
	}))
}

func aigoraFixtureAdapter(t *testing.T, endpoint, aigoraOrigin string) *Adapter {
	t.Helper()
	dir := t.TempDir()
	credential := filepath.Join(dir, "token")
	if err := os.WriteFile(credential, []byte(aigoraFixtureKey+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	a, err := NewWithAigora(endpoint, credential, root, aigoraOrigin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func TestAigoraLegacyModeUnchangedWithoutExplicitURL(t *testing.T) {
	var seen atomic.Int64
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+aigoraFixtureKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seen.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]bool{"ok": true}})
	}))
	defer owner.Close()
	a := aigoraFixtureAdapter(t, owner.URL, "")
	if _, err := a.Call(context.Background(), "capabilities", nil); err != nil {
		t.Fatal(err)
	}
	if seen.Load() != 1 {
		t.Fatalf("legacy mode calls=%d", seen.Load())
	}
	if a.unifiedIdentity {
		t.Fatal("legacy mode marked as aigora exchange")
	}
}

func TestAigoraKeyNeverSentToFileService(t *testing.T) {
	var exchanges atomic.Int64
	gateway := aigoraFixtureGateway(t, &exchanges)
	defer gateway.Close()
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.Contains(auth, aigoraFixtureKey) {
			t.Error("long-term key reached the file service")
		}
		if auth != "Bearer "+aigoraFixtureProof {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), aigoraFixtureKey) {
			t.Error("long-term key in request body")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]bool{"ok": true}})
	}))
	defer owner.Close()
	a := aigoraFixtureAdapter(t, owner.URL, gateway.URL)
	if _, err := a.Call(context.Background(), "capabilities", nil); err != nil {
		t.Fatal(err)
	}
	if exchanges.Load() != 1 {
		t.Fatalf("exchanges=%d", exchanges.Load())
	}
	if !a.unifiedIdentity {
		t.Fatal("aigora mode not marked")
	}
}

func TestAigoraRedirectAndExchangeFailureDoNotFallBack(t *testing.T) {
	for name, gatewayHandler := range map[string]http.HandlerFunc{
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://example.invalid/elsewhere")
			w.WriteHeader(http.StatusFound)
		},
		"denied": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
	} {
		t.Run(name, func(t *testing.T) {
			gateway := httptest.NewServer(gatewayHandler)
			defer gateway.Close()
			var ownerCalls atomic.Int64
			owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ownerCalls.Add(1) }))
			defer owner.Close()
			a := aigoraFixtureAdapter(t, owner.URL, gateway.URL)
			if _, err := a.Call(context.Background(), "capabilities", nil); err == nil {
				t.Fatal("exchange failure ignored")
			} else if !safeRemoteCode(err.Error()) {
				t.Fatalf("untyped error surfaced: %v", err)
			}
			if ownerCalls.Load() != 0 {
				t.Fatal("file service contacted without a service context")
			}
		})
	}
}

func TestAigoraAdmissionRequiredStopsWithStableCode(t *testing.T) {
	var exchanges atomic.Int64
	gateway := aigoraFixtureGateway(t, &exchanges)
	defer gateway.Close()
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "service_admission_required", "message": "hosted write path not enabled"}})
	}))
	defer owner.Close()
	a := aigoraFixtureAdapter(t, owner.URL, gateway.URL)
	_, err := a.Call(context.Background(), "spaces", nil)
	if err == nil || err.Error() != "service_admission_required" {
		t.Fatalf("admission stop code lost: %v", err)
	}
	server := a.MCP()
	response := server.HandleMessage(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"spaces","arguments":{}}}`))
	body, _ := json.Marshal(response)
	text := string(body)
	if !strings.Contains(text, "service_admission_required") {
		t.Fatalf("MCP surface lost the stop code: %s", text)
	}
	for _, banned := range []string{"direct", "byok", "swap", "replace"} {
		if strings.Contains(strings.ToLower(text), banned) {
			t.Fatalf("MCP output suggests a forbidden fallback: %s", banned)
		}
	}
}

func TestAigoraResumeUsesOriginalOperationIdentifiers(t *testing.T) {
	var exchanges atomic.Int64
	gateway := aigoraFixtureGateway(t, &exchanges)
	defer gateway.Close()
	var uploads, resumes atomic.Int64
	var idempotencyKeys []string
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/uploads":
			uploads.Add(1)
			if k := r.Header.Get("Idempotency-Key"); k != "" {
				idempotencyKeys = append(idempotencyKeys, k)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-1", "status": "completed", "file_id": "f-1", "version_id": "v-1"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/files/f-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref": "db://space/f-1/version/v-1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/operations/op-1/resume":
			resumes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-1", "status": "awaiting_upload"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/operations/op-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"operation": map[string]any{"id": "op-1", "status": "awaiting_upload"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "remote_not_found"}})
		}
	}))
	defer owner.Close()
	a := aigoraFixtureAdapter(t, owner.URL, gateway.URL)

	if err := os.WriteFile(filepath.Join(a.Root.Name(), "input.bin"), []byte("fixture-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Call(context.Background(), "upload", map[string]any{"space_id": "s-1", "local_path": "input.bin", "path": "input.bin", "idempotency_key": "key-1"}); err != nil {
		t.Fatal(err)
	}
	if uploads.Load() != 1 || len(idempotencyKeys) != 1 || idempotencyKeys[0] != "key-1" {
		t.Fatalf("uploads=%d keys=%v", uploads.Load(), idempotencyKeys)
	}
	out, err := a.Call(context.Background(), "operation_get", map[string]any{"operation_id": "op-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Call(context.Background(), "resume", map[string]any{"operation_id": "op-1"}); err != nil {
		t.Fatal(err)
	}
	_ = out
	if resumes.Load() != 1 {
		t.Fatalf("resumes=%d", resumes.Load())
	}
	if uploads.Load() != 1 {
		t.Fatal("resume created a replacement paid operation")
	}
}

func safeRemoteCode(v string) bool {
	if v == "" || len(v) > 80 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return false
		}
	}
	return true
}
