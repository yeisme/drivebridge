package remote

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSemanticHandoffRemoteMCPPinnedReadAndCurrentAuthority(t *testing.T) {
	const ref = "drivebridge://local-instance/file/fixed-file/version/fixed-version"
	raw := []byte("fixed version bytes, not the current head")
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	var revoked, corrupt atomic.Bool
	var reads atomic.Int64
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+"fixture-token" {
			t.Error("missing current service identity")
		}
		if revoked.Load() {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "forbidden"}})
			return
		}
		switch r.URL.Path {
		case "/api/v1/references":
			if r.URL.Query().Get("ref") != ref {
				t.Error("reference changed at server boundary")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref": ref, "file": map[string]any{"id": "fixed-file", "version_id": "fixed-version", "sha256": digest}}})
		case "/api/v1/files/fixed-file/content":
			reads.Add(1)
			if r.URL.Query().Get("version") != "fixed-version" {
				t.Error("download silently requested head")
			}
			w.Header().Set("ETag", `"opaque-server-etag"`)
			w.Header().Set("X-Content-SHA256", digest)
			w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
			payload := bytes.Clone(raw)
			if corrupt.Load() {
				payload[0] ^= 1
			}
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected controlled server contract: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(owner.Close)
	a := surfacesFixtureAdapter(t, owner.URL)
	srv := a.MCP()
	call := func(name string, args map[string]any) (string, bool) {
		t.Helper()
		request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		wire, _ := json.Marshal(srv.HandleMessage(context.Background(), request))
		var out struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(wire, &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Result.Content) != 1 || out.Result.Content[0].Type != "text" {
			t.Fatalf("missing MCP text result: %s", wire)
		}
		for _, private := range []string{"fixture-token", owner.URL, a.Root.Name()} {
			if strings.Contains(string(wire), private) {
				t.Fatal("MCP returned private transport details")
			}
		}
		return out.Result.Content[0].Text, out.Result.IsError
	}
	listed, _ := json.Marshal(srv.HandleMessage(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)))
	for _, field := range []string{"download", "file_ref", "local_path"} {
		if !bytes.Contains(listed, []byte(field)) {
			t.Fatalf("public schema missing %s", field)
		}
	}
	body, failed := call("stat", map[string]any{"ref": ref})
	var resolved struct {
		Ref  string `json:"ref"`
		File struct {
			ID      string `json:"id"`
			Version string `json:"version_id"`
		} `json:"file"`
	}
	if err := json.Unmarshal([]byte(body), &resolved); err != nil || failed || resolved.Ref != ref || resolved.File.ID != "fixed-file" || resolved.File.Version != "fixed-version" {
		t.Fatalf("legacy ref resolution: %s (%v)", body, err)
	}
	// Legacy sdk.File deliberately has no digest field. Byte integrity is
	// established by Download's X-Content-SHA256 check below, not stat metadata.
	if body, failed := call("download", map[string]any{"file_ref": ref, "local_path": "received.bin"}); failed {
		t.Fatal(body)
	}
	got, err := os.ReadFile(filepath.Join(a.Root.Name(), "received.bin"))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("pinned bytes differ", err)
	}
	if err := os.WriteFile(filepath.Join(a.Root.Name(), "existing.bin"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, failed := call("download", map[string]any{"ref": ref, "local_path": "existing.bin"}); !failed || !strings.Contains(body, "target_conflict") {
		t.Fatalf("old target replaced: %s", body)
	}
	got, err = os.ReadFile(filepath.Join(a.Root.Name(), "existing.bin"))
	if err != nil || string(got) != "keep" {
		t.Fatal("existing bytes changed", err)
	}
	corrupt.Store(true)
	if body, failed := call("download", map[string]any{"ref": ref, "local_path": "corrupt.bin"}); !failed || !strings.Contains(body, "content_mismatch") {
		t.Fatalf("ETag treated as bytes evidence: %s", body)
	}
	if _, err := os.Stat(filepath.Join(a.Root.Name(), "corrupt.bin")); !os.IsNotExist(err) {
		t.Fatal("corrupt download published")
	}
	before := reads.Load()
	for _, path := range []string{"../escape.bin", "/server/private/video.mp4"} {
		if body, failed := call("download", map[string]any{"ref": ref, "local_path": path}); !failed || !strings.Contains(body, "path_not_allowed") {
			t.Fatalf("client path became remote path: %s", body)
		}
	}
	revoked.Store(true)
	if body, failed := call("download", map[string]any{"ref": ref, "local_path": "revoked.bin"}); !failed || !strings.Contains(body, "forbidden") {
		t.Fatalf("cached preparation bypassed current authority: %s", body)
	}
	if reads.Load() != before {
		t.Fatal("rejected requests read restricted bytes")
	}
}
