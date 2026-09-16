package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalShareRemoteCopyAndRevocation(t *testing.T) {
	if testing.Short() {
		t.Skip("two-owner system fixture")
	}
	owner, err := filepath.Abs("../../../../backend-server/drivebridge")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(owner, "go.mod")); err != nil {
		t.Skip("remote owner checkout unavailable")
	}
	binary := filepath.Join(t.TempDir(), "fixture")
	build := exec.Command("go", "build", "-o", binary, "./cmd/drivebridge-fixture")
	build.Dir = owner
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build failed: %s", out)
	}
	cmd := exec.Command(binary, "--listen", "127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Signal(os.Interrupt); cmd.Wait() })
	line := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			line <- scanner.Text()
		}
	}()
	var base string
	select {
	case value := <-line:
		base = strings.TrimPrefix(value, "Offline fixture ready at ")
	case <-time.After(30 * time.Second):
		t.Fatal("fixture startup timed out")
	}
	if !strings.HasPrefix(base, "http://127.0.0.1:") {
		t.Fatal("fixture unavailable")
	}
	s, _, remoteRoot, socket := listFixture(t)
	client := dialUnix(t, socket)
	call := func(body any, out any) int {
		t.Helper()
		encoded, _ := json.Marshal(body)
		status, response := postSocket(t, client, "/files/share", string(encoded))
		if status == 200 && out != nil {
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal([]byte(response), &envelope) != nil || json.Unmarshal(envelope.Data, out) != nil {
				t.Fatal("invalid share projection")
			}
		}
		return status
	}
	credential := "dbu_" + strings.Repeat("22", 48)
	if status := call(map[string]any{"action": "configure", "destination": map[string]string{"url": base, "token": credential}}, nil); status != 200 {
		t.Fatal("configure", status)
	}
	stored, err := os.ReadFile(filepath.Join(s.state, "remote-share.connection"))
	if err != nil || strings.Contains(string(stored), credential) {
		t.Fatal("connection must be encrypted")
	}
	// Imported fixture users can bind their own disposable platform space.
	remoteClient, err := s.shareClient()
	if err != nil {
		t.Fatal(err)
	}
	var target struct {
		ID string `json:"id"`
	}
	if err = remoteClient.JSON(context.Background(), "POST", "/api/v1/spaces", "", map[string]any{"name": "Share target", "connection_id": "platform"}, &target); err != nil {
		t.Fatal(err)
	}
	var copy ShareCopy
	if status := call(map[string]any{"action": "create", "share": map[string]any{"space_id": "nas", "dir": "sub", "name": "Local delivery", "mode": "fixed", "target_space": target.ID, "days": 1}}, &copy); status != 200 {
		t.Fatal("create", status)
	}
	if err = s.syncShare(context.Background(), copy); err != nil {
		t.Fatal("copy", err)
	}
	s.DB.First(&copy, "id = ?", copy.ID)
	if copy.RemoteID == "" || len(copy.Refs) != 1 {
		t.Fatal("copy identity missing")
	}
	link, err := s.openShare(copy.Link)
	if err != nil || !strings.HasPrefix(string(link), base+"/#share/") {
		t.Fatal("link missing")
	}
	if err = s.syncShare(context.Background(), copy); err != nil {
		t.Fatal("replay", err)
	}
	var shares []struct {
		ID string `json:"id"`
	}
	if err = remoteClient.JSON(context.Background(), "GET", "/api/v1/shares", "", nil, &shares); err != nil || len(shares) != 1 {
		t.Fatal("share duplicated", err, len(shares))
	}
	// A frozen local version must never be replaced silently after source edits.
	if err = os.WriteFile(filepath.Join(remoteRoot, "sub", "inner.bin"), []byte("changed source bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.syncShare(context.Background(), copy); err == nil {
		t.Fatal("fixed source version changed silently")
	}
	if status := call(map[string]string{"action": "revoke", "id": copy.ID}, nil); status != 200 {
		t.Fatal("revoke", status)
	}
	if status := call(map[string]string{"action": "link", "id": copy.ID}, nil); status == http.StatusOK {
		t.Fatal("revoked link exposed")
	}
}
