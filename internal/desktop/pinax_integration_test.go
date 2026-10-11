//go:build integration

package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/pinax/pkg/pinaxclient"
)

// TestDesktopSessionPinaxIntegration builds the real owner, creates only an
// isolated CLI-authored fixture, and never enables body/writer scopes. It is
// explicit opt-in because ordinary unit tests must not build another owner.
func TestDesktopSessionPinaxIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	owner, err := filepath.Abs("../../../pinax")
	if err != nil {
		t.Fatal(err)
	}
	root := existingRoot(t)
	binaryPath := filepath.Join(root, "pinax")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", binaryPath, "./cmd/pinax")
	build.Dir = owner
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := build.Run(); err != nil {
		t.Fatal("owner fixture build failed")
	}
	if err := os.Chmod(binaryPath, 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(executable)
	// CLI setup alone gets a disposable home and PATH for its normal tools.
	// The production supervisor itself uses the stricter empty environment.
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
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Dir = root
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		// Never include owner stdout/stderr (paths, content) in failure evidence.
		out, err := cmd.Output()
		if err != nil {
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
	binary := ownerBinary{path: binaryPath, sha256: "sha256:" + hex.EncodeToString(digest[:]), manifestDigest: manifest.Facts["digest"]}
	vault := filepath.Join(root, "vault")
	run("", "init", vault, "--title", "Integration fixture", "--json")
	const content = "desktop-private-body-sentinel"
	run(content, "note", "add", "Integration note", "--dir", "index", "--stdin", "--vault", vault, "--json")
	before := vaultFingerprint(t, vault)
	p, err := startMetadataProcess(ctx, binary, vault)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	client, err := pinaxclient.New(pinaxclient.Config{BaseURL: p.ready.Address, Token: p.bearer, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal("client config failed")
	}
	source, err := client.ReaderSource(ctx)
	if err != nil {
		t.Fatal("metadata source failed")
	}
	if source.SessionID != p.ready.SessionID || source.BindingID != p.ready.BindingID {
		t.Fatal("source session mismatch")
	}
	notes, err := client.ReaderNotes(ctx, pinaxclient.ReaderPageQuery{})
	if err != nil || len(notes.Items) == 0 {
		t.Fatal("metadata notes failed")
	}
	note := notes.Items[0]
	_, err = client.ReaderDocument(ctx, pinaxclient.ReaderDocumentRequest{ObjectID: note.ObjectID, Revision: note.Revision})
	var denied *pinaxclient.Error
	if !errors.As(err, &denied) || denied.HTTPStatus != 403 || denied.Code != "insufficient_scope" {
		t.Fatal("body scope boundary failed")
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, tc := range []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/v1/reader/source", "", 401},
		// Reader-only bootstrap omits the writer route group altogether.
		{"POST", "/v1/writer/proposals", p.bearer, 404},
	} {
		request, err := http.NewRequestWithContext(ctx, tc.method, p.ready.Address+tc.path, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		if tc.token != "" {
			request.Header.Set("Authorization", "Bearer "+tc.token)
		}
		response, err := httpClient.Do(request)
		if err != nil {
			t.Fatal("boundary request failed")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		response.Body.Close()
		if err != nil || len(body) > 65536 || response.StatusCode != tc.want {
			t.Fatalf("boundary status: got %d want %d", response.StatusCode, tc.want)
		}
		if bytes.Contains(body, []byte(content)) || bytes.Contains(body, []byte(vault)) || bytes.Contains(body, []byte(p.bearer)) {
			t.Fatal("boundary leaked private data")
		}
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	after := vaultFingerprint(t, vault)
	if !reflect.DeepEqual(before, after) {
		for name, hash := range before {
			if after[name] != hash {
				t.Logf("fixture entry changed or removed: %s", name)
			}
		}
		for name := range after {
			if _, ok := before[name]; !ok {
				t.Logf("fixture entry added: %s", name)
			}
		}
		t.Fatal("metadata session changed vault files")
	}
	request, err := http.NewRequestWithContext(ctx, "GET", p.ready.Address+"/v1/reader/source", nil)
	if err != nil {
		t.Fatal(err)
	}
	if response, err := httpClient.Do(request); err == nil {
		response.Body.Close()
		t.Fatal("listener survived EOF")
	}
	// A second process demonstrates cancellation independently of explicit EOF.
	childCtx, stop := context.WithCancel(ctx)
	second, err := startMetadataProcess(childCtx, binary, vault)
	if err != nil {
		stop()
		t.Fatal(err)
	}
	stop()
	select {
	case <-second.done:
	case <-time.After(7 * time.Second):
		_ = second.Close()
		t.Fatal("owner survived cancellation")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if second.ready.SessionID == p.ready.SessionID || second.bearer == p.bearer {
		t.Fatal("session identity reused")
	}
	// Existing plain directories are not initialized by supervised startup.
	plain := filepath.Join(root, "plain")
	if err := os.Mkdir(plain, 0700); err != nil {
		t.Fatal(err)
	}
	if failed, err := startMetadataProcess(ctx, binary, plain); failed != nil || err == nil {
		t.Fatal("plain directory became a vault")
	}
	entries, err := os.ReadDir(plain)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid vault startup wrote files")
	}
	if !reflect.DeepEqual(before, vaultFingerprint(t, vault)) {
		t.Fatal("owner restart changed vault files")
	}
	// Exercise the actual loopback facade -> process -> typed owner path.
	facade, err := startFacade(ctx, binary)
	if err != nil {
		t.Fatal(err)
	}
	defer facade.Close()
	ticket := selected(t, facade.preflight, vault)
	encoded, _ := json.Marshal(ticket)
	status, data := facadeRequest(t, facade, "POST", "/v1/desktop/bindings/preview", string(encoded), nil)
	var preview struct {
		Data desktopcontract.BindingPreview `json:"data"`
	}
	if status != 200 || json.Unmarshal(data, &preview) != nil || preview.Data.Validate() != nil || preview.Data.Decision != "blocked" || preview.Data.Reason != "managed_scope_unverifiable" {
		t.Fatal("real metadata probe bypassed guard or failed")
	}
	for _, secret := range []string{vault, content, facade.token} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("facade leaked private data")
		}
	}
	encoded, _ = json.Marshal(confirmation(preview.Data))
	status, _ = facadeRequest(t, facade, "POST", "/v1/desktop/bindings", string(encoded), map[string]string{"Idempotency-Key": "fixture-original-key"})
	if status != 422 {
		t.Fatal("facade accepted unprotected binding")
	}
	facade.Close()
	if !reflect.DeepEqual(before, vaultFingerprint(t, vault)) {
		t.Fatal("facade mutated vault")
	}
	runHostProcessIntegration(t, ctx, binary, vault, env)
	if !reflect.DeepEqual(before, vaultFingerprint(t, vault)) {
		t.Fatal("CLI host mutated vault")
	}
}

// Test-only byte inventory, never a product fallback for reading Pinax data.
func vaultFingerprint(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[rel] = "directory"
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("unsupported fixture file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		result[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal("fixture inventory failed")
	}
	return result
}
