package service

import (
	"encoding/hex"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// rotateWorkspaceCredentialOn accepts only a freshly generated hash, never a
// plaintext credential. The application owns one-time delivery, request
// receipts and audit, all in the same transaction before returning a secret.
func rotateWorkspaceCredentialOn(tx *gorm.DB, owner, tenant, id string, expected uint64, newHash string, now time.Time) (WorkspaceGrant, error) {
	var grant WorkspaceGrant
	if owner == "" || id == "" || expected == math.MaxUint64 || len(newHash) != 64 || newHash != strings.ToLower(newHash) {
		return grant, code("invalid_request")
	}
	if _, err := hex.DecodeString(newHash); err != nil {
		return grant, code("invalid_request")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, owner, tenant).First(&grant).Error; err != nil {
		return grant, code("not_found")
	}
	if grant.Revision != expected {
		return WorkspaceGrant{}, code("revision_conflict")
	}
	if grant.CredentialGeneration == math.MaxUint64 {
		return WorkspaceGrant{}, code("invalid_request")
	}
	state := workspaceGrantState(grant, now)
	if state != "active" && state != "disabled" {
		return WorkspaceGrant{}, code("forbidden")
	}
	var agent Agent
	if err := tx.Where("id = ? AND owner = ?", grant.AgentID, owner).First(&agent).Error; err != nil {
		return WorkspaceGrant{}, code("not_found")
	}
	if (agent.RestrictionProfile != "" && agent.RestrictionProfile != grant.RestrictionProfile) || agent.Revoked || agent.Archived || agent.ArchiveSuspended || agent.TokenHash == newHash {
		return WorkspaceGrant{}, code("forbidden")
	}
	// Reject collisions with live identities and any historical generation.
	for _, model := range []any{&Agent{}, &UserKey{}, &WorkspaceCredentialHistory{}} {
		var count int64
		if err := tx.Model(model).Where("token_hash = ?", newHash).Count(&count).Error; err != nil {
			return WorkspaceGrant{}, code("database_unavailable")
		}
		if count != 0 {
			return WorkspaceGrant{}, code("idempotency_conflict")
		}
	}
	result := tx.Model(&WorkspaceGrant{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, owner, tenant, expected).Updates(map[string]any{"revision": expected + 1, "credential_generation": grant.CredentialGeneration + 1, "updated_at": now})
	if result.Error != nil {
		return WorkspaceGrant{}, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return WorkspaceGrant{}, code("revision_conflict")
	}
	// Keep the new credential unusable by legacy binaries even when repairing
	// a missing profile marker on a pre-existing Agent binding.
	legacyExpiry := agent.Expires
	if legacyExpiry.After(now) {
		legacyExpiry = now
	}
	result = tx.Model(&Agent{}).Where("id = ? AND owner = ? AND token_hash = ?", agent.ID, owner, agent.TokenHash).Updates(map[string]any{"token_hash": newHash, "restriction_profile": grant.RestrictionProfile, "scopes": "", "expires": legacyExpiry})
	if result.Error != nil {
		return WorkspaceGrant{}, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return WorkspaceGrant{}, code("revision_conflict")
	}
	// Preserve the initial generation if a pre-history fixture is migrated.
	var previous WorkspaceCredentialHistory
	err := tx.Where("grant_id = ? AND generation = ?", id, grant.CredentialGeneration).First(&previous).Error
	if err == gorm.ErrRecordNotFound {
		previous = WorkspaceCredentialHistory{ID: ID(), GrantID: id, Generation: grant.CredentialGeneration, TokenHash: agent.TokenHash, CreatedAt: grant.IssuedAt, RevokedAt: &now}
		if err = tx.Create(&previous).Error; err != nil {
			return WorkspaceGrant{}, code("database_unavailable")
		}
	} else if err != nil {
		return WorkspaceGrant{}, code("database_unavailable")
	} else {
		if previous.TokenHash != agent.TokenHash || previous.RevokedAt != nil {
			return WorkspaceGrant{}, code("forbidden")
		}
		if err = tx.Model(&previous).Update("revoked_at", now).Error; err != nil {
			return WorkspaceGrant{}, code("database_unavailable")
		}
	}
	next := WorkspaceCredentialHistory{ID: ID(), GrantID: id, Generation: grant.CredentialGeneration + 1, TokenHash: newHash, CreatedAt: now}
	if err = tx.Create(&next).Error; err != nil {
		return WorkspaceGrant{}, code("database_unavailable")
	}
	grant.Revision++
	grant.CredentialGeneration++
	grant.UpdatedAt = now
	return grant, nil
}
