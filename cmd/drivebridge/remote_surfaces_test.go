package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge/internal/bridge"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

// cliContractFixture serves the public JSON contract for the CLI entry test:
// capabilities plus one readable batch.
func cliContractFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"instance_id": "SRV-CLI", "actions": bridge.RemoteSurfaceActionNames()}})
	})
	mux.HandleFunc("GET /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"batch_id": r.PathValue("id"), "phase": "sealed", "items": []any{}}})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func cliFixtureDirs(t *testing.T) (credential, root string) {
	t.Helper()
	dir := t.TempDir()
	credential = filepath.Join(dir, "token")
	if err := os.WriteFile(credential, []byte("fixture-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return credential, root
}

// TestFPRemoteSurfacesCLIEntryUsesApplicationContract pins that the CLI
// `remote --action` surface and the application service produce the same
// payload for the same fixture call, and that the new entry stays closed
// without the explicit opt-in switch.
func TestFPRemoteSurfacesCLIEntryUsesApplicationContract(t *testing.T) {
	server := cliContractFixture(t)
	credential, root := cliFixtureDirs(t)
	c := &cli{}
	c.mode.JSON = true
	args := map[string]any{"batch_id": "SRV-BATCH-CLI"}
	var out string
	var err error
	stdout, _ := capture(t, func() {
		err = runRemoteCall(c, server.URL, credential, root, "upload_batch_get", args, true)
	})
	out = stdout
	if err != nil {
		t.Fatalf("cli surface call: %v", err)
	}
	var envelope map[string]any
	if json.Unmarshal([]byte(out), &envelope) != nil {
		t.Fatalf("cli stdout is not one JSON envelope: %q", out)
	}
	data, _ := envelope["data"].(map[string]any)
	if data["batch_id"] != "SRV-BATCH-CLI" || data["phase"] != "sealed" {
		t.Fatalf("cli payload diverged from the contract: %v", data)
	}
	// Without the opt-in the same action stays unsupported: the legacy
	// adapter has no surface attached and reports unknown_tool.
	stdout2, _ := capture(t, func() {
		err = runRemoteCall(c, server.URL, credential, root, "upload_batch_get", args, false)
	})
	_ = stdout2
	if err == nil || err.Error() != "unknown_tool" {
		t.Fatalf("surface must stay closed without the opt-in: %v", err)
	}
	// The application service alone returns the identical payload.
	adapter, aerr := remoteclient.NewWithAigora(server.URL, credential, root, "")
	if aerr != nil {
		t.Fatal(aerr)
	}
	defer adapter.Close()
	if err := adapter.AttachRemoteSurfaces(); err != nil {
		t.Fatal(err)
	}
	direct, derr := adapter.Surfaces.Run(context.Background(), "upload_batch_get", args)
	if derr != nil {
		t.Fatal(derr)
	}
	directJSON, _ := json.Marshal(direct)
	if !strings.Contains(out, string(directJSON)) {
		t.Fatalf("cli envelope must embed the application payload:\n%s\nwant %s", out, directJSON)
	}
}
