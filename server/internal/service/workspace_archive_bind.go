package service

import (
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// The intent is server-generated, not part of the legacy archive wire DTO.
type workspaceArchiveIntent struct {
	SubmissionID  string
	RequestKey    string `json:",omitempty"`
	RequestDigest string `json:",omitempty"`
	Request       contract.ArchiveSubmissionRequest
}

func (s *Service) bindWorkspaceArchiveOn(tx *gorm.DB, p Principal, op Operation, origin *ingestOrigin) error {
	if origin == nil || origin.Workspace == nil {
		return nil
	}
	intent := origin.Workspace
	q, err := s.prepareWorkspaceArchiveOn(tx, p, intent.SubmissionID, intent.Request)
	if err != nil {
		return err
	}
	if origin.Kind != "archive" || origin.AutoRename || q != origin.Source || op.Kind != "archive" || op.ArchiveFileID != q.SourceFileID || op.ArchiveVersionID != q.SourceVersionID || op.SpaceID != q.TargetSpaceID || op.Path != q.TargetPath {
		return code("invalid_archive")
	}
	if (intent.RequestKey == "") != (intent.RequestDigest == "") || len(intent.RequestKey) > 200 {
		return code("invalid_request")
	}
	now := time.Now().UTC()
	result := tx.Model(&WorkspaceSubmission{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ? AND state = ? AND archive_operation_id IS NULL", intent.SubmissionID, p.UserID, p.TenantID, intent.Request.ExpectedRevision, "accepted").Updates(map[string]any{"archive_operation_id": op.ID, "state": "archiving", "revision": intent.Request.ExpectedRevision + 1, "updated_at": now})
	if result.Error != nil {
		return code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return code("revision_conflict")
	}
	if intent.RequestKey != "" {
		receipt := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "submission.archive", RequestKey: intent.RequestKey, RequestDigest: intent.RequestDigest, ResourceID: op.ID, ResultRevision: intent.Request.ExpectedRevision + 1, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
	}
	audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.submission.archive", Resource: intent.SubmissionID, Outcome: "success", At: now}
	if err := tx.Create(&audit).Error; err != nil {
		return code("audit_unavailable")
	}
	return nil
}
