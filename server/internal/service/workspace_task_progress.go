package service

import (
	"errors"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// This adapter counts the single file publication, not bytes transferred,
// downstream acceptance, or present-day provider availability.
func operationTaskProgressOn(tx *gorm.DB, op Operation) (contract.TaskProgress, error) {
	var progress contract.TaskProgress
	if op.Kind != "upload" {
		return progress, nil
	}
	one, zero := int64(1), int64(0)
	progress.Total = &one
	switch op.Status {
	case "awaiting_upload", "transferring", "verifying", "queued", "running", "pending", "paused", "interrupted", "failed":
		progress.Pending = &one
	case "completed":
		// Old completions can outlive retention of their fixed output. Do not use
		// the latest head, transfer counter or a different owner's object instead.
		_, _, found, err := operationUploadResultOn(tx, op)
		if err != nil || !found {
			return progress, err
		}
		progress.Succeeded = &one
		progress.Failed = &zero
		progress.Pending = &zero
	}
	return progress, nil
}

// This is publication identity, not current file-read permission.
func operationUploadResultOn(tx *gorm.DB, op Operation) (File, Version, bool, error) {
	var file File
	var version Version
	if op.Kind != "upload" || op.Status != "completed" || op.FileID == "" || op.VersionID == "" {
		return file, version, false, nil
	}
	err := tx.Where("id = ? AND owner = ? AND space_id = ?", op.FileID, op.Owner, op.SpaceID).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return File{}, Version{}, false, nil
	}
	if err != nil {
		return File{}, Version{}, false, code("database_unavailable")
	}
	if file.PurgeStartedAt != nil {
		return File{}, Version{}, false, nil
	}
	err = tx.Where("id = ? AND owner = ? AND file_id = ?", op.VersionID, op.Owner, op.FileID).First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return File{}, Version{}, false, nil
	}
	if err != nil {
		return File{}, Version{}, false, code("database_unavailable")
	}
	if version.Size != op.Size || version.SHA256 != op.SHA256 || version.ConnectionID != op.ConnectionID || version.ObjectKey == "" {
		return File{}, Version{}, false, nil
	}
	return file, version, true, nil
}
