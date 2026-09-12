package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"

	// The memory backend is linked in for server-side copy tests only.
	_ "github.com/rclone/rclone/backend/memory"
)

// Copy duplicates an existing stored object: same bytes, new remote identity.
func (f *fakeProvider) Copy(ctx context.Context, p Space, src File, target string) (File, error) {
	if ctx.Err() != nil {
		return File{}, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.copyErr != nil {
		return File{}, f.copyErr
	}
	data, ok := f.data[src.ItemID]
	if !ok {
		return File{}, fmt.Errorf("remote_not_found")
	}
	id := newID()
	name := target
	if i := strings.LastIndex(target, "/"); i >= 0 {
		name = target[i+1:]
	}
	copied := File{ItemID: id, DriveID: p.DriveID, Version: "version1", Name: name, Size: int64(len(data)), MIME: src.MIME}
	f.files[id] = copied
	f.data[id] = data
	f.copies++
	return copied, nil
}

func TestContentAddressedReuseUpload(t *testing.T) {
	s, p, local, _ := setup(t)
	original := uploadFixture(t, s, local)
	if p.copies != 0 {
		t.Fatal("unexpected copy during plain upload")
	}
	// Same content, new local name and new key: the object is duplicated
	// server-side, zero bytes leave the process and the receipt keeps the
	// same operation envelope.
	same, err := os.ReadFile(filepath.Join(local, "reference.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(local, "duplicate.png"), same, 0600); err != nil {
		t.Fatal(err)
	}
	op, err := s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "duplicate.png"), "duplicate.png", "reuse-key")
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "completed" || op.Transferred != 0 || op.Bytes != int64(len(same)) {
		t.Fatalf("reuse receipt: status=%s transferred=%d bytes=%d", op.Status, op.Transferred, op.Bytes)
	}
	if op.FileRef == "" || op.FileRef == original.FileRef {
		t.Fatal("reuse did not register a distinct reference")
	}
	if p.uploads != 1 || p.copies != 1 {
		t.Fatalf("byte transfers=%d copies=%d; want 1 transfer and 1 copy", p.uploads, p.copies)
	}
	if _, err = s.Stat(context.Background(), op.FileRef); err != nil {
		t.Fatal(err)
	}
	// A retried identical request reuses the operation, not a new copy.
	replay, err := s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "duplicate.png"), "duplicate.png", "reuse-key")
	if err != nil || replay.ID != op.ID {
		t.Fatal("reuse replay lost idempotency")
	}
	if p.copies != 1 {
		t.Fatal("replay duplicated the object")
	}
	// The same key with different content is a conflict, as with plain upload.
	if err = os.WriteFile(filepath.Join(local, "duplicate.png"), []byte("different"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "duplicate.png"), "duplicate.png", "reuse-key"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed content reused the key")
	}
	// Unique content with no same-space digest falls back to an ordinary
	// byte transfer instead of fabricating a reference.
	if err = os.WriteFile(filepath.Join(local, "novel.png"), []byte("novel bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	fallback, err := s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "novel.png"), "novel.png", "novel-key")
	if err != nil {
		t.Fatal(err)
	}
	fallback = waitOp(t, s, fallback.ID)
	if fallback.Status != "completed" || p.uploads != 2 || p.copies != 1 {
		t.Fatalf("fallback transfer: status=%s transfers=%d copies=%d", fallback.Status, p.uploads, p.copies)
	}
	// A backend without a server-side copy capability degrades to the byte
	// path instead of failing the upload.
	if err = os.WriteFile(filepath.Join(local, "again.png"), same, 0600); err != nil {
		t.Fatal(err)
	}
	p.copyErr = fs.ErrorCantCopy
	degraded, err := s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "again.png"), "again.png", "again-key")
	if err != nil {
		t.Fatal(err)
	}
	degraded = waitOp(t, s, degraded.ID)
	if degraded.Status != "completed" || p.uploads != 3 {
		t.Fatalf("cant-copy fallback: status=%s transfers=%d", degraded.Status, p.uploads)
	}
	// An indexed object that vanished remotely falls back to re-transfer.
	p.copyErr = nil
	for id := range p.data {
		delete(p.data, id)
		delete(p.files, id)
	}
	vanished, err := s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "again.png"), "again2.png", "vanished-key")
	if err != nil {
		t.Fatal(err)
	}
	vanished = waitOp(t, s, vanished.ID)
	if vanished.Status != "completed" {
		t.Fatalf("vanished source did not re-transfer: %s %s", vanished.Status, vanished.Error)
	}
	// Other copy failures surface before any receipt is created.
	p.copyErr = errors.New("graph_unreachable")
	if _, err = s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "again.png"), "again3.png", "hard-key"); err == nil || err.Error() != "graph_unreachable" {
		t.Fatalf("hard copy error swallowed: %v", err)
	}
	var ops []Operation
	s.DB.Where("key = ?", operationKey("creative", "upload", "hard-key")).Find(&ops)
	if len(ops) != 0 {
		t.Fatal("failed reuse left an operation receipt")
	}
}

func TestLinkRefRegistersWithoutCopyingBytes(t *testing.T) {
	s, p, local, _ := setup(t)
	if err := s.AddSpace(Space{ID: "archive", Remote: "onedrive", DriveID: "drive", RootID: "root", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	original := uploadFixture(t, s, local)
	linked, err := s.LinkRef(context.Background(), original.FileRef, "archive", "inbox", "shared.png")
	if err != nil {
		t.Fatal(err)
	}
	if linked.SpaceID != "archive" || linked.Name != "shared.png" || linked.Dir != "inbox" || linked.Ref == original.FileRef {
		t.Fatalf("link registration: %+v", linked)
	}
	if p.uploads != 1 || p.copies != 0 {
		t.Fatalf("link moved bytes: transfers=%d copies=%d", p.uploads, p.copies)
	}
	if _, err = s.Stat(context.Background(), linked.Ref); err != nil {
		t.Fatal(err)
	}
	// Default name keeps the source's remote name.
	source, err := s.Stat(context.Background(), original.FileRef)
	if err != nil {
		t.Fatal(err)
	}
	defaultNamed, err := s.LinkRef(context.Background(), original.FileRef, "archive", "", "")
	if err != nil || defaultNamed.Name != source.Name || defaultNamed.Ref != linked.Ref {
		t.Fatalf("default-name link: %+v err=%v", defaultNamed, err)
	}
	// A target space on another drive cannot reach the object: refuse.
	if err = s.AddSpace(Space{ID: "foreign", Remote: "onedrive", DriveID: "other-drive", RootID: "root", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.LinkRef(context.Background(), original.FileRef, "foreign", "", "x.png"); err == nil || err.Error() != "remote_outside_space" {
		t.Fatalf("cross-drive link accepted: %v", err)
	}
	// A trashed source is not linkable.
	if _, err = s.Trash(context.Background(), original.FileRef); err != nil {
		t.Fatal(err)
	}
	if _, err = s.LinkRef(context.Background(), original.FileRef, "archive", "", "y.png"); err == nil || err.Error() != "file_trashed" {
		t.Fatalf("trashed source linked: %v", err)
	}
}

func TestOneDriveCopyPollsMonitor(t *testing.T) {
	p := Space{ID: "space", DriveID: "drive", RootID: "allowed"}
	provider := NewOneDrive()
	provider.authToken = func(context.Context, Space) (string, error) { return "fixture", nil }
	var polls int
	provider.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer fixture" || r.URL.Host != "graph.microsoft.com" {
			t.Fatal("wrong Graph authentication target")
		}
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/items/source/copy"):
			resp := &http.Response{StatusCode: 202, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}
			resp.Header.Set("Location", "https://graph.microsoft.com/v1.0/monitor/copy-1")
			return resp, nil
		case r.Method == "GET" && r.URL.Path == "/v1.0/monitor/copy-1":
			polls++
			if polls < 2 {
				return &http.Response{StatusCode: 202, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"percentageComplete":10}`)), Request: r}, nil
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"newitem","name":"dup.png","size":4,"eTag":"v2","parentReference":{"id":"allowed","driveId":"drive"},"file":{"mimeType":"image/png"}}`)), Request: r}, nil
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/items/newitem"):
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"newitem","name":"dup.png","size":4,"eTag":"v2","parentReference":{"id":"allowed","driveId":"drive"},"file":{"mimeType":"image/png"}}`)), Request: r}, nil
		}
		return &http.Response{StatusCode: 404, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})
	copied, err := provider.Copy(context.Background(), p, File{ItemID: "source", Version: "v1", Size: 4}, "op1-dup.png")
	if err != nil {
		t.Fatal(err)
	}
	if copied.ItemID != "newitem" || copied.Size != 4 || polls != 2 {
		t.Fatalf("copy result: %+v polls=%d", copied, polls)
	}
	// A monitor URL on any other host is refused, never followed.
	provider.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/copy") {
			resp := &http.Response{StatusCode: 202, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}
			resp.Header.Set("Location", "https://evil.example.com/monitor")
			return resp, nil
		}
		t.Fatal("monitor should not have been followed")
		return nil, nil
	})
	if _, err = provider.Copy(context.Background(), p, File{ItemID: "source", Version: "v1", Size: 4}, "op2-dup.png"); err == nil || err.Error() != "graph_unavailable" {
		t.Fatalf("foreign monitor accepted: %v", err)
	}
}

func TestGenericCopyUsesServerSideCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("generic backend test exercises real rclone backends")
	}
	// The memory backend advertises a real server-side copy.
	const remote = "generic-copy-fixture"
	config.Data().SetValue(remote, "type", "memory")
	defer config.Data().DeleteSection(remote)
	g := NewGeneric()
	p := Space{ID: "gen", Remote: remote, Backend: "generic", DriveID: "rclone:" + remote, RootID: "", LocalRoot: t.TempDir(), MaxBytes: 1 << 20}
	seeded, err := g.Upload(context.Background(), p, "a.bin", strings.NewReader("payload"), 7)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := g.Copy(context.Background(), p, File{ItemID: seeded.ItemID, Name: seeded.Name, Size: 7, Version: seeded.Version}, "reuse-b.bin")
	if err != nil {
		t.Fatal(err)
	}
	if copied.ItemID != "reuse-b.bin" || copied.Size != 7 {
		t.Fatalf("generic copy: %+v", copied)
	}
	body, err := g.Open(context.Background(), p, File{ItemID: "reuse-b.bin"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(body)
	body.Close()
	if string(data) != "payload" {
		t.Fatalf("copied bytes: %q", data)
	}
	// Unknown sources surface the fixed not-found code.
	if _, err = g.Copy(context.Background(), p, File{ItemID: "missing.bin", Name: "missing.bin"}, "x.bin"); err == nil || err.Error() != "remote_not_found" {
		t.Fatalf("missing source: %v", err)
	}
	// A backend without the copy capability reports the rclone sentinel so
	// callers can fall back to a byte transfer.
	const alias = "generic-copy-alias"
	dir := t.TempDir()
	config.Data().SetValue(alias, "type", "alias")
	config.Data().SetValue(alias, "remote", dir)
	defer config.Data().DeleteSection(alias)
	if err = os.WriteFile(filepath.Join(dir, "a.bin"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	pa := Space{ID: "genalias", Remote: alias, Backend: "generic", DriveID: "rclone:" + alias, RootID: "", LocalRoot: dir, MaxBytes: 1 << 20}
	if _, err = g.Copy(context.Background(), pa, File{ItemID: "a.bin", Name: "a.bin"}, "y.bin"); !errors.Is(err, fs.ErrorCantCopy) {
		t.Fatalf("copy-less backend: %v", err)
	}
}
