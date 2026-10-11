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

type teamBlockedPut struct {
	*memoryStore
	started, release chan struct{}
}

func (s teamBlockedPut) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	close(s.started)
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.memoryStore.Put(ctx, key, r, size)
}
func TestTeamArchiveDoesNotWaitForUploadAndPreventsCommit(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
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
	team := Team{ID: ID(), TenantID: s.Instance, Name: "Concurrent", Revision: 1}
	sp := Space{ID: ID(), TenantID: s.Instance, TeamID: team.ID, Owner: ID(), Name: "Assets", ConnectionID: "platform", MaxBytes: 1000}
	for _, v := range []any{&team, &sp, &TeamMember{TeamID: team.ID, UserID: owner.ID, Role: "admin"}, &SpacePermission{SpaceID: sp.ID, UserID: owner.ID, Role: "custodian"}, &TeamStorage{TeamID: team.ID, TenantID: s.Instance, ConnectionID: "platform"}} {
		if e = s.DB.Create(v).Error; e != nil {
			t.Fatal(e)
		}
	}
	p := Principal{UserID: owner.ID, TenantID: s.Instance, DataOwner: sp.Owner, ScopeSpace: sp.ID}
	body := []byte("in flight")
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: "flight.txt", Size: int64(len(body)), SHA256: Hash(string(body))}, ID())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
		t.Fatal(e)
	}
	started, release := make(chan struct{}), make(chan struct{})
	s.storage = func(S3Config, bool) (Storage, error) { return teamBlockedPut{mem, started, release}, nil }
	complete := make(chan error, 1)
	go func() { _, err := s.Complete(context.Background(), p, op.ID); complete <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("upload did not begin")
	}
	rev := uint64(1)
	data, _ := json.Marshal(governanceInput{TeamID: team.ID, ExpectedRevision: &rev})
	req := httptest.NewRequest("POST", "/api/v1/governance/team-archive", bytes.NewReader(data))
	req.SetPathValue("action", "team-archive")
	archived := make(chan error, 1)
	go func() {
		archived <- s.governanceChange(httptest.NewRecorder(), req, Principal{UserID: root.ID, TenantID: s.Instance})
	}()
	select {
	case err := <-archived:
		if err != nil {
			close(release)
			<-complete
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		<-complete
		t.Fatal("archive waited for external I/O")
	}
	close(release)
	select {
	case err := <-complete:
		if safeCode(err) != "team_archived" {
			t.Fatal("archived upload published", safeCode(err))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not finish")
	}
	var n int64
	s.DB.Model(&File{}).Where("space_id = ?", sp.ID).Count(&n)
	if n != 0 {
		t.Fatal("file metadata published after archive")
	}
	if _, e = s.Cancel(p, op.ID); e != nil {
		t.Fatal(e)
	}
	s.DB.First(&sp, "id = ?", sp.ID)
	if sp.Reserved != 0 {
		t.Fatal("cancel retained reserved quota")
	}
}
