package service

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// changeWorkspaceExpiryOn changes only expiry, with receipt in the same caller
// transaction. Replays preserve the original timestamp and never renew expiry.
func changeWorkspaceExpiryOn(tx *gorm.DB, owner, tenant, actor, id, key string, expected uint64, input contract.ExpiryInput, now time.Time) (WorkspaceRequest, error) {
	var receipt WorkspaceRequest
	if owner == "" || actor == "" || id == "" || key == "" || len(key) > 200 || expected == math.MaxUint64 {
		return receipt, code("invalid_request")
	}
	mode, seconds, err := input.Resolve(true)
	if err != nil {
		return receipt, err
	}
	if mode == "" {
		return receipt, code("invalid_expiry")
	}
	payload, err := json.Marshal(struct {
		ID       string
		Revision uint64
		Mode     string
		Seconds  int64
	}{id, expected, mode, seconds})
	if err != nil {
		return receipt, code("invalid_request")
	}
	digest := Hash(string(payload))
	err = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", owner, tenant, actor, "grant.expiry", key).First(&receipt).Error
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
	if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, owner, tenant).First(&grant).Error; err != nil {
		return receipt, code("not_found")
	}
	if grant.Revision != expected {
		return receipt, code("revision_conflict")
	}
	state := workspaceGrantState(grant, now)
	if state != "active" && state != "disabled" {
		return receipt, code("forbidden")
	}
	var expiry *time.Time
	if mode == "duration" {
		deadline := now.Add(time.Duration(seconds) * time.Second)
		expiry = &deadline
	}
	result := tx.Model(&WorkspaceGrant{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, owner, tenant, expected).Updates(map[string]any{"expiry_mode": mode, "expires_at": expiry, "expiry_changed_at": now, "revision": expected + 1, "updated_at": now})
	if result.Error != nil {
		return receipt, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return receipt, code("revision_conflict")
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: owner, TenantID: tenant, ActorID: actor, Action: "grant.expiry", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: expected + 1, CreatedAt: now}
	if err = tx.Create(&receipt).Error; err != nil {
		return WorkspaceRequest{}, code("database_unavailable")
	}
	return receipt, nil
}
