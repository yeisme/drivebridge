package service

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// finishWorkspaceArchiveOn joins the target publication transaction. Legacy
// archives have no submission binding and retain their existing behavior.
func (s *Service) finishWorkspaceArchiveOn(tx *gorm.DB, p Principal, op Operation, now time.Time) error {
	if op.Kind != "archive" {
		return nil
	}
	var sub WorkspaceSubmission
	err := tx.Where("archive_operation_id = ?", op.ID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return code("database_unavailable")
	}
	if p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" || sub.Owner != p.UserID || sub.TenantID != p.TenantID || op.Owner != p.UserID {
		return code("forbidden")
	}
	var human User
	if err := tx.Where("id = ?", p.UserID).First(&human).Error; err != nil || human.Disabled || human.Archived {
		return code("forbidden")
	}
	if sub.State != "archiving" || sub.FileID == nil || sub.VersionID == nil || *sub.FileID != op.ArchiveFileID || *sub.VersionID != op.ArchiveVersionID || op.Status != "completed" || op.FileID == "" || op.RenameOnConflict {
		return code("invalid_archive")
	}
	var source File
	if err := tx.Where("id = ? AND owner = ? AND space_id = ?", *sub.FileID, p.UserID, sub.SpaceID).First(&source).Error; err != nil || source.Trashed || source.PurgeStartedAt != nil {
		return code("not_found")
	}
	if source.VersionID != *sub.VersionID {
		return code("version_conflict")
	}
	if err := s.governanceAllow(tx, p, sub.SpaceID, source.Path, "read"); err != nil {
		return err
	}
	result := tx.Model(&WorkspaceSubmission{}).Where("id = ? AND revision = ? AND state = ? AND archive_operation_id = ?", sub.ID, sub.Revision, "archiving", op.ID).Updates(map[string]any{"state": "archived", "revision": sub.Revision + 1, "archived_file_ref": op.FileID, "updated_at": now})
	if result.Error != nil {
		return code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return code("revision_conflict")
	}
	audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.submission.archived", Resource: sub.ID, Outcome: "success", At: now}
	if err := tx.Create(&audit).Error; err != nil {
		return code("audit_unavailable")
	}
	return nil
}
