package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// rotateWorkspaceRequestOn excludes the generated credential from the request
// fingerprint. Replay returns metadata and never performs another rotation.
// The boolean reports a new committed candidate; callers expose its secret
// only after their enclosing audit transaction commits successfully.
func rotateWorkspaceRequestOn(tx *gorm.DB, owner, tenant, actor, id, key string, expected uint64, newHash string, now time.Time) (WorkspaceRequest, bool, error) {
	var receipt WorkspaceRequest
	if owner == "" || actor == "" || id == "" || key == "" || len(key) > 200 {
		return receipt, false, code("invalid_request")
	}
	payload, err := json.Marshal(struct {
		ID       string
		Revision uint64
	}{id, expected})
	if err != nil {
		return receipt, false, code("invalid_request")
	}
	digest := Hash(string(payload))
	err = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", owner, tenant, actor, "grant.rotate", key).First(&receipt).Error
	if err == nil {
		if receipt.RequestDigest != digest {
			return WorkspaceRequest{}, false, code("idempotency_conflict")
		}
		return receipt, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return receipt, false, code("database_unavailable")
	}
	grant, err := rotateWorkspaceCredentialOn(tx, owner, tenant, id, expected, newHash, now)
	if err != nil {
		return receipt, false, err
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: owner, TenantID: tenant, ActorID: actor, Action: "grant.rotate", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: grant.Revision, CredentialGeneration: grant.CredentialGeneration, CreatedAt: now}
	if err = tx.Create(&receipt).Error; err != nil {
		return WorkspaceRequest{}, false, code("database_unavailable")
	}
	return receipt, true, nil
}
