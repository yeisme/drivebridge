package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// changeWorkspaceGrantRequestOn commits a state change and secret-free receipt
// in the caller's transaction. Human authentication and auditing belong to the
// application entrypoint; this primitive is not an externally callable API.
func changeWorkspaceGrantRequestOn(tx *gorm.DB, owner, tenant, actor, id, key string, expected uint64, action string, now time.Time) (WorkspaceRequest, error) {
	var receipt WorkspaceRequest
	if owner == "" || actor == "" || id == "" || key == "" || len(key) > 200 {
		return receipt, code("invalid_request")
	}
	if action != "disable" && action != "enable" && action != "revoke" {
		return receipt, code("invalid_request")
	}
	payload, err := json.Marshal(struct {
		ID       string
		Revision uint64
		Action   string
	}{id, expected, action})
	if err != nil {
		return receipt, code("invalid_request")
	}
	digest := Hash(string(payload))
	query := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", owner, tenant, actor, "grant."+action, key)
	err = query.First(&receipt).Error
	if err == nil {
		if receipt.RequestDigest != digest {
			return WorkspaceRequest{}, code("idempotency_conflict")
		}
		return receipt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return receipt, code("database_unavailable")
	}
	grant, err := changeWorkspaceGrantStateOn(tx, owner, tenant, id, expected, action, now)
	if err != nil {
		return receipt, err
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: owner, TenantID: tenant, ActorID: actor, Action: "grant." + action, RequestKey: key, RequestDigest: digest, ResourceID: grant.ID, ResultRevision: grant.Revision, CreatedAt: now}
	if err = tx.Create(&receipt).Error; err != nil {
		return WorkspaceRequest{}, code("database_unavailable")
	}
	return receipt, nil
}
