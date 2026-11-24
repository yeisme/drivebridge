//go:build integration

package desktop

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// This is the D08/D14.3 admission precondition end to end: a real `drivebridge
// desktop serve` process, a real Pinax owner binary over a disposable vault, and a
// real fenced owner state holding the guard. Nothing is mocked — the preview can
// only become `ready` if the host attached the owner admission after the private
// bootstrap, and the confirmation can only become `active` if the owner CAS
// install ran.
func TestDesktopHostBindingWithRealOwnerState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	owner, err := filepath.Abs("../../../pinax")
	if err != nil {
		t.Fatal(err)
	}
	root := existingRoot(t)
	pinaxPath := filepath.Join(root, "pinax")
	if runtime.GOOS == "windows" {
		pinaxPath += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", pinaxPath, "./cmd/pinax")
	build.Dir = owner
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := build.Run(); err != nil {
		t.Fatal("owner fixture build failed")
	}
	if err := os.Chmod(pinaxPath, 0700); err != nil {
		t.Fatal(err)
	}
	ownerBytes, err := os.ReadFile(pinaxPath)
	if err != nil {
		t.Fatal(err)
	}
	ownerDigest := sha256.Sum256(ownerBytes)
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + home, "XDG_CACHE_HOME=" + home, "PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	if v := os.Getenv("SystemRoot"); v != "" {
		env = append(env, "SystemRoot="+v)
	}
	run := func(input string, args ...string) []byte {
		t.Helper()
		command := exec.CommandContext(ctx, pinaxPath, args...)
		command.Dir = root
		command.Env = env
		command.Stdin = strings.NewReader(input)
		out, runErr := command.Output()
		if runErr != nil {
			t.Fatal("owner fixture command failed: " + args[0])
		}
		return out
	}
	var manifest struct {
		Status string            `json:"status"`
		Facts  map[string]string `json:"facts"`
	}
	if json.Unmarshal(run("", "api", "manifest", "--json"), &manifest) != nil || manifest.Status != "success" || !fixedDigest(manifest.Facts["digest"]) {
		t.Fatal("owner manifest invalid")
	}
	vault := filepath.Join(root, "vault")
	run("", "init", vault, "--title", "Admission fixture", "--json")
	run("disposable admission body", "note", "add", "Admission note", "--dir", "index", "--stdin", "--vault", vault, "--json")

	// Owner state: one space registered for the disposable vault, then fenced
	// through the owner protocol so the guard has a provable layout.
	state := filepath.Join(root, "owner-state")
	service, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.DB.Create(&bridge.Space{ID: "vault", Remote: "remote", LocalRoot: vault, MaxBytes: 1 << 20}).Error; err != nil {
		service.Close()
		t.Fatal(err)
	}
	if err = service.DB.Create(&bridge.AdoptRecord{ID: "owner-vault", Consumer: "pinax", Kind: "local", SpaceID: "vault", Location: vault, IdempotencyKey: "owner-vault"}).Error; err != nil {
		service.Close()
		t.Fatal(err)
	}
	service.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "cli", "migration-binding-e2e")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		t.Fatal(err)
	}

	// Real packaged-equivalent CLI next to the owner binary.
	cliPath := filepath.Join(root, "drivebridge")
	if runtime.GOOS == "windows" {
		cliPath += ".exe"
	}
	cliBuild := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", cliPath, "./cmd/drivebridge")
	cliBuild.Dir = "../.."
	cliBuild.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := cliBuild.Run(); err != nil {
		t.Fatal("desktop CLI build failed")
	}
	if err := os.Chmod(cliPath, 0700); err != nil {
		t.Fatal(err)
	}

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
	command := exec.CommandContext(ctx, cliPath, "desktop", "serve")
	command.Dir = root
	// Owner state comes from configuration that predates the process: the
	// desktop surface has no flag for it.
	command.Env = append(append([]string{}, env...), "DRIVEBRIDGE_STATE="+state, "DRIVEBRIDGE_CONFIG=/nonexistent/admission-fixture")
	command.Stdin, command.Stdout = input, output
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
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(parentOutput)
		scanner.Buffer(make([]byte, 1024), 64*1024)
		for scanner.Scan() {
			select {
			case frames <- append([]byte(nil), scanner.Bytes()...):
			case <-ctx.Done():
				return
			}
		}
	}()
	event := func() HostEvent {
		t.Helper()
		select {
		case raw, ok := <-frames:
			if !ok {
				t.Fatal("host event stream closed early")
			}
			var value HostEvent
			if json.Unmarshal(raw, &value) != nil || value.SchemaVersion != HostVersion {
				t.Fatal("invalid host event envelope")
			}
			return value
		case <-ctx.Done():
			t.Fatal("host event timeout")
			return HostEvent{}
		}
	}
	token, err := opaqueID()
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := opaqueID()
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(parentInput)
	if err := encoder.Encode(HostBootstrap{SchemaVersion: HostVersion, SessionNonce: nonce, Bearer: token, BinaryDigest: "sha256:" + hex.EncodeToString(ownerDigest[:]), ManifestDigest: manifest.Facts["digest"]}); err != nil {
		t.Fatal(err)
	}
	ready := event()
	if ready.Type != "ready" || ready.SessionNonce != nonce || !strings.HasPrefix(ready.Address, "http://127.0.0.1:") {
		t.Fatalf("ready event: %#v", ready)
	}
	if err := encoder.Encode(HostSelection{SchemaVersion: HostVersion, Action: "select", RequestID: "select-binding", NativePath: vault}); err != nil {
		t.Fatal(err)
	}
	selection := event()
	if selection.Type != "selection" || !facadeKey.MatchString(selection.SelectionID) {
		t.Fatalf("selection event: %#v", selection)
	}
	host := &facade{address: ready.Address, token: token}
	body, _ := json.Marshal(desktopcontract.NativeSelection{NativeSelectionID: selection.SelectionID})
	status, data := facadeRequest(t, host, "POST", "/v1/desktop/bindings/preview", string(body), nil)
	var preview struct {
		Data desktopcontract.BindingPreview `json:"data"`
	}
	if status != 200 || json.Unmarshal(data, &preview) != nil {
		t.Fatalf("preview: %d %s", status, string(data))
	}
	if preview.Data.Decision != "ready" || preview.Data.Reason != "none" || preview.Data.ExpectedGuardRevision != 0 {
		t.Fatalf("admission preview: %#v", preview.Data)
	}
	body, _ = json.Marshal(confirmation(preview.Data))
	status, data = facadeRequest(t, host, "POST", "/v1/desktop/bindings", string(body), map[string]string{"Idempotency-Key": "binding-e2e-key"})
	if status != 200 {
		t.Fatalf("confirm: %d %s", status, string(data))
	}
	var receipt struct {
		Data desktopcontract.BindingMutationReceipt `json:"data"`
	}
	if json.Unmarshal(data, &receipt) != nil || receipt.Data.Action != "confirm" || receipt.Data.Binding.State != "active" || !receipt.Data.Binding.ProtectionActive {
		t.Fatalf("confirm receipt: %s", string(data))
	}
	// Read surface: the same bound source must serve a note page and the fixed
	// revision Markdown source through the host, with the owner binary real.
	status, data = facadeRequest(t, host, "GET", "/v1/desktop/sources/"+receipt.Data.Binding.SourceID+"/notes", "", nil)
	if status != 200 {
		t.Fatalf("notes: %d %s", status, string(data))
	}
	var notes struct {
		Data struct {
			Items []struct {
				ObjectID string `json:"object_id"`
				Revision string `json:"revision"`
				Title    string `json:"title"`
			} `json:"items"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &notes) != nil || len(notes.Data.Items) == 0 {
		t.Fatalf("notes payload: %s", string(data))
	}
	note := notes.Data.Items[0]
	status, data = facadeRequest(t, host, "GET", "/v1/desktop/sources/"+receipt.Data.Binding.SourceID+"/writer/editor-source?object_id="+note.ObjectID+"&revision="+note.Revision, "", nil)
	if status != 200 {
		t.Fatalf("editor source: %d %s", status, string(data))
	}
	var source struct {
		Data struct {
			Body  string `json:"body"`
			Bytes int    `json:"bytes"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &source) != nil || source.Data.Bytes == 0 || !strings.Contains(source.Data.Body, "disposable admission body") {
		t.Fatalf("editor source payload: %s", string(data))
	}
	body, _ = json.Marshal(map[string]string{"object_id": note.ObjectID, "revision": note.Revision, "body": "disposable admission body\n\nappended by the host test"})
	status, data = facadeRequest(t, host, "POST", "/v1/desktop/sources/"+receipt.Data.Binding.SourceID+"/writer/editor:propose", string(body), nil)
	if status != 200 {
		t.Fatalf("editor propose: %d %s", status, string(data))
	}
	var proposal struct {
		Data struct {
			PreviewDigest string `json:"preview_digest"`
			State         string `json:"state"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &proposal) != nil || !strings.HasPrefix(proposal.Data.PreviewDigest, "sha256:") {
		t.Fatalf("editor proposal payload: %s", string(data))
	}
	// Release revokes the binding: the same source id must stop resolving.
	body, _ = json.Marshal(map[string]any{"expected_revision": receipt.Data.Binding.Revision, "confirmed": true})
	status, data = facadeRequest(t, host, "POST", "/v1/desktop/bindings/"+receipt.Data.Binding.BindingID+"/release", string(body), map[string]string{"Idempotency-Key": "release-e2e"})
	if status != 200 {
		t.Fatalf("release: %d %s", status, string(data))
	}
	status, data = facadeRequest(t, host, "GET", "/v1/desktop/sources/"+receipt.Data.Binding.SourceID+"/notes", "", nil)
	if status != 404 || !strings.Contains(string(data), "source_unknown") {
		t.Fatalf("released source: %d %s", status, string(data))
	}
	parentInput.Close()
	if shutdown := event(); shutdown.Type != "shutdown" || shutdown.SessionNonce != nonce {
		t.Fatalf("shutdown: %#v", shutdown)
	}
	select {
	case err := <-done:
		waited = true
		if err != nil {
			t.Fatal("host exit failed")
		}
	case <-ctx.Done():
		t.Fatal("host did not exit")
	}
	if stderr.overflow {
		t.Fatal("host stderr exceeded budget")
	}
	for _, secret := range []string{token, vault, state} {
		if bytes.Contains(data, []byte(secret)) || bytes.Contains(stderr.data.Bytes(), []byte(secret)) {
			t.Fatal("host stream leaked private data")
		}
	}
}

// A writer session is a distinct owner session: the same binary must open it with
// writer scopes and readonly=false, and the reader-only session must not be
// reusable for writes.
func TestOwnerWriterSessionOpensWithWriteScopes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	owner, err := filepath.Abs("../../../pinax")
	if err != nil {
		t.Fatal(err)
	}
	root := existingRoot(t)
	pinaxPath := filepath.Join(root, "pinax")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", pinaxPath, "./cmd/pinax")
	build.Dir = owner
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := build.Run(); err != nil {
		t.Fatal("owner fixture build failed")
	}
	if err := os.Chmod(pinaxPath, 0700); err != nil {
		t.Fatal(err)
	}
	ownerBytes, err := os.ReadFile(pinaxPath)
	if err != nil {
		t.Fatal(err)
	}
	ownerDigest := sha256.Sum256(ownerBytes)
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + home, "XDG_CACHE_HOME=" + home, "PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	run := func(input string, args ...string) []byte {
		t.Helper()
		command := exec.CommandContext(ctx, pinaxPath, args...)
		command.Dir = root
		command.Env = env
		command.Stdin = strings.NewReader(input)
		out, runErr := command.Output()
		if runErr != nil {
			t.Fatal("owner fixture command failed: " + args[0])
		}
		return out
	}
	var manifest struct {
		Status string            `json:"status"`
		Facts  map[string]string `json:"facts"`
	}
	if json.Unmarshal(run("", "api", "manifest", "--json"), &manifest) != nil || manifest.Status != "success" {
		t.Fatal("owner manifest invalid")
	}
	binary := ownerBinary{path: pinaxPath, sha256: "sha256:" + hex.EncodeToString(ownerDigest[:]), manifestDigest: manifest.Facts["digest"]}
	vault := filepath.Join(root, "vault")
	run("", "init", vault, "--title", "Writer fixture", "--json")

	writer, err := startOwnerSession(ctx, binary, vault, []string{"writer.editor"}, false)
	if err != nil {
		t.Fatalf("writer session: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("writer session close: %v", err)
	}
	reader, err := startOwnerSession(ctx, binary, vault, []string{"reader.metadata", "reader.body"}, true)
	if err != nil {
		t.Fatalf("reader session: %v", err)
	}
	if err = reader.Close(); err != nil {
		t.Fatalf("reader session close: %v", err)
	}
	// The scope invariant is enforced before any child starts.
	if _, err = startOwnerSession(ctx, binary, vault, []string{"writer.editor"}, true); err == nil || err.Error() != "session_request_invalid" {
		t.Fatalf("writer scope with readonly: %v", err)
	}
	if _, err = startOwnerSession(ctx, binary, vault, []string{"reader.metadata"}, false); err == nil || err.Error() != "session_request_invalid" {
		t.Fatalf("reader scope without readonly: %v", err)
	}
}
