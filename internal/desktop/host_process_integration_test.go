//go:build integration

package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

// runHostProcessIntegration extends the existing real owner fixture instead
// of building a second test stack or manufacturing a vault by hand.
func runHostProcessIntegration(t *testing.T, ctx context.Context, binary ownerBinary, vault string, env []string) {
	t.Helper()
	name := "drivebridge"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(filepath.Dir(binary.path), name)
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", executable, "./cmd/drivebridge")
	build.Dir = "../.."
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := build.Run(); err != nil {
		t.Fatal("desktop CLI build failed")
	}
	if err := os.Chmod(executable, 0700); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"preflight-eof", "invalid-frame", "digest-mismatch"} {
		t.Run("cli-"+scenario, func(t *testing.T) {
			childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			command := exec.CommandContext(childCtx, executable, "desktop", "serve")
			command.Dir = filepath.Dir(executable)
			command.Env = append(append([]string{}, env...), "DRIVEBRIDGE_CONFIG=/nonexistent/private-fixture")
			input, parentInput, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			defer parentInput.Close()
			parentOutput, output, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer parentOutput.Close()
			defer output.Close()
			command.Stdin, command.Stdout = input, output
			// Bounded diagnostic capture: never print upstream streams, including on
			// failure. os/exec waits for its stderr copier before Wait returns.
			stderr := &limitedHostLog{limit: 64 * 1024}
			command.Stderr = stderr
			command.WaitDelay = time.Second
			if err := command.Start(); err != nil {
				t.Fatal("host CLI failed to start")
			}
			input.Close()
			output.Close()
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			waited := false
			defer func() {
				parentInput.Close()
				if !waited {
					_ = command.Process.Kill()
					<-done
				}
			}()
			frames := make(chan []byte, 8)
			scanDone := make(chan struct{})
			go func() {
				defer close(scanDone)
				defer close(frames)
				scanner := bufio.NewScanner(parentOutput)
				scanner.Buffer(make([]byte, 1024), 64*1024)
				for scanner.Scan() {
					select {
					case frames <- append([]byte(nil), scanner.Bytes()...):
					case <-childCtx.Done():
						return
					}
				}
			}()
			token, err := opaqueID()
			if err != nil {
				t.Fatal(err)
			}
			nonce, err := opaqueID()
			if err != nil {
				t.Fatal(err)
			}
			var transcript bytes.Buffer
			event := func() HostEvent {
				t.Helper()
				select {
				case raw, ok := <-frames:
					if !ok {
						t.Fatal("host event stream closed early")
					}
					transcript.Write(raw)
					var value HostEvent
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.DisallowUnknownFields()
					if decoder.Decode(&value) != nil || value.SchemaVersion != HostVersion {
						t.Fatal("invalid host event envelope")
					}
					return value
				case <-childCtx.Done():
					t.Fatal("host event timeout")
					return HostEvent{}
				}
			}
			encoder := json.NewEncoder(parentInput)
			if scenario == "invalid-frame" {
				_, err = parentInput.WriteString(`{"bearer":"` + token + `","native_path":"private-fixture","argv":[]}` + "\n")
			} else {
				digest := binary.sha256
				if scenario == "digest-mismatch" {
					digest = "sha256:" + strings.Repeat("0", 64)
				}
				err = encoder.Encode(HostBootstrap{SchemaVersion: HostVersion, SessionNonce: nonce, Bearer: token, BinaryDigest: digest, ManifestDigest: binary.manifestDigest})
			}
			if err != nil {
				t.Fatal("bootstrap pipe failed")
			}
			ready := event()
			if scenario == "preflight-eof" {
				if ready.Type != "ready" || ready.SessionNonce != nonce || !strings.HasPrefix(ready.Address, "http://127.0.0.1:") {
					t.Fatal("invalid CLI ready")
				}
				f := &facade{address: ready.Address, token: token}
				if err := encoder.Encode(HostSelection{SchemaVersion: HostVersion, Action: "select", RequestID: "choose-fixture", NativePath: vault}); err != nil {
					t.Fatal("select pipe failed")
				}
				selection := event()
				if selection.Type != "selection" || selection.RequestID != "choose-fixture" || !facadeKey.MatchString(selection.SelectionID) {
					t.Fatal("CLI selection missing")
				}
				body, _ := json.Marshal(desktopcontract.NativeSelection{NativeSelectionID: selection.SelectionID})
				status, data := facadeRequest(t, f, "POST", "/v1/desktop/bindings/preview", string(body), nil)
				var preview struct {
					Data desktopcontract.BindingPreview `json:"data"`
				}
				if status != 200 || json.Unmarshal(data, &preview) != nil || preview.Data.Validate() != nil || preview.Data.Decision != "blocked" || preview.Data.Reason != "managed_scope_unverifiable" {
					t.Fatal("CLI real owner probe did not preserve guard refusal")
				}
				transcript.Write(data)
				body, _ = json.Marshal(confirmation(preview.Data))
				status, data = facadeRequest(t, f, "POST", "/v1/desktop/bindings", string(body), map[string]string{"Idempotency-Key": "original-cli-key"})
				if status != 422 {
					t.Fatal("CLI accepted unprotected binding")
				}
				transcript.Write(data)
				if err := encoder.Encode(HostCancel{SchemaVersion: HostVersion, Action: "cancel", RequestID: "cancel-fixture", SelectionID: preview.Data.PreviewID}); err != nil {
					t.Fatal("cancel pipe failed")
				}
				cancelled := event()
				if cancelled.Type != "cancelled" || cancelled.RequestID != "cancel-fixture" {
					t.Fatal("CLI cancel missing")
				}
				status, _ = facadeRequest(t, f, "POST", "/v1/desktop/bindings", string(body), map[string]string{"Idempotency-Key": "original-cli-key"})
				if status != 410 {
					t.Fatal("cancelled CLI preview retained")
				}
				parentInput.Close()
				shutdown := event()
				if shutdown.Type != "shutdown" || shutdown.SessionNonce != nonce {
					t.Fatal("CLI EOF shutdown missing")
				}
			} else {
				if ready.Type != "error" || ready.Code != "desktop_host_invalid" {
					t.Fatal("bad CLI bootstrap not safely rejected")
				}
				parentInput.Close()
			}
			select {
			case err := <-done:
				waited = true
				if scenario == "preflight-eof" && err != nil {
					t.Fatal("CLI EOF exit failed")
				}
				if scenario != "preflight-eof" && err == nil {
					t.Fatal("invalid CLI exit succeeded")
				}
			case <-childCtx.Done():
				t.Fatal("CLI did not exit")
			}
			select {
			case <-scanDone:
			case <-childCtx.Done():
				t.Fatal("CLI output handles survived exit")
			}
			for raw := range frames {
				transcript.Write(raw)
			}
			for _, secret := range []string{token, vault, binary.path, "desktop-private-body-sentinel", "private-fixture"} {
				if bytes.Contains(transcript.Bytes(), []byte(secret)) || bytes.Contains(stderr.data.Bytes(), []byte(secret)) {
					t.Fatal("CLI stream leaked private data")
				}
			}
			if stderr.overflow {
				t.Fatal("CLI stderr exceeded budget")
			}
			if scenario == "preflight-eof" {
				transport := &http.Transport{Proxy: nil}
				defer transport.CloseIdleConnections()
				client := &http.Client{Transport: transport, Timeout: time.Second}
				response, err := client.Get(ready.Address + "/v1/desktop/capabilities")
				if err == nil {
					response.Body.Close()
					t.Fatal("CLI listener survived process exit")
				}
			}
		})
	}
}

type limitedHostLog struct {
	data     bytes.Buffer
	limit    int
	overflow bool
}

func (l *limitedHostLog) Write(p []byte) (int, error) {
	n := len(p)
	remaining := l.limit - l.data.Len()
	if n > remaining {
		l.overflow = true
		p = p[:remaining]
	}
	_, _ = l.data.Write(p)
	return n, nil
}

var _ io.Writer = (*limitedHostLog)(nil)
