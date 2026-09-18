package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

// openListFixture serves the two surfaces a real OpenList exposes: the
// unauthenticated /api/public/settings JSON and the /dav WebDAV tree behind
// basic auth, so the openlist onboarding path runs end to end offline.
// x/net/webdav is mounted under Prefix "/dav" so hrefs echo the full
// /dav-prefixed paths, which is what rclone's webdav backend expects when
// relativizing listings against its configured root URL (real OpenList
// behaves the same). Stripping the prefix makes rclone silently drop every
// child from directory listings.
func openListFixture(t *testing.T, user, pass string) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	dav := &webdav.Handler{
		FileSystem: webdav.Dir(root),
		Prefix:     "/dav",
		LockSystem: webdav.NewMemLS(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/settings", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"version":"fixture-3.41.0","site_title":"OpenList"}}`))
	})
	mux.Handle("/dav/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != user || secret != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="OpenList"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		dav.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, root
}

// TestOpenListPathEndToEnd covers the full guided path offline: remote
// creation with obscured credentials, probe with OpenList detection, and a
// real upload/list roundtrip through the generic webdav provider.
func TestOpenListPathEndToEnd(t *testing.T) {
	installRcloneConfig(t)
	srv, remoteRoot := openListFixture(t, "media", "s3cret")
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()

	if err := EnsureWebDAVRemote("db-openlist-cloud", srv.URL+"/dav/", "media", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if !WebDAVRemoteMatches("db-openlist-cloud", srv.URL+"/dav/", "media", "s3cret") {
		t.Fatal("matching credentials must be reusable")
	}

	res, err := ProbeStorage(context.Background(), "openlist", "db-openlist-cloud", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reachable {
		t.Fatal("webdav root must be reachable")
	}
	if res.Aggregator != "openlist" || res.AggregatorVersion != "fixture-3.41.0" {
		t.Fatalf("openlist detection missing: %+v", res)
	}
	if res.Capabilities["direct_url"] || res.Capabilities["reference_handoff"] {
		t.Fatal("openlist spaces stay transfer-only")
	}

	s, err := New(state, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.AddSpace(Space{ID: "cloud", Remote: "db-openlist-cloud", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(local, "source.txt")
	if err = os.WriteFile(src, []byte("openlist payload"), 0600); err != nil {
		t.Fatal(err)
	}
	op, err := s.Upload(context.Background(), "cloud", "", src, "source.txt", "openlist-e2e")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" || op.Bytes != int64(len("openlist payload")) {
		t.Fatalf("upload not completed: %+v", op)
	}

	// The file really landed on the WebDAV side behind the basic-auth /dav.
	onDisk, err := filepath.Glob(filepath.Join(remoteRoot, "*source.txt"))
	if err != nil || len(onDisk) != 1 {
		t.Fatalf("remote side missing upload: glob=%v err=%v", onDisk, err)
	}

	files, err := s.List(context.Background(), "cloud", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range files {
		if f.Name == op.Target || strings.HasSuffix(f.Name, "source.txt") {
			found = true
		}
	}
	if !found {
		raw, _ := os.ReadDir(remoteRoot)
		rawNames := make([]string, 0, len(raw))
		for _, e := range raw {
			rawNames = append(rawNames, e.Name())
		}
		t.Fatalf("uploaded file not listed: listed=%d onDisk=%v", len(files), rawNames)
	}
}
