package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSemanticHandoffCLIParsesPinnedDownloadAndPreservesTarget(t *testing.T) {
	const ref = "drivebridge://cli-instance/file/file-one/version/version-one"
	raw := []byte("CLI pinned version content")
	sum := sha256.Sum256(raw)
	var contentReads atomic.Int64
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+"fixture-token" {
			t.Error("CLI missing service authority")
		}
		switch r.URL.Path {
		case "/api/v1/references":
			if r.URL.Query().Get("ref") != ref {
				t.Error("CLI did not preserve pinned reference")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref": ref, "file": map[string]any{"id": "file-one", "version_id": "version-one"}}})
		case "/api/v1/files/file-one/content":
			contentReads.Add(1)
			if r.URL.Query().Get("version") != "version-one" {
				t.Error("CLI requested mutable head")
			}
			w.Header().Set("X-Content-SHA256", hex.EncodeToString(sum[:]))
			_, _ = w.Write(raw)
		default:
			t.Errorf("unexpected CLI server request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(owner.Close)
	credential, local := cliFixtureDirs(t)
	// Isolate configuration while executing the real Cobra command tree and
	// stdin JSON parser, not the runRemoteCall helper used by older tests.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, key := range []string{"DRIVEBRIDGE_CONFIG", "DRIVEBRIDGE_AIGORA_URL"} {
		t.Setenv(key, "")
	}
	call := func(path string) (string, error) {
		t.Helper()
		input, err := os.CreateTemp(t.TempDir(), "stdin-*")
		if err != nil {
			t.Fatal(err)
		}
		defer input.Close()
		if err := json.NewEncoder(input).Encode(map[string]any{"file_ref": ref, "local_path": path}); err != nil {
			t.Fatal(err)
		}
		if _, err := input.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		old := os.Stdin
		os.Stdin = input
		defer func() { os.Stdin = old }()
		root := newRootCommand(&cli{})
		root.SetArgs([]string{"--json", "remote", "--remote-url", owner.URL, "--remote-token-file", credential, "--local-root", local, "--action", "download"})
		var executeErr error
		stdout, _ := capture(t, func() { executeErr = root.Execute() })
		for _, secret := range []string{"fixture-token", owner.URL, credential} {
			if strings.Contains(stdout, secret) {
				t.Fatal("CLI receipt leaked private details")
			}
		}
		return stdout, executeErr
	}
	stdout, err := call("received.bin")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Status string `json:"status"`
			FileID string `json:"file_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || envelope.Data.Status != "completed" || envelope.Data.FileID != "file-one" {
		t.Fatalf("CLI success is not a receipt: %s (%v)", stdout, err)
	}
	got, err := os.ReadFile(filepath.Join(local, "received.bin"))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("CLI saved wrong bytes", err)
	}
	if err := os.WriteFile(filepath.Join(local, "preserve.bin"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := call("preserve.bin"); err == nil || err.Error() != "target_conflict" {
		t.Fatal("CLI overwrote existing target", err)
	}
	got, err = os.ReadFile(filepath.Join(local, "preserve.bin"))
	if err != nil || string(got) != "original" {
		t.Fatal("CLI lost previous target", err)
	}
	before := contentReads.Load()
	if _, err := call("/server-only/private.mp4"); err == nil || err.Error() != "path_not_allowed" || contentReads.Load() != before {
		t.Fatal("CLI confused server and client paths", err)
	}
}
