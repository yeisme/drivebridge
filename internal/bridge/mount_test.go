package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mountFixture(t *testing.T, writable bool) (*Service, *fakeProvider, string, *httptest.Server, string) {
	t.Helper()
	s, p, local, _ := setup(t)
	fixture(t, local)
	if _, err := s.Upload(context.Background(), "creative", "", "reference.png", "reference.png", "mount-fixture"); err != nil {
		t.Fatal(err)
	}
	var f File
	for range 500 {
		s.DB.First(&f, "space_id = ?", "creative")
		if f.Ref != "" {
			break
		}
	}
	if f.Ref == "" {
		t.Fatal("fixture upload never landed")
	}
	handler := &WebDAVHandler{VFS: NewVFS(s, []string{"creative"}, writable), Token: "fixture-session-token"}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return s, p, local, server, f.Name
}

func dav(t *testing.T, server *httptest.Server, method, path, token string, body io.Reader, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestWebDAVRootListsOnlyAuthorizedSpaces(t *testing.T) {
	_, _, _, server, _ := mountFixture(t, false)
	status, body := dav(t, server, "PROPFIND", "/", "fixture-session-token", nil, map[string]string{"Depth": "1"})
	if status != 207 || !strings.Contains(body, "creative") {
		t.Fatalf("root propfind: %d %s", status, body)
	}
	if strings.Contains(body, "other-space") || strings.Contains(body, casRoot) {
		t.Fatal("unlisted space or CAS leaked into the tree")
	}
	status, _ = dav(t, server, "PROPFIND", "/other-space/secret.txt", "fixture-session-token", nil, nil)
	if status != 404 {
		t.Fatalf("unauthorized space visible: %d", status)
	}
}

func TestWebDAVAuthAndAgentTokenRejected(t *testing.T) {
	_, _, _, server, _ := mountFixture(t, false)
	if status, _ := dav(t, server, "PROPFIND", "/", "", nil, nil); status != 401 {
		t.Fatalf("anonymous: %d", status)
	}
	agentToken := "db_" + strings.Repeat("a", 48)
	status, body := dav(t, server, "PROPFIND", "/", agentToken, nil, nil)
	if status != 403 || !strings.Contains(body, "agent_token_rejected") {
		t.Fatalf("agent token: %d %s", status, body)
	}
}

func TestWebDAVRangeGetFreezesVersion(t *testing.T) {
	_, _, _, server, name := mountFixture(t, false)
	status, body := dav(t, server, "GET", "/creative/"+name, "fixture-session-token", nil, map[string]string{"Range": "bytes=0-3"})
	if status != 206 || len(body) != 4 {
		t.Fatalf("range get: %d len=%d", status, len(body))
	}
	status, body = dav(t, server, "GET", "/creative/"+name, "fixture-session-token", nil, nil)
	if status != 200 || len(body) == 0 {
		t.Fatalf("full get: %d", status)
	}
}

func TestWebDAVReadOnlyAndLockUnsupported(t *testing.T) {
	_, _, _, server, name := mountFixture(t, false)
	if status, body := dav(t, server, "LOCK", "/creative/"+name, "fixture-session-token", nil, nil); status != 405 || !strings.Contains(body, "unsupported") {
		t.Fatalf("lock: %d %s", status, body)
	}
	for _, method := range []string{"MKCOL", "PUT", "MOVE", "DELETE"} {
		var headers map[string]string
		var body io.Reader
		if method == "MOVE" {
			headers = map[string]string{"Destination": "/creative/y"}
		} else if method == "PUT" {
			body = strings.NewReader("x")
		}
		status, text := dav(t, server, method, "/creative/x", "fixture-session-token", body, headers)
		if status == 201 || status == 204 || !strings.Contains(text, "read_only") {
			t.Fatalf("%s on readonly mount: %d %s", method, status, text)
		}
	}
}

func TestWebDAVWritableDeleteMapsToTrash(t *testing.T) {
	s, _, _, server, name := mountFixture(t, true)
	status, _ := dav(t, server, "DELETE", "/creative/"+name, "fixture-session-token", nil, nil)
	if status != 204 {
		t.Fatalf("delete: %d", status)
	}
	var f File
	s.DB.First(&f, "space_id = ?", "creative")
	if f.Trashed == nil {
		t.Fatal("DELETE did not map to trash")
	}
}

func TestMountStatusReadOnlyProjection(t *testing.T) {
	s, _, _, server, _ := mountFixture(t, false)
	listen := strings.TrimPrefix(server.URL, "http://")
	if err := s.RegisterMount(MountState{Adapter: "webdav", Listen: listen, Mode: "readonly", SpaceIDs: []string{"creative"}, Token: "secret-token", PID: 4242}); err != nil {
		t.Fatal(err)
	}
	out, err := s.Call(context.Background(), "mount_status", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(out)
	text := string(body)
	if !strings.Contains(text, `"alive":true`) || !strings.Contains(text, "webdav") {
		t.Fatalf("mount_status: %s", text)
	}
	if strings.Contains(text, "secret-token") {
		t.Fatal("mount_status leaked the session token")
	}
	if err = s.RegisterMount(MountState{Adapter: "webdav"}); err == nil || err.Error() != "mount_already_running" {
		t.Fatalf("second mount: %v", err)
	}
	s.ClearMount(4242)
	out, _ = s.Call(context.Background(), "mount_status", nil)
	body, _ = json.Marshal(out)
	if !strings.Contains(string(body), `"alive":false`) {
		t.Fatalf("cleared mount still alive: %s", body)
	}
	for _, tool := range s.Tools() {
		if tool.Name == "mount_start" || tool.Name == "mount_stop" {
			t.Fatal("agent-writable mount tool exposed")
		}
	}
	var names []string
	for _, tool := range s.Tools() {
		names = append(names, tool.Name)
	}
	found := false
	for _, n := range names {
		if n == "mount_status" {
			found = true
		}
	}
	if !found {
		t.Fatal("mount_status missing")
	}
	capabilities, _ := json.Marshal(s.Capabilities())
	if !strings.Contains(string(capabilities), "cli_only") || !strings.Contains(string(capabilities), "cas_reuse") {
		t.Fatalf("capabilities missing mount/cas entries: %s", capabilities)
	}
}
