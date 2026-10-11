package service

import (
	"context"
	"io"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// Fail a real completion read-back after Put has succeeded, leaving the
// persisted operation in verifying rather than fabricating its state.
type archiveReadbackFailure struct {
	Storage
	fail bool
}

func (s *archiveReadbackFailure) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	if err := s.Storage.Put(ctx, key, r, size); err != nil {
		return err
	}
	s.fail = true
	return nil
}
func (s *archiveReadbackFailure) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.fail {
		return nil, code("storage_read_failed")
	}
	return s.Storage.Get(ctx, key)
}

func TestWorkspaceArchiveResumeAfterReadbackFailure(t *testing.T) {
	s, store, p := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	source := uploadBytes(t, s, p, sp.ID, "source.txt", []byte("archive recovery bytes"))
	broken := &archiveReadbackFailure{Storage: store}
	s.storage = func(S3Config, bool) (Storage, error) { return broken, nil }
	fileID, versionID := source.FileID, source.VersionID
	sub := WorkspaceSubmission{ID: ID(), Owner: p.UserID, SpaceID: sp.ID, AgentID: "agent", GrantID: "grant", UploadOperationID: source.ID, CreationKey: "submission", RequestDigest: "digest", FileID: &fileID, VersionID: &versionID, State: "accepted", Revision: 3}
	if err := s.DB.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	intent := &workspaceArchiveIntent{SubmissionID: sub.ID, RequestKey: "original", RequestDigest: "digest", Request: contract.ArchiveSubmissionRequest{ExpectedRevision: 3, VersionID: versionID, TargetSpaceID: sp.ID, TargetPrefix: "archive"}}
	q, err := s.prepareWorkspaceArchiveOn(s.DB, p, sub.ID, intent.Request)
	if err != nil {
		t.Fatal(err)
	}
	op, err := s.createUpload(p, UploadRequest{SpaceID: sp.ID, Path: q.TargetPath, Size: source.Size, SHA256: source.SHA256}, "recover-original", &ingestOrigin{Kind: "archive", Source: q, Workspace: intent})
	if err != nil {
		t.Fatal(err)
	}
	op, err = s.streamExistingVersion(t.Context(), p, op, fileID, versionID)
	if err == nil || err.Error() != "storage_read_failed" {
		t.Fatalf("expected read-back failure: %v", err)
	}
	var pending Operation
	if err := s.DB.First(&pending, "id = ?", op.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pending.Status != "verifying" {
		t.Fatalf("unexpected persisted status %s", pending.Status)
	}
	var inProgress WorkspaceSubmission
	if err := s.DB.First(&inProgress, "id = ?", sub.ID).Error; err != nil || inProgress.State != "archiving" || inProgress.ArchivedFileRef != nil {
		t.Fatal("read-back failure falsely archived submission")
	}
	recovered, found, err := s.workspaceArchiveReplayOn(s.DB, p, sub.ID, intent.RequestKey, intent.RequestDigest)
	if err != nil || !found || recovered.ID != op.ID || recovered.Status != "verifying" {
		t.Fatal("original request did not recover interrupted operation")
	}
	s.storage = func(S3Config, bool) (Storage, error) { return store, nil }
	if err := s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resume(t.Context(), p, op.ID); err == nil {
		t.Fatal("disabled owner published archive")
	}
	var unpublished int64
	if err := s.DB.Model(&File{}).Where("space_id = ? AND path = ?", sp.ID, q.TargetPath).Count(&unpublished).Error; err != nil || unpublished != 0 {
		t.Fatal("disabled owner left published target")
	}
	if err := s.DB.First(&inProgress, "id = ?", sub.ID).Error; err != nil || inProgress.State != "archiving" || inProgress.ArchivedFileRef != nil {
		t.Fatal("disabled owner changed archive result")
	}
	if err := s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	resumed, err := s.Resume(t.Context(), p, op.ID)
	if err != nil || resumed.Status != "completed" || resumed.ID != op.ID {
		t.Fatalf("resume failed: %v", err)
	}
	replay, err := s.Resume(t.Context(), p, op.ID)
	if err != nil || replay.VersionID != resumed.VersionID {
		t.Fatal("resume replay changed version")
	}
	var archived WorkspaceSubmission
	if err := s.DB.First(&archived, "id = ?", sub.ID).Error; err != nil || archived.State != "archived" || archived.Revision != 5 || archived.ArchivedFileRef == nil || *archived.ArchivedFileRef != resumed.FileID {
		t.Fatal("recovery failed to publish submission result")
	}
	var count int64
	if err := s.DB.Model(&Operation{}).Where("owner = ? AND key = ?", p.UserID, "recover-original").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("recovery duplicated operation")
	}
	if err := s.DB.Model(&Version{}).Where("file_id = ?", resumed.FileID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("recovery duplicated target version")
	}
}
