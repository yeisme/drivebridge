package service

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceSubmissionPublicationJoinsTransaction(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}, &Operation{}, &File{}, &Version{}, &Audit{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "connection", Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture"}
	g := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "inbox.submit", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	op := Operation{ID: "op", Owner: a.Owner, ActorID: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Path: "refs/staged", Kind: "upload", Purpose: "inbox_staging", WorkspaceGrantID: g.ID, Status: "verifying", VersionID: "version", Size: 10, SHA256: "fixture"}
	sub := WorkspaceSubmission{ID: "sub", Owner: a.Owner, TenantID: g.TenantID, GrantID: g.ID, AgentID: a.ID, SpaceID: a.SpaceID, UploadOperationID: op.ID, CreationKey: "key", RequestDigest: "digest", State: "uploading", Revision: 1}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &Space{ID: a.SpaceID, Owner: a.Owner, TenantID: g.TenantID, ConnectionID: a.ConnectionID}, &a, &g, &op, &sub} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: a.Owner, TenantID: g.TenantID, Agent: &a}
	if err := s.finishWorkspaceSubmissionOn(db, p, op.ID, now); err == nil {
		t.Fatal("unfinished operation accepted")
	}
	publish := func(tx *gorm.DB) error {
		f := File{ID: "file", Owner: a.Owner, SpaceID: a.SpaceID, Path: op.Path, VersionID: op.VersionID, ReviewPending: true}
		v := Version{ID: op.VersionID, Owner: a.Owner, FileID: f.ID, ConnectionID: a.ConnectionID, SHA256: op.SHA256, Size: op.Size}
		for _, row := range []any{&f, &v} {
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "completed", "file_id": f.ID}).Error; err != nil {
			return err
		}
		return s.finishWorkspaceSubmissionOn(tx, p, op.ID, now)
	}
	callback := "fixture_submission_publication_audit_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*Audit); ok && audit.Action == "workspace.submission.submitted" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed := db.Transaction(publish)
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatalf("audit failure not returned: %v", failed)
	}
	for _, model := range []any{&File{}, &Version{}, &Audit{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("publication rollback retained rows")
		}
	}
	var unchanged WorkspaceSubmission
	if err := db.First(&unchanged, "id = ?", sub.ID).Error; err != nil || unchanged.State != "uploading" || unchanged.Revision != 1 {
		t.Fatal("rollback changed submission")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(publish); err == nil {
		t.Fatal("revoked grant published fixed references")
	}
	for _, model := range []any{&File{}, &Version{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("revoked publication retained metadata")
		}
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(publish); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return s.finishWorkspaceSubmissionOn(tx, p, op.ID, now) }); err != nil {
		t.Fatal(err)
	}
	var result WorkspaceSubmission
	if err := db.First(&result, "id = ?", sub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if result.State != "submitted" || result.Revision != 2 || result.FileID == nil || *result.FileID != "file" || result.VersionID == nil || *result.VersionID != op.VersionID {
		t.Fatal("fixed references not persisted")
	}
	var count int64
	if err := db.Model(&Audit{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("replay duplicated audit")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.finishWorkspaceSubmissionOn(db, p, op.ID, now); err == nil {
		t.Fatal("revoked publication replay accepted")
	}
}
