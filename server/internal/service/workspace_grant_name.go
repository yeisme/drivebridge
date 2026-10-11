package service

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Rename preserves credential and expiry state. The caller supplies the
// authorization, transaction and audit, just as for expiry mutations.
func changeWorkspaceGrantNameOn(tx *gorm.DB, owner, tenant, actor, id, key string, expected uint64, name string, now time.Time) (WorkspaceRequest, error) {
	var receipt WorkspaceRequest
	input := contract.PatchGrantRequest{Name: contract.Field[string]{Present: true, Value: name}}
	if err := input.Validate(); err != nil {
		return receipt, err
	}
	if owner == "" || actor == "" || id == "" || key == "" || len(key) > 200 || expected == math.MaxUint64 {
		return receipt, code("invalid_request")
	}
	payload, err := json.Marshal(struct {
		ID       string
		Revision uint64
		Name     string
	}{id, expected, name})
	if err != nil {
		return receipt, code("invalid_request")
	}
	digest := Hash(string(payload))
	err = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", owner, tenant, actor, "grant.rename", key).First(&receipt).Error
	if err == nil {
		if receipt.RequestDigest != digest {
			return WorkspaceRequest{}, code("idempotency_conflict")
		}
		return receipt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return receipt, code("database_unavailable")
	}
	var grant WorkspaceGrant
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, owner, tenant).First(&grant).Error; err != nil {
		return receipt, code("not_found")
	}
	if grant.Revision != expected {
		return receipt, code("revision_conflict")
	}
	state := workspaceGrantState(grant, now)
	if state != "active" && state != "disabled" {
		return receipt, code("forbidden")
	}
	result := tx.Model(&WorkspaceGrant{}).Where("id = ? AND revision = ?", id, expected).Updates(map[string]any{"revision": expected + 1, "updated_at": now})
	if result.Error != nil {
		return receipt, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return receipt, code("revision_conflict")
	}
	result = tx.Model(&Agent{}).Where("id = ? AND owner = ? AND restriction_profile = ?", grant.AgentID, owner, "personal_workspace_v1").Update("name", name)
	if result.Error != nil {
		return receipt, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return receipt, code("not_found")
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: owner, TenantID: tenant, ActorID: actor, Action: "grant.rename", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: expected + 1, CreatedAt: now}
	if err := tx.Create(&receipt).Error; err != nil {
		return WorkspaceRequest{}, code("database_unavailable")
	}
	return receipt, nil
}
