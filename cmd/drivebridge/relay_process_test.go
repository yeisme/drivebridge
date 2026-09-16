package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRelayProcessInterruptionRecoversOriginalInput kills a real relay CLI
// process mid-transfer, then resumes the same operation in a fresh process:
// the owner never sees a second import and the completed input keeps its
// original reference.
func TestRelayProcessInterruptionRecoversOriginalInput(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the real CLI binary")
	}
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	local := filepath.Join(dir, "local")
	remote := filepath.Join(dir, "remote-files")
	for _, d := range []string{state, local, remote} {
		if err := os.Mkdir(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	conf := filepath.Join(dir, "rclone.conf")
	if err := os.WriteFile(conf, []byte("[nas]\ntype = alias\nremote = "+remote+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "owner.key")
	if err := os.WriteFile(key, []byte("fixture-owner-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "drivebridge.yaml"), []byte("state: "+state+"\nrclone_config: "+conf+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// A tiny real PNG so the owner-side media contract matches Eikona images.
	var png bytes.Buffer
	png.WriteString("\x89PNG\r\n\x1a\n")
	png.Write(make([]byte, 64))
	source := filepath.Join(local, "reference.png")
	if err := os.WriteFile(source, png.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(dir, "drivebridge")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", out)
	}
	run := func(args ...string) (string, string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	var imports, puts atomic.Int64
	var completed atomic.Bool
	var digest atomic.Value
	putArrived := make(chan struct{}, 1)
	hold := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(hold) }) }
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/input-requests/") {
			if r.Header.Get("X-Input-Grant") != "grant-process-transient" {
				w.WriteHeader(403)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/content") {
				puts.Add(1)
				data, _ := io.ReadAll(r.Body)
				sum := sha256.Sum256(data)
				digest.Store(hex.EncodeToString(sum[:]))
				select {
				case putArrived <- struct{}{}:
				default:
				}
				// Hold the transfer open so the CLI process can be killed
				// mid-operation, exactly like a machine going offline.
				<-hold
				w.WriteHeader(204)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/complete") {
				completed.Store(true)
				w.WriteHeader(204)
				return
			}
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-owner-key" {
			w.WriteHeader(401)
			return
		}
		respond := func(v any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
		}
		switch {
		case r.URL.Path == "/api/v1/drivebridge/imports" && r.Method == "POST":
			imports.Add(1)
			respond(map[string]any{"import_id": "imp_process", "input_request_id": "inp_process", "state": "awaiting_transfer"})
		case strings.HasSuffix(r.URL.Path, "/transfer"):
			respond(map[string]any{"request": map[string]string{"input_request_id": "inp_process"}, "transfer_url": "http://" + r.Host + "/input-requests/inp_process#grant=grant-process-transient"})
		case r.Method == "GET":
			st := "awaiting_transfer"
			if completed.Load() {
				st = "ready"
			}
			sum, _ := digest.Load().(string)
			respond(map[string]any{"import_id": "imp_process", "input_request_id": "inp_process", "state": st, "asset_uri": "eikona://asset/process", "sha256": sum, "domain_state": "unreviewed"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer owner.Close()
	defer release()

	if out, _, err := run("space", "add", "--space", "nas", "--backend", "generic", "--remote", "nas", "--local-root", local, "--max-bytes", "1048576"); err != nil {
		t.Fatalf("space add: %s %v", out, err)
	}
	if out, _, err := run("mkdir", "--space", "nas", "--path", "relay"); err != nil {
		t.Fatalf("mkdir: %s %v", out, err)
	}
	out, _, err := run("upload", "--space", "nas", "--file", "reference.png", "--dir", "relay", "--json")
	if err != nil {
		t.Fatalf("upload: %s %v", out, err)
	}
	var uploadEnvelope struct {
		Data struct {
			FileRef   string `json:"file_ref"`
			Operation struct {
				FileRef string `json:"file_ref"`
			} `json:"operation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &uploadEnvelope); err != nil {
		t.Fatalf("upload projection: %v %s", err, out)
	}
	fileRef := uploadEnvelope.Data.FileRef
	if fileRef == "" {
		fileRef = uploadEnvelope.Data.Operation.FileRef
	}
	if fileRef == "" {
		t.Fatalf("upload projection missing file_ref: %s", out)
	}
	if out, _, err := run("relay", "target-set", "images", "--space", "nas", "--project", "p", "--connection", "laptop", "--url", owner.URL, "--key-file", key, "--owner", "eikona"); err != nil {
		t.Fatalf("target-set: %s %v", out, err)
	}

	// Start the relay in a real process and kill it once bytes are in flight.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	interrupted := exec.CommandContext(ctx, binary, "relay", "start", fileRef, "--target", "images", "--idempotency-key", "process-one")
	interrupted.Dir = dir
	if err := interrupted.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-putArrived:
	case <-ctx.Done():
		_ = interrupted.Process.Kill()
		t.Fatal("transfer never started")
	}
	if err := interrupted.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = interrupted.Wait()

	// Find the interrupted operation in a fresh process, release the held
	// transfer, and resume the original operation.
	recent, _, err := run("recent", "--json")
	if err != nil {
		t.Fatalf("recent: %s %v", recent, err)
	}
	var envelope struct {
		Data struct {
			Operations []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"operations"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(recent), &envelope); err != nil {
		t.Fatalf("recent projection: %v %s", err, recent)
	}
	var opID string
	for _, op := range envelope.Data.Operations {
		if op.Kind == "creative_relay" {
			opID = op.ID
		}
	}
	if opID == "" {
		t.Fatal("interrupted relay operation missing from recent")
	}
	release()
	resumed, _, err := run("resume", opID, "--json")
	if err != nil {
		t.Fatalf("resume: %s %v", resumed, err)
	}
	if !strings.Contains(resumed, "eikona://asset/process") {
		t.Fatalf("resume did not surface the original input reference: %s", resumed)
	}
	if imports.Load() != 1 {
		t.Fatalf("owner saw %d imports; the original input must not be rebuilt", imports.Load())
	}
	if puts.Load() < 1 {
		t.Fatal("no transfer attempt reached the owner")
	}
}
