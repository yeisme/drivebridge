package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func reuseFixtureAdapter(t *testing.T, endpoint string) *Adapter {
	t.Helper()
	dir := t.TempDir()
	credential := filepath.Join(dir, "token")
	if err := os.WriteFile(credential, []byte("fixture-domain-token"), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.bin"), []byte("fixture-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := NewWithAigora(endpoint, credential, root, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// A server-side blob hit completes the upload at create time; the client must
// not send a single part. Another owner's digest is never treated as
// authorization: only the server's completed receipt decides.
func TestRemoteBlobReuseSkipsParts(t *testing.T) {
	var creates, parts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/uploads":
			creates.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-1", "status": "completed", "file_id": "f-1", "version_id": "v-1", "bytes_transferred": 0}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/files/f-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref": "db://space/f-1/version/v-1"}})
		default:
			parts.Add(1)
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "remote_not_found"}})
		}
	}))
	defer server.Close()
	a := reuseFixtureAdapter(t, server.URL)
	out, err := a.Call(context.Background(), "upload", map[string]any{"space_id": "s-1", "local_path": "input.bin", "path": "input.bin", "idempotency_key": "k-1"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, ok := out.(UploadResult)
	if !ok || receipt.Status != "completed" {
		t.Fatalf("receipt: %#v", out)
	}
	if creates.Load() != 1 || parts.Load() != 0 {
		t.Fatalf("creates=%d parts=%d", creates.Load(), parts.Load())
	}
}

// A miss keeps the original chunked upload; an idempotency_conflict from the
// server passes through with its stable code.
func TestRemoteBlobMissKeepsPartsAndConflictPassesThrough(t *testing.T) {
	var parts atomic.Int64
	conflict := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/uploads":
			if r.Header.Get("Idempotency-Key") == "k-conflict" {
				conflict = true
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "idempotency_conflict"}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-2", "status": "awaiting_upload", "file_id": "f-2", "version_id": "v-0"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/operations/op-2":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"operation": map[string]any{"id": "op-2", "status": "awaiting_upload"}, "received_parts": []int{}}})
		case r.Method == http.MethodPut:
			parts.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-2", "status": "awaiting_upload"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/operations/op-2/complete":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "op-2", "status": "completed", "file_id": "f-2", "version_id": "v-2"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/files/f-2":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref": "db://space/f-2/version/v-2"}})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "remote_not_found"}})
		}
	}))
	defer server.Close()
	a := reuseFixtureAdapter(t, server.URL)
	out, err := a.Call(context.Background(), "upload", map[string]any{"space_id": "s-1", "local_path": "input.bin", "path": "input.bin", "idempotency_key": "k-miss"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt, ok := out.(UploadResult); !ok || receipt.Status != "completed" {
		t.Fatalf("miss receipt: %#v", out)
	}
	if parts.Load() != 1 {
		t.Fatalf("parts=%d", parts.Load())
	}
	if _, err = a.Call(context.Background(), "upload", map[string]any{"space_id": "s-1", "local_path": "input.bin", "path": "other.bin", "idempotency_key": "k-conflict"}); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatalf("conflict: %v", err)
	}
	if !conflict {
		t.Fatal("conflict path never reached the fake server")
	}
}
