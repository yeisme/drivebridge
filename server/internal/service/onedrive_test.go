package service

import (
	"bytes"
	"context"
	"encoding/json"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memoryStore) Put(_ context.Context, k string, r io.Reader, _ int64) error {
	b, e := io.ReadAll(r)
	m.mu.Lock()
	m.data[k] = b
	m.mu.Unlock()
	return e
}
func (m *memoryStore) Get(_ context.Context, k string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return io.NopCloser(bytes.NewReader(m.data[k])), nil
}
func (m *memoryStore) Delete(_ context.Context, k string) error {
	m.mu.Lock()
	delete(m.data, k)
	m.mu.Unlock()
	return nil
}
func (m *memoryStore) Check(context.Context) error { return nil }
func (m *memoryStore) Stat(_ context.Context, k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[k]; !ok {
		return code("object_missing")
	}
	return nil
}

type graphRoundTrip func(*http.Request) (*http.Response, error)

func (f graphRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOneDriveImportExportAndConnectionBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	storage := &memoryStore{data: map[string][]byte{}}
	s.storage = func(S3Config, bool) (Storage, error) { return storage, nil }
	if e = s.SetPlatform(context.Background(), S3Config{}); e != nil {
		t.Fatal(e)
	}
	alice, e := s.ensureUser("fixture", "alice", "Alice")
	if e != nil {
		t.Fatal(e)
	}
	bob, _ := s.ensureUser("fixture", "bob", "Bob")
	p := Principal{UserID: alice.ID}
	var space Space
	s.DB.First(&space, "owner = ?", alice.ID)
	secret, _ := s.seal(p.UserID, oneDriveCredential{Token: oauth2.Token{AccessToken: "fixture-private", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}, DriveID: "drive-a"})
	conn := Connection{ID: ID(), Owner: p.UserID, Kind: "onedrive", Name: "fixture", Secret: secret, Status: "verified"}
	s.DB.Create(&conn)
	var mu sync.Mutex
	source := []byte("original external content")
	revision := "v1"
	exported := []byte(nil)
	uploads := 0
	s.graphHTTP = &http.Client{Transport: graphRoundTrip(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer fixture-private" || r.URL.Host != "graph.microsoft.com" {
			t.Error("incorrect identity target")
		}
		status := 200
		var out any
		switch {
		case strings.HasSuffix(r.URL.Path, "/createUploadSession"):
			uploads++
			out = map[string]string{"uploadUrl": "https://fixture.files.1drv.com/session"}
		case strings.Contains(r.URL.Path, "/root:"):
			if exported == nil {
				status = 404
				out = map[string]string{"error": "missing"}
			} else {
				out = map[string]any{"id": "exported", "eTag": "e1", "size": len(exported), "@microsoft.graph.downloadUrl": "https://fixture.files.1drv.com/exported"}
			}
		default:
			out = map[string]any{"id": "source", "name": "source.txt", "size": len(source), "eTag": revision, "file": map[string]string{"mimeType": "text/plain"}, "@microsoft.graph.downloadUrl": "https://fixture.files.1drv.com/source"}
		}
		b, _ := json.Marshal(out)
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(b)), Header: make(http.Header), Request: r}, nil
	})}
	s.sensitiveTransfer = func(_ context.Context, method, raw string, r io.Reader, _ map[string]string) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		var b []byte
		if method == "PUT" {
			exported, _ = io.ReadAll(r)
			b = []byte(`{"id":"exported","eTag":"e1"}`)
		} else if strings.HasSuffix(raw, "exported") {
			b = exported
		} else {
			b = source
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}, nil
	}
	grant, _, grantErr := s.createAgent(p, "connector", space.ID, "imports", "read,write,import,export", conn.ID, 7)
	if grantErr != nil {
		t.Fatal(grantErr)
	}
	browse := httptest.NewRequest("GET", "/", nil)
	browse.SetPathValue("id", conn.ID)
	if err := s.browseOneDrive(httptest.NewRecorder(), browse, Principal{UserID: p.UserID, Agent: &grant}); err != nil {
		t.Fatal("authorized connector browse rejected")
	}
	blocked := grant
	blocked.ConnectionID = ""
	if err := s.browseOneDrive(httptest.NewRecorder(), browse, Principal{UserID: p.UserID, Agent: &blocked}); err == nil {
		t.Fatal("unbound connector browse accepted")
	}
	if _, _, e = s.oneDrive(Principal{UserID: bob.ID}, conn.ID); e == nil {
		t.Fatal("connection escaped owner")
	}
	request := func(handler endpoint, id string, body any, key string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(b))
		req.SetPathValue("id", id)
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		if e := handler(w, req, p); e != nil {
			t.Fatal(e)
		}
		return w
	}
	response := request(s.importOneDrive, conn.ID, map[string]any{"item_id": "source", "version": "v1", "space_id": space.ID, "path": "imports/source.txt"}, "import-once")
	var result struct {
		Data Operation `json:"data"`
	}
	json.Unmarshal(response.Body.Bytes(), &result)
	if result.Data.Status != "completed" {
		t.Fatal("import incomplete")
	}
	request(s.importOneDrive, conn.ID, map[string]any{"item_id": "source", "version": "v1", "space_id": space.ID, "path": "imports/source.txt"}, "import-once")
	var count int64
	s.DB.Model(&File{}).Where("owner = ?", p.UserID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate import")
	}
	mu.Lock()
	revision = "changed"
	source = []byte("changed remote file")
	mu.Unlock()
	_, _, b, e := s.Download(context.Background(), p, result.Data.FileID, "")
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(b)
	b.Close()
	if string(data) != "original external content" {
		t.Fatal("external changes mutated imported snapshot")
	}
	request(s.exportOneDrive, conn.ID, map[string]string{"file_id": result.Data.FileID}, "export-once")
	request(s.exportOneDrive, conn.ID, map[string]string{"file_id": result.Data.FileID}, "export-once")
	if uploads != 1 || string(exported) != "original external content" {
		t.Fatal("export replay or content mismatch")
	}
	folder, err := s.ExportDirectory(context.Background(), p, DirectoryExportRequest{SpaceID: space.ID, Path: "imports", ConnectionID: conn.ID}, "export-dir")
	if err != nil || folder.Status != "completed" || folder.TotalFiles != 1 {
		t.Fatalf("directory export %+v %v", folder, err)
	}
	replay, err := s.ExportDirectory(context.Background(), p, DirectoryExportRequest{SpaceID: space.ID, Path: "imports", ConnectionID: conn.ID}, "export-dir")
	if err != nil || replay.ID != folder.ID {
		t.Fatalf("directory export replay %+v %v", replay, err)
	}
	s.DB.Model(&conn).Update("disabled", true)
	if _, _, e = s.oneDrive(p, conn.ID); e == nil {
		t.Fatal("disconnected credentials usable")
	}
	_, _, b, e = s.Download(context.Background(), p, result.Data.FileID, "")
	if e != nil {
		t.Fatal("disconnect destroyed imported snapshot")
	}
	b.Close()
}

func TestProductionCookieCannotBeSetForAParentDomain(t *testing.T) {
	s := &Service{Config: Config{Development: false}}
	w := httptest.NewRecorder()
	s.cookie(w, "db_session", "fixture", 60)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("cookie missing")
	}
	c := cookies[0]
	if c.Name != "__Host-db_session" || !c.Secure || !c.HttpOnly || c.Domain != "" || c.Path != "/" || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("production session cookie boundary weakened")
	}
}

type cancellingStore struct {
	Storage
	started chan struct{}
}

func (c cancellingStore) Put(ctx context.Context, _ string, _ io.Reader, _ int64) error {
	close(c.started)
	<-ctx.Done()
	return code("storage_write_failed")
}
func TestCancelInterruptsActiveCloudWrite(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	mem := &memoryStore{data: map[string][]byte{}}
	s.storage = func(S3Config, bool) (Storage, error) { return mem, nil }
	s.SetPlatform(context.Background(), S3Config{})
	u, _ := s.ensureUser("fixture", "writer", "Writer")
	p := Principal{UserID: u.ID}
	var space Space
	s.DB.First(&space, "owner = ?", u.ID)
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: space.ID, Path: "cancel.txt", Size: 1, SHA256: Hash("x")}, "active-cancel")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PutPart(p, op.ID, 0, bytes.NewReader([]byte("x"))); e != nil {
		t.Fatal(e)
	}
	started := make(chan struct{})
	s.storage = func(S3Config, bool) (Storage, error) { return cancellingStore{mem, started}, nil }
	ctx, cleanup := context.WithCancel(context.Background())
	defer cleanup()
	finished := make(chan struct{})
	go func() { defer close(finished); s.Complete(ctx, p, op.ID) }()
	<-started
	cancelled := make(chan Operation, 1)
	go func() { result, _ := s.Cancel(p, op.ID); cancelled <- result }()
	select {
	case result := <-cancelled:
		if result.Status != "cancelled" {
			t.Fatal("active cancellation was not durable")
		}
	case <-time.After(3 * time.Second):
		cleanup()
		<-finished
		t.Fatal("cancel waited for cloud transfer instead of interrupting it")
	}
	<-finished
}
