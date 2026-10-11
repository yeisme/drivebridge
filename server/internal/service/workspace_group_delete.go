package service

import (
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// deleteWorkspaceGroupOn deletes organization metadata only. The caller owns
// the transaction, idempotency receipt and audit. Pinned state is preserved.
func (s *Service) deleteWorkspaceGroupOn(tx *gorm.DB, p Principal, id string, revision uint64, now time.Time) error {
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return err
	}
	var group WorkspaceAgentGroup
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&group).Error; err != nil {
		return code("not_found")
	}
	if revision == 0 || group.Revision != revision {
		return code("revision_conflict")
	}
	// Refuse overflow rather than wrapping an organization revision.
	var organizations []WorkspaceAgentOrganization
	if err := tx.Where("owner = ? AND tenant_id = ? AND group_id = ?", p.UserID, p.TenantID, id).Find(&organizations).Error; err != nil {
		return code("database_unavailable")
	}
	for _, organization := range organizations {
		if organization.Revision == math.MaxUint64 {
			return code("revision_conflict")
		}
	}
	result := tx.Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, p.UserID, p.TenantID, revision).Delete(&WorkspaceAgentGroup{})
	if result.Error != nil {
		return code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return code("revision_conflict")
	}
	if err := tx.Model(&WorkspaceAgentOrganization{}).Where("owner = ? AND tenant_id = ? AND group_id = ?", p.UserID, p.TenantID, id).Updates(map[string]any{"group_id": nil, "revision": gorm.Expr("revision + ?", 1), "updated_at": now}).Error; err != nil {
		return code("database_unavailable")
	}
	return nil
}
