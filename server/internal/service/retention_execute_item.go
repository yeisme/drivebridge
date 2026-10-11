package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Each item result commits with its database effects. A failed result write
// rolls everything back; a commit error is not evidence of non-commit.
func (s *Service) executeRetentionItem(ctx context.Context, p Principal, expected RetentionItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.retentionHumanOn(tx, p); err != nil {
			return err
		}
		var plan RetentionPlan
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&plan, "id = ? AND owner = ?", expected.PlanID, p.dataOwner()).Error; err != nil {
			return code("database_unavailable")
		}
		if err := retentionPlanCurrentOn(tx, plan); err != nil {
			return err
		}
		var item RetentionItem
		if err := tx.First(&item, "id = ? AND plan_id = ?", expected.ID, plan.ID).Error; err != nil {
			return code("database_unavailable")
		}
		if item.State != "candidate" {
			return nil
		}
		if plan.Status != "preview" && plan.Status != "executing" {
			return code("retention_plan_stale")
		}
		if item.FileID != expected.FileID || item.VersionID != expected.VersionID || item.Kind != expected.Kind {
			return code("dependency_unavailable")
		}
		now := time.Now().UTC()
		// Check rollback itself before persisting a failed result. If savepoint
		// recovery fails, the outer transaction must also be rolled back.
		if err := tx.SavePoint("retention_item_effects").Error; err != nil {
			return code("database_unavailable")
		}
		state, detail, err := s.retentionItemEffects(tx, p, item, now)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if rollback := tx.RollbackTo("retention_item_effects").Error; rollback != nil {
				return code("database_unavailable")
			}
			state, detail = "failed", safeCode(err)
		}
		result := tx.Model(&RetentionItem{}).Where("id = ? AND plan_id = ? AND state = ?", item.ID, plan.ID, "candidate").Updates(map[string]any{"state": state, "detail": detail, "updated_at": now})
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return code("dependency_unavailable")
		}
		return nil
	})
}

func (s *Service) retentionItemEffects(tx *gorm.DB, p Principal, item RetentionItem, now time.Time) (string, string, error) {
	var f File
	if err := tx.First(&f, "id = ? AND owner = ?", item.FileID, p.dataOwner()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "skipped_changed", "file_gone", nil
		}
		return "", "", code("database_unavailable")
	}
	if err := s.retentionFileDeleteOn(tx, p, f); err != nil {
		return "", "", err
	}
	if item.Kind != "file" && item.Kind != "version" {
		return "", "", code("dependency_unavailable")
	}
	if item.Kind == "file" && !f.Trashed {
		return "skipped_changed", "file_restored", nil
	}
	var versions []Version
	if item.Kind == "file" {
		if err := tx.Where("file_id = ? AND owner = ?", f.ID, p.dataOwner()).Find(&versions).Error; err != nil {
			return "", "", code("database_unavailable")
		}
	} else {
		var v Version
		if err := tx.First(&v, "id = ? AND file_id = ? AND owner = ?", item.VersionID, f.ID, p.dataOwner()).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "skipped_changed", "version_gone", nil
			}
			return "", "", code("database_unavailable")
		}
		if f.VersionID == v.ID {
			return "skipped_changed", "version_is_live", nil
		}
		versions = []Version{v}
	}
	for _, v := range versions {
		hold, err := activeHoldForVersion(tx, f.ID, v.ID, now)
		if err != nil {
			return "", "", err
		}
		if hold != nil {
			return "skipped_protected", "protected_by:" + hold.ID + ":" + hold.Purpose, nil
		}
	}
	var sp Space
	if err := tx.First(&sp, "id = ? AND owner = ?", f.SpaceID, p.dataOwner()).Error; err != nil {
		return "", "", code("database_unavailable")
	}
	var total int64
	for _, v := range versions {
		total += v.Size
		if v.BlobSHA256 != "" {
			// Resolve on this transaction, never through the outer DB connection.
			var blob Blob
			err := tx.First(&blob, "id = ?", blobID(v.Owner, blobScope(sp), v.BlobSHA256)).Error
			if err != nil {
				return "", "", code("database_unavailable")
			}
			if blob.Refs > 0 {
				if err := tx.Model(&blob).Update("refs", blob.Refs-1).Error; err != nil {
					return "", "", code("database_unavailable")
				}
			}
		}
	}
	if item.Kind == "file" {
		res := tx.Where("id = ? AND owner = ? AND trashed = ?", f.ID, p.dataOwner(), true).Delete(&File{})
		if res.Error != nil {
			return "", "", code("database_unavailable")
		}
		if res.RowsAffected != 1 {
			return "", "", code("trash_required")
		}
		if err := tx.Where("file_id = ? AND owner = ?", f.ID, p.dataOwner()).Delete(&Version{}).Error; err != nil {
			return "", "", code("database_unavailable")
		}
		if err := tx.Model(&sp).Update("used", sp.Used-total).Error; err != nil {
			return "", "", code("database_unavailable")
		}
		if err := s.recordFileEvent(tx, p.dataOwner(), f.ID, now.UnixNano(), true, map[string]any{"path": f.Path, "purged": true, "plan": "retention"}); err != nil {
			return "", "", code("database_unavailable")
		}
	} else {
		res := tx.Where("id = ? AND file_id = ?", item.VersionID, f.ID).Delete(&Version{})
		if res.Error != nil {
			return "", "", code("database_unavailable")
		}
		if res.RowsAffected != 1 {
			return "", "", code("dependency_unavailable")
		}
	}
	return "deleted", "", nil
}
