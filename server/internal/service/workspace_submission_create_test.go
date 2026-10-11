package service

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

type workspaceRevokingReader struct {
	io.Reader
	revoke func()
}

func (r *workspaceRevokingReader) Read(p []byte) (int, error) {
	if r.revoke != nil {
		f := r.revoke
		r.revoke = nil
		f()
	}
	return r.Reader.Read(p)
}

func TestWorkspaceSubmissionAtomicAllocationAndReplay(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Agent{}, &Space{}, &Connection{}, &Operation{}, &Part{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "conn", Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture"}
	g := WorkspaceGrant{ID: "grant", Owner: a.Owner, TenantID: "tenant", AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, RestrictionProfile: a.RestrictionProfile, Actions: "inbox.submit", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &Space{ID: a.SpaceID, Owner: a.Owner, TenantID: g.TenantID, ConnectionID: a.ConnectionID, MaxBytes: 100}, &Connection{ID: a.ConnectionID, Owner: a.Owner}, &a, &g} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	s.Config.MaxBytes = 100
	s.Config.State = t.TempDir()
	p := Principal{UserID: a.Owner, TenantID: g.TenantID, Agent: &a}
	input := contract.CreateSubmissionRequest{Name: "result.txt", MIMEType: "text/plain", SizeBytes: 10, SHA256: strings.Repeat("a", 64)}
	var first WorkspaceSubmission
	var operation Operation
	create := func(key string, in contract.CreateSubmissionRequest) error {
		return db.Transaction(func(tx *gorm.DB) error {
			sub, op, created, err := s.createWorkspaceSubmissionOn(tx, p, key, in, now)
			if err != nil {
				return err
			}
			if first.ID == "" {
				first, operation = sub, op
				if !created {
					t.Fatal("first allocation was replay")
				}
			} else if key == "key" && (created || sub.ID != first.ID || op.ID != operation.ID) {
				t.Fatal("replay changed identity")
			}
			return nil
		})
	}
	if err := create("key", input); err != nil {
		t.Fatal(err)
	}
	if operation.Purpose != "inbox_staging" || operation.WorkspaceGrantID != g.ID || operation.FileID != "" || operation.Status != "awaiting_upload" || !strings.HasPrefix(operation.Path, "refs/.inbox-staging/grant/") {
		t.Fatal("unsafe allocation")
	}
	if err := create("key", input); err != nil {
		t.Fatal(err)
	}
	changed := input
	changed.Name = "different.txt"
	if err := create("key", changed); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed replay accepted")
	}
	abort := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, _, _, err := s.createWorkspaceSubmissionOn(tx, p, "rollback", input, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	tooLarge := input
	tooLarge.SizeBytes = 91
	if err := create("over-quota", tooLarge); err == nil || err.Error() != "quota_exceeded" {
		t.Fatalf("quota accepted: %v", err)
	}
	var sp Space
	if err := db.First(&sp, "id = ?", a.SpaceID).Error; err != nil {
		t.Fatal(err)
	}
	if sp.Reserved != 10 || sp.Used != 0 {
		t.Fatal("replay or rollback changed quota")
	}
	for _, model := range []any{&Operation{}, &WorkspaceSubmission{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatal("rollback retained allocation")
		}
	}
	if _, err := s.PutPart(p, operation.ID, 0, strings.NewReader("0123456789")); err == nil {
		t.Fatal("legacy part admitted staging")
	}
	reader := &workspaceRevokingReader{Reader: strings.NewReader("0123456789"), revoke: func() {
		if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := s.putPartAuthorized(p, operation.ID, 0, reader, false, true); err == nil {
		t.Fatal("revocation during byte transfer accepted")
	}
	var parts int64
	if err := db.Model(&Part{}).Where("operation_id = ?", operation.ID).Count(&parts).Error; err != nil || parts != 0 {
		t.Fatal("revoked transfer committed part")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", false).Error; err != nil {
		t.Fatal(err)
	}
	part, err := s.putPartAuthorized(p, operation.ID, 0, strings.NewReader("0123456789"), false, true)
	if err != nil || part.Transferred != 10 {
		t.Fatalf("dedicated part failed: %v", err)
	}
	if _, err := s.putPartAuthorized(p, operation.ID, 0, strings.NewReader("0123456789"), false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.putPartAuthorized(p, operation.ID, 0, strings.NewReader("different!"), false, true); err == nil || err.Error() != "part_conflict" {
		t.Fatal("conflicting part accepted")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := create("key", input); err == nil {
		t.Fatal("revoked replay accepted")
	}
	if _, err := s.putPartAuthorized(p, operation.ID, 0, strings.NewReader("0123456789"), false, true); err == nil {
		t.Fatal("revoked part replay accepted")
	}
}
