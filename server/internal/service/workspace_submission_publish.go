package service

import (
	"time"

	"gorm.io/gorm"
)

// finishWorkspaceSubmissionOn joins the caller's file/version publication
// transaction. It must never be called after that transaction has committed.
func (s *Service) finishWorkspaceSubmissionOn(tx *gorm.DB, p Principal, id string, now time.Time) error {
	op, sub, err := s.workspaceOperationOn(tx, p, id, now)
	if err != nil {
		return err
	}
	if op.Status != "completed" || op.FileID == "" || op.VersionID == "" {
		return code("invalid_operation")
	}
	var file File
	if err := tx.Where("id = ? AND owner = ? AND space_id = ?", op.FileID, p.UserID, op.SpaceID).First(&file).Error; err != nil {
		return code("invalid_operation")
	}
	if file.VersionID != op.VersionID || file.Path != op.Path || file.Trashed || file.PurgeStartedAt != nil || !file.ReviewPending {
		return code("invalid_operation")
	}
	var version Version
	if err := tx.Where("id = ? AND owner = ? AND file_id = ?", op.VersionID, p.UserID, op.FileID).First(&version).Error; err != nil {
		return code("invalid_operation")
	}
	if version.ConnectionID != op.ConnectionID || version.SHA256 != op.SHA256 || version.Size != op.Size {
		return code("invalid_operation")
	}
	if sub.State == "submitted" && sub.FileID != nil && sub.VersionID != nil && *sub.FileID == file.ID && *sub.VersionID == version.ID {
		return nil
	}
	if sub.State != "uploading" || sub.Revision != 1 || sub.FileID != nil || sub.VersionID != nil {
		return code("revision_conflict")
	}
	result := tx.Model(&WorkspaceSubmission{}).Where("id = ? AND revision = ? AND state = ? AND file_id IS NULL AND version_id IS NULL", sub.ID, sub.Revision, "uploading").Updates(map[string]any{"file_id": file.ID, "version_id": version.ID, "state": "submitted", "revision": 2, "updated_at": now})
	if result.Error != nil {
		return code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return code("revision_conflict")
	}
	audit := Audit{Owner: p.UserID, ActorID: p.UserID, AgentID: op.AgentID, TenantID: p.TenantID, Action: "workspace.submission.submitted", Resource: sub.ID, Outcome: "success", At: now}
	if err := tx.Create(&audit).Error; err != nil {
		return code("audit_unavailable")
	}
	return nil
}
