package service

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceArchiveAllocationBindsSubmissionAtomically(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	source := uploadBytes(t, s, p, sp.ID, "source.txt", []byte("source bytes"))
	fileID, versionID := source.FileID, source.VersionID
	sub := WorkspaceSubmission{ID: ID(), Owner: p.UserID, SpaceID: sp.ID, AgentID: "agent", GrantID: "grant", UploadOperationID: source.ID, CreationKey: "submission", RequestDigest: "digest", FileID: &fileID, VersionID: &versionID, State: "accepted", Revision: 3}
	if err := s.DB.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	intent := &workspaceArchiveIntent{SubmissionID: sub.ID, RequestKey: "original-key", RequestDigest: "original-digest", Request: contract.ArchiveSubmissionRequest{ExpectedRevision: 3, VersionID: versionID, TargetSpaceID: sp.ID, TargetPrefix: "archive"}}
	q, err := s.prepareWorkspaceArchiveOn(s.DB, p, sub.ID, intent.Request)
	if err != nil {
		t.Fatal(err)
	}
	origin := &ingestOrigin{Kind: "archive", Source: q, Workspace: intent}
	upload := UploadRequest{SpaceID: sp.ID, Path: q.TargetPath, Size: source.Size, SHA256: source.SHA256}
	const callback = "archive_binding_audit_failure"
	if err := s.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*Audit); ok && a.Action == "workspace.submission.archive" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failed := s.createUpload(p, upload, "archive-request", origin)
	if err := s.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatalf("expected failed audit: %v", failed)
	}
	var current WorkspaceSubmission
	if err := s.DB.First(&current, "id = ?", sub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.State != "accepted" || current.Revision != 3 || current.ArchiveOperationID != nil {
		t.Fatal("rollback retained binding")
	}
	var count int64
	if err := s.DB.Model(&Operation{}).Where("owner = ? AND key = ?", p.UserID, "archive-request").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("rollback retained operation")
	}
	if err := s.DB.Model(&WorkspaceRequest{}).Where("action = ?", "submission.archive").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed audit retained request receipt")
	}
	var quota Space
	if err := s.DB.First(&quota, "id = ?", sp.ID).Error; err != nil || quota.Reserved != 0 {
		t.Fatal("rollback retained quota")
	}
	op, err := s.createUpload(p, upload, "archive-request", origin)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DB.First(&current, "id = ?", sub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.State != "archiving" || current.Revision != 4 || current.ArchiveOperationID == nil || *current.ArchiveOperationID != op.ID {
		t.Fatal("operation not atomically bound")
	}
	replay, err := s.createUpload(p, upload, "archive-request", origin)
	if err != nil || replay.ID != op.ID {
		t.Fatal("replay changed operation")
	}
	if _, err := s.createUpload(p, upload, "another-key", origin); err == nil {
		t.Fatal("second key allocated another archive")
	}
	if err := s.DB.First(&quota, "id = ?", sp.ID).Error; err != nil || quota.Reserved != source.Size {
		t.Fatal("allocation reserved quota twice")
	}
	recovered, found, err := s.workspaceArchiveReplayOn(s.DB, p, sub.ID, intent.RequestKey, intent.RequestDigest)
	if err != nil || !found || recovered.ID != op.ID {
		t.Fatal("request recovery failed")
	}
	if _, _, err := s.workspaceArchiveReplayOn(s.DB, p, sub.ID, intent.RequestKey, "changed"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed request recovered operation")
	}
	if err := s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.workspaceArchiveReplayOn(s.DB, p, sub.ID, intent.RequestKey, intent.RequestDigest); err == nil {
		t.Fatal("disabled human recovered archive")
	}
	if err := s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	const finishCallback = "archive_finish_audit_failure"
	if err := s.DB.Callback().Create().Before("gorm:create").Register(finishCallback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*Audit); ok && a.Action == "workspace.submission.archived" {
			tx.AddError(errors.New("fixture finish failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failed = s.streamExistingVersion(t.Context(), p, op, fileID, versionID)
	if err := s.DB.Callback().Create().Remove(finishCallback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatalf("finish audit failure: %v", failed)
	}
	current = WorkspaceSubmission{}
	if err := s.DB.First(&current, "id = ?", sub.ID).Error; err != nil || current.State != "archiving" || current.ArchivedFileRef != nil {
		t.Fatal("failed publication marked archived")
	}
	if err := s.DB.Model(&File{}).Where("space_id = ? AND path = ?", sp.ID, q.TargetPath).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed publication retained target")
	}
	completed, err := s.Complete(t.Context(), p, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DB.First(&current, "id = ?", sub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.State != "archived" || current.Revision != 5 || current.ArchivedFileRef == nil || *current.ArchivedFileRef != completed.FileID || *current.VersionID != versionID {
		t.Fatal("archive result not atomically attached")
	}
	if _, err := s.Complete(t.Context(), p, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Model(&Audit{}).Where("resource = ? AND action = ?", sub.ID, "workspace.submission.archived").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("archive replay duplicated audit")
	}
}
