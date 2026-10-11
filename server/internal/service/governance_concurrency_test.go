package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type governanceBlockingStore struct {
	*memoryStore
	key     string
	started chan struct{}
	release chan struct{}
}

func (g governanceBlockingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == g.key {
		close(g.started)
		select {
		case <-g.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return g.memoryStore.Get(ctx, key)
}
func TestGovernanceRevocationDoesNotWaitForExecutionIO(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	mem := &memoryStore{data: map[string][]byte{}}
	s.storage = func(S3Config, bool) (Storage, error) { return mem, nil }
	if e = s.SetPlatform(context.Background(), S3Config{}); e != nil {
		t.Fatal(e)
	}
	root, _ := s.ensureUser("fixture", "root", "Root")
	owner, _ := s.ensureUser("fixture", "owner", "Owner")
	if e = s.BootstrapGovernance(root.ID); e != nil {
		t.Fatal(e)
	}
	var sp Space
	s.DB.First(&sp, "owner = ?", owner.ID)
	p := Principal{UserID: owner.ID, TenantID: s.Instance}
	seed := func(path, content string) Operation {
		op, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: path, Size: int64(len(content)), SHA256: Hash(content)}, ID())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.PutPart(p, op.ID, 0, bytes.NewBufferString(content)); e != nil {
			t.Fatal(e)
		}
		op, e = s.Complete(context.Background(), p, op.ID)
		if e != nil {
			t.Fatal(e)
		}
		return op
	}
	target := seed("target.txt", "old")
	source := seed("source.txt", "new")
	a := AccessRequest{ID: ID(), TenantID: s.Instance, SpaceID: sp.ID, Requester: root.ID, Approver: owner.ID, Kind: "exact", Status: "approved", GrantUntil: time.Now().Add(time.Hour), Items: []RequestedAction{{FileID: target.FileID, VersionID: target.VersionID, SourceFileID: source.FileID, SourceVersionID: source.VersionID, Action: "overwrite", Status: "pending"}}}
	if e = s.DB.Create(&a).Error; e != nil {
		t.Fatal(e)
	}
	var version Version
	s.DB.First(&version, "id = ?", source.VersionID)
	started, release := make(chan struct{}), make(chan struct{})
	s.storage = func(S3Config, bool) (Storage, error) {
		return governanceBlockingStore{mem, version.ObjectKey, started, release}, nil
	}
	call := func(actor Principal, action string) error {
		body, _ := json.Marshal(governanceInput{ID: a.ID})
		r := httptest.NewRequest("POST", "/api/v1/governance/"+action, bytes.NewReader(body))
		r.SetPathValue("action", action)
		return s.governanceChange(httptest.NewRecorder(), r, actor)
	}
	done := make(chan error, 1)
	go func() { done <- call(Principal{UserID: root.ID, TenantID: s.Instance}, "execute") }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("execution did not start")
	}
	revoked := make(chan error, 1)
	go func() { revoked <- call(p, "revoke") }()
	select {
	case err := <-revoked:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		<-done
		t.Fatal("revocation blocked behind I/O")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("execution did not stop")
	}
	s.DB.First(&a, "id = ?", a.ID)
	if a.Status != "revoked" {
		t.Fatalf("revocation overwritten: %s", a.Status)
	}
	var file File
	s.DB.First(&file, "id = ?", target.FileID)
	if file.VersionID != target.VersionID {
		t.Fatal("revoked overwrite committed")
	}
}
