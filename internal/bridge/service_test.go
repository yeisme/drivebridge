package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	mu           sync.Mutex
	files        map[string]File
	data         map[string][]byte
	uploads      int
	opens        int
	copies       int
	copyErr      error
	failAfterPut bool
	accesses     int
}

func fake() *fakeProvider { return &fakeProvider{files: map[string]File{}, data: map[string][]byte{}} }
func (f *fakeProvider) List(ctx context.Context, p Space, dir string) ([]File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []File{}
	for _, i := range f.files {
		out = append(out, i)
	}
	return out, nil
}
func (f *fakeProvider) Mkdir(context.Context, Space, string) error              { return nil }
func (f *fakeProvider) Move(context.Context, Space, File, string, string) error { return nil }
func (f *fakeProvider) Delete(context.Context, Space, File) error               { return nil }
func (f *fakeProvider) Stat(ctx context.Context, p Space, id string) (File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.files[id]
	if !ok {
		return File{}, errors.New("remote_not_found")
	}
	if i.DriveID != p.DriveID {
		return File{}, errors.New("remote_outside_space")
	}
	return i, nil
}
func (f *fakeProvider) Upload(ctx context.Context, p Space, name string, r io.Reader, n int64) (File, error) {
	b, e := io.ReadAll(r)
	if e != nil {
		return File{}, e
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	id := newID()
	i := File{ItemID: id, DriveID: p.DriveID, Version: "version1", Name: name, Size: int64(len(b)), MIME: "image/png"}
	f.files[id] = i
	f.data[id] = b
	f.uploads++
	if f.failAfterPut {
		f.failAfterPut = false
		return File{}, errors.New("connection_lost")
	}
	return i, nil
}
func (f *fakeProvider) Open(ctx context.Context, p Space, i File) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	return io.NopCloser(bytes.NewReader(f.data[i.ItemID])), nil
}
func (f *fakeProvider) Access(ctx context.Context, p Space, i File) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accesses++
	return "https://fixture.files.1drv.com/content?credential=ephemeral-test-value", nil
}
func setup(t *testing.T) (*Service, *fakeProvider, string, string) {
	t.Helper()
	state := t.TempDir()
	if e := os.Chmod(state, 0700); e != nil {
		t.Fatal(e)
	}
	local := t.TempDir()
	p := fake()
	s, e := New(state, p, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.AddSpace(Space{ID: "creative", Remote: "onedrive", DriveID: "drive", RootID: "root", LocalRoot: local, MaxBytes: 1 << 20, Project: "project", Actor: "actor"}); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s, p, local, state
}
func fixture(t *testing.T, local string) string {
	t.Helper()
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	path := filepath.Join(local, "reference.png")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func waitOp(t *testing.T, s *Service, id string) Operation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		op, e := s.Get(id)
		if e != nil {
			t.Fatal(e)
		}
		if op.Status == "completed" || op.Status == "failed" || op.Status == "cancelled" || op.Status == "interrupted" {
			return op
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operation did not settle")
	return Operation{}
}
func uploadFixture(t *testing.T, s *Service, local string) Operation {
	t.Helper()
	op, e := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "input")
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" {
		t.Fatalf("upload: %s %s", op.Status, op.Error)
	}
	return op
}
func TestUploadResumeAfterLostAcknowledgement(t *testing.T) {
	s, p, local, _ := setup(t)
	p.failAfterPut = true
	op, e := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "key")
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" {
		t.Fatal(op.Status)
	}
	time.Sleep(10 * time.Millisecond)
	if _, e = s.Resume(op.ID); e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" {
		t.Fatal(op.Error)
	}
	if p.uploads != 1 {
		t.Fatal("duplicate remote file")
	}
	replay, e := s.Upload(context.Background(), "creative", "", filepath.Join(local, "reference.png"), "reference.png", "key")
	if e != nil || replay.ID != op.ID {
		t.Fatal("lost idempotency")
	}
	if _, e = s.Upload(context.Background(), "creative", "", filepath.Join(local, "reference.png"), "changed.png", "key"); e == nil {
		t.Fatal("changed request reused key")
	}
}
func TestIsolationVersionAndNonOverwrite(t *testing.T) {
	s, p, local, _ := setup(t)
	op := uploadFixture(t, s, local)
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("outside"), 0600)
	os.Symlink(outside, filepath.Join(local, "escape"))
	for _, path := range []string{outside, "../secret", "escape"} {
		if _, e := s.Upload(context.Background(), "creative", "", path, "x", "escape-"+path); e == nil {
			t.Fatal("escaped local root")
		}
	}
	if _, e := s.Upload(context.Background(), "another", "", "reference.png", "x", "wrong-space"); e == nil {
		t.Fatal("space bypass")
	}
	os.WriteFile(filepath.Join(local, "existing.png"), []byte("preserve"), 0600)
	download, e := s.Download(context.Background(), op.FileRef, "existing.png", "output")
	if e != nil {
		t.Fatal(e)
	}
	download = waitOp(t, s, download.ID)
	if download.Error != "target_conflict" {
		t.Fatal(download.Error)
	}
	b, _ := os.ReadFile(filepath.Join(local, "existing.png"))
	if string(b) != "preserve" {
		t.Fatal("overwrote user file")
	}
	file, e := s.Stat(context.Background(), op.FileRef)
	if e != nil {
		t.Fatal(e)
	}
	p.mu.Lock()
	i := p.files[file.ItemID]
	i.Version = "replaced"
	p.files[file.ItemID] = i
	p.mu.Unlock()
	if _, e = s.Stat(context.Background(), op.FileRef); e == nil || e.Error() != "file_version_changed" {
		t.Fatal("replacement accepted")
	}
}
func TestPrivateHandoffNeverReturnsURLThroughMCP(t *testing.T) {
	s, p, local, _ := setup(t)
	op := uploadFixture(t, s, local)
	if _, e := s.Call(context.Background(), "prepare_reference", map[string]any{"file_ref": op.FileRef}); e == nil {
		t.Fatal("missing handoff accepted")
	}
	s.Handoff = true
	for _, name := range []string{"capabilities", "prepare_reference", "stat", "operation_get"} {
		v, e := s.Call(context.Background(), name, map[string]any{"file_ref": op.FileRef, "operation_id": op.ID})
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), "https://") || strings.Contains(string(b), "ephemeral-test-value") {
			t.Fatal("sensitive URL in ordinary tool output")
		}
	}
	if p.accesses != 0 {
		t.Fatal("URL fetched too early")
	}
	for _, actor := range []string{"intruder", "actor"} {
		body, _ := json.Marshal(HandoffRequest{Ref: op.FileRef, Project: "project", Actor: actor})
		req := httptest.NewRequest("POST", "/v1/acquire", bytes.NewReader(body))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if actor == "actor" {
			if w.Code != 200 || !strings.Contains(w.Body.String(), "ephemeral-test-value") {
				t.Fatal("private handoff failed")
			}
		} else if w.Code == 200 {
			t.Fatal("wrong identity accepted")
		}
	}
	var ops []Operation
	s.DB.Find(&ops)
	b, _ := json.Marshal(ops)
	if strings.Contains(string(b), "ephemeral-test-value") {
		t.Fatal("URL persisted")
	}
}
func TestFalseMIMEAndURLRestrictions(t *testing.T) {
	s, _, local, _ := setup(t)
	os.WriteFile(filepath.Join(local, "fake.png"), []byte("not media"), 0600)
	op, e := s.Upload(context.Background(), "creative", "", "fake.png", "fake.png", "fake")
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	s.Handoff = true
	if _, _, e = s.reference(context.Background(), HandoffRequest{Ref: op.FileRef, Project: "project", Actor: "actor"}); e == nil {
		t.Fatal("spoofed MIME accepted")
	}
	for _, u := range []string{"http://x.files.1drv.com/f", "https://127.0.0.1/f", "https://evilsharepoint.com/f", "https://x.sharepoint.com.evil.test/f", "https://user:pass@x.sharepoint.com/f", "https://x.sharepoint.com:443/f"} {
		if validDownloadURL(u) {
			t.Fatal("unsafe URL accepted")
		}
	}
}
func TestRestartMarksUnfinishedAndPreservesReceipt(t *testing.T) {
	p := fake()
	state := t.TempDir()
	if e := os.Chmod(state, 0700); e != nil {
		t.Fatal(e)
	}
	local := t.TempDir()
	s, e := New(state, p, true)
	if e != nil {
		t.Fatal(e)
	}
	s.AddSpace(Space{ID: "creative", Remote: "onedrive", DriveID: "drive", RootID: "root", LocalRoot: local, MaxBytes: 1 << 20})
	op := uploadFixture(t, s, local)
	s.DB.Create(&Operation{ID: "interrupted", Key: "interrupted", Status: "transferring", SpaceID: "creative"})
	s.Close()
	s, e = New(state, p, true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	stored, e := s.Get(op.ID)
	if e != nil || stored.FileRef != op.FileRef {
		t.Fatal("receipt lost")
	}
	stored, _ = s.Get("interrupted")
	if stored.Status != "interrupted" {
		t.Fatal("restart recovery missing")
	}
}
func TestScaenaOutputUsesOnlyExistingTaskAndPrivateGrant(t *testing.T) {
	s, p, local, _ := setup(t)
	raw, _ := os.ReadFile(fixture(t, local))
	h, _, _ := digestReader(bytes.NewReader(raw))
	var callsMu sync.Mutex
	calls := []string{}
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callsMu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		callsMu.Unlock()
		if r.Header.Get("Authorization") != "Bearer fixture" {
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/video-tasks/"):
			io.WriteString(w, `{"data":{"status":"succeeded","project_ref":"project","submission_ref":"video-provider-task:one","asset_ref":"asset:one","import_status":"imported"}}`)
		case r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"grant_ref": "grant:one", "size_bytes": len(raw), "digest": "sha256:" + h, "access_kind": "resource_link", "resource_link": "https://untrusted.invalid/ignored"}})
		default:
			w.Write(raw)
		}
	}))
	defer owner.Close()
	client, e := NewScaenaOutput(owner.URL, func() (string, error) { return "fixture", nil })
	if e != nil {
		t.Fatal(e)
	}
	s.Output = client
	p.failAfterPut = true
	op, e := s.SaveOutput("creative", "video-provider-task:one", "save")
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" {
		t.Fatal(op.Status)
	}
	time.Sleep(10 * time.Millisecond)
	s.Resume(op.ID)
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" {
		t.Fatal(op.Error)
	}
	if p.uploads != 1 {
		t.Fatal("duplicate output upload")
	}
	again, e := s.SaveOutput("creative", "video-provider-task:one", "save")
	if e != nil || again.ID != op.ID {
		t.Fatal("output replay conflict")
	}
	callsMu.Lock()
	defer callsMu.Unlock()
	for _, call := range calls {
		if strings.HasPrefix(call, "POST ") && !strings.HasSuffix(call, "/access-grants") {
			t.Fatal("generation was submitted")
		}
	}
}

func (b blockingProvider) List(ctx context.Context, p Space, dir string) ([]File, error) {
	return nil, nil
}
func (b blockingProvider) Mkdir(context.Context, Space, string) error { return nil }

type blockingProvider struct {
	*fakeProvider
	started chan struct{}
}

func (b blockingProvider) Upload(ctx context.Context, p Space, name string, r io.Reader, n int64) (File, error) {
	close(b.started)
	<-ctx.Done()
	return File{}, ctx.Err()
}
func TestCancelIsDurableAndNeverResumes(t *testing.T) {
	s, p, local, _ := setup(t)
	started := make(chan struct{})
	s.Provider = blockingProvider{p, started}
	op, e := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "cancel")
	if e != nil {
		t.Fatal(e)
	}
	<-started
	if _, e = s.Cancel(op.ID); e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "cancelled" {
		t.Fatalf("cancellation became %s", op.Status)
	}
	time.Sleep(10 * time.Millisecond)
	if _, e = s.Resume(op.ID); e == nil {
		t.Fatal("cancelled operation resumed")
	}
}
func TestCancelledBeforeWorkerStarts(t *testing.T) {
	for i := 0; i < 10; i++ {
		s, _, local, _ := setup(t)
		op, e := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "early")
		if e != nil {
			t.Fatal(e)
		}
		s.Cancel(op.ID)
		op = waitOp(t, s, op.ID)
		if op.Status != "cancelled" && op.Status != "completed" {
			t.Fatalf("early cancel became %s", op.Status)
		}
	}
}

func TestRealMP4ReferenceProbe(t *testing.T) {
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Skip("ffmpeg fixture generator unavailable")
	}
	s, _, local, _ := setup(t)
	path := filepath.Join(local, "reference.mp4")
	command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=32x32:r=10", "-t", "1", "-c:v", "mpeg4", path)
	if e := command.Run(); e != nil {
		t.Fatal("MP4 fixture generation failed")
	}
	op, e := s.Upload(context.Background(), "creative", "", path, "reference.mp4", "video")
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	s.Handoff = true
	file, _, e := s.reference(context.Background(), HandoffRequest{Ref: op.FileRef, Project: "project", Actor: "actor"})
	if e != nil {
		t.Fatal(e)
	}
	if file.MIME != "video/mp4" || file.Duration != 1 || !file.Verified {
		t.Fatal("MP4 did not enter verified reference flow")
	}
}

func TestPublicStateDirectoryRejected(t *testing.T) {
	state := t.TempDir()
	if e := os.Chmod(state, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := New(state, fake(), true); e == nil || e.Error() != "state_not_private" {
		t.Fatal("public state accepted")
	}
}
