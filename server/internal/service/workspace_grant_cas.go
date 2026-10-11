package service

import (
	"math"
	"time"

	"gorm.io/gorm"
)

// changeWorkspaceGrantStateOn is an internal transaction primitive. Callers
// must authenticate the human and persist idempotency/audit in this same tx.
// It never enables authentication or modifies a legacy Agent credential.
func changeWorkspaceGrantStateOn(tx *gorm.DB, owner, tenant, id string, expected uint64, action string, now time.Time) (WorkspaceGrant, error) {
	var grant WorkspaceGrant
	if owner == "" || id == "" || expected == math.MaxUint64 {
		return grant, code("invalid_request")
	}
	if action != "disable" && action != "enable" && action != "revoke" {
		return grant, code("invalid_request")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, owner, tenant).First(&grant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkspaceGrant{}, code("not_found")
		}
		return WorkspaceGrant{}, code("database_unavailable")
	}
	if grant.Revision != expected {
		return WorkspaceGrant{}, code("revision_conflict")
	}
	state := workspaceGrantState(grant, now)
	if state != "active" && state != "disabled" {
		return WorkspaceGrant{}, code("forbidden")
	}
	updates := map[string]any{"revision": expected + 1, "updated_at": now}
	switch action {
	case "disable":
		updates["disabled"] = true
	case "enable":
		updates["disabled"] = false
	case "revoke":
		updates["revoked"] = true
	}
	result := tx.Model(&WorkspaceGrant{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, owner, tenant, expected).Updates(updates)
	if result.Error != nil {
		return WorkspaceGrant{}, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return WorkspaceGrant{}, code("revision_conflict")
	}
	grant.Revision = expected + 1
	grant.UpdatedAt = now
	if action == "revoke" {
		grant.Revoked = true
	} else {
		grant.Disabled = action == "disable"
	}
	return grant, nil
}
