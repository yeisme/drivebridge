package service

import (
	"time"

	"gorm.io/gorm"
)

// RetireSpaces is an offline operator operation. Spaces disappear from normal
// GORM reads; version bytes and audit history remain recoverable, never purged.
// Stop the HTTP service and workers and take a backup before calling it.
func (s *Service) RetireSpaces(ids []string) error {
	if len(ids) == 0 || len(ids) > 50 || !s.governanceActive() {
		return code("invalid_request")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if len(id) != 48 || seen[id] {
			return code("invalid_request")
		}
		seen[id] = true
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var spaces []Space
		if err := tx.Unscoped().Where("id IN ?", ids).Find(&spaces).Error; err != nil {
			return err
		}
		if len(spaces) != len(ids) {
			return code("not_found")
		}
		var pending int64
		if err := tx.Model(&Operation{}).Where("space_id IN ? AND status NOT IN ?", ids, []string{"completed", "cancelled", "failed", "awaiting_upload"}).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return code("operation_reconciliation_required")
		}
		now := time.Now().UTC()
		if err := tx.Model(&Operation{}).Where("space_id IN ? AND status = ?", ids, "awaiting_upload").Updates(map[string]any{"status": "cancelled", "updated": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Space{}).Where("id IN ?", ids).Update("reserved", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&Agent{}).Where("space_id IN ?", ids).Update("revoked", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&WorkspaceGrant{}).Where("space_id IN ?", ids).Updates(map[string]any{"revoked": true, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Share{}).Where("space_id IN ?", ids).Update("revoked", true).Error; err != nil {
			return err
		}
		for _, sp := range spaces {
			if sp.DeletedAt.Valid {
				continue
			}
			if err := tx.Delete(&sp).Error; err != nil {
				return err
			}
			if err := tx.Create(&GovernanceEvent{ID: ID(), TenantID: sp.TenantID, Action: "spaces.retire.operator", ResourceID: sp.ID, Outcome: "success", At: now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
