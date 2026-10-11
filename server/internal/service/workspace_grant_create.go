package service

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// createWorkspaceGrantOn creates an explicitly marked identity with no legacy
// scopes. Human authorization, audit and one-time delivery belong to the caller
// transaction. The currently closed authentication gate remains unchanged.
func createWorkspaceGrantOn(tx *gorm.DB, owner, tenant, actor, key, hash string, input contract.CreateGrantRequest, now time.Time) (WorkspaceGrant, Agent, bool, error) {
	var grant WorkspaceGrant
	var agent Agent
	fail := func(err error) (WorkspaceGrant, Agent, bool, error) { return WorkspaceGrant{}, Agent{}, false, err }
	if owner == "" || actor == "" || key == "" || len(key) > 200 {
		return fail(code("invalid_request"))
	}
	if err := input.Validate(); err != nil {
		return fail(err)
	}
	if input.Prefix != "" {
		if normalized, err := cleanPath(input.Prefix); err != nil || normalized != input.Prefix {
			return fail(code("invalid_request"))
		}
	}
	mode, seconds, err := (contract.ExpiryInput{ExpiryMode: input.ExpiryMode, ExpiresInSeconds: input.ExpiresInSeconds}).Resolve(false)
	if err != nil {
		return fail(err)
	}
	// Normalize defaults without serializing presence-wrapper internals.
	payload, err := json.Marshal(struct {
		Name, Protocol, Space, Connection, Prefix string
		Actions                                   []string
		Mode                                      string
		Seconds                                   int64
	}{input.Name, input.Protocol, input.SpaceID, input.ConnectionID, input.Prefix, input.Actions, mode, seconds})
	if err != nil {
		return fail(code("invalid_request"))
	}
	digest := Hash(string(payload))
	var receipt WorkspaceRequest
	err = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", owner, tenant, actor, "grant.create", key).First(&receipt).Error
	if err == nil {
		if receipt.RequestDigest != digest {
			return fail(code("idempotency_conflict"))
		}
		if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, owner, tenant).First(&grant).Error; err != nil {
			return fail(code("not_found"))
		}
		if err = tx.Where("id = ? AND owner = ?", grant.AgentID, owner).First(&agent).Error; err != nil {
			return fail(code("not_found"))
		}
		return grant, agent, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fail(code("database_unavailable"))
	}
	if len(hash) != 64 || hash != strings.ToLower(hash) {
		return fail(code("invalid_request"))
	}
	if _, err = hex.DecodeString(hash); err != nil {
		return fail(code("invalid_request"))
	}
	for _, model := range []any{&Agent{}, &UserKey{}, &WorkspaceCredentialHistory{}} {
		var count int64
		if err = tx.Model(model).Where("token_hash = ?", hash).Count(&count).Error; err != nil {
			return fail(code("database_unavailable"))
		}
		if count != 0 {
			return fail(code("idempotency_conflict"))
		}
	}
	var expires *time.Time
	if mode == "duration" {
		deadline := now.Add(time.Duration(seconds) * time.Second)
		expires = &deadline
	}
	// Legacy expiry remains fail-closed, not an artificial far-future sentinel.
	agent = Agent{ID: ID(), Owner: owner, Name: input.Name, SpaceID: input.SpaceID, ConnectionID: input.ConnectionID, Prefix: input.Prefix, TokenHash: hash, Expires: now, RestrictionProfile: "personal_workspace_v1"}
	grant = WorkspaceGrant{ID: ID(), AgentID: agent.ID, Owner: owner, TenantID: tenant, SpaceID: input.SpaceID, ConnectionID: input.ConnectionID, Protocol: input.Protocol, Prefix: input.Prefix, Actions: strings.Join(input.Actions, ","), RestrictionProfile: agent.RestrictionProfile, ExpiryMode: mode, ExpiresAt: expires, IssuedAt: now, Revision: 1, CredentialGeneration: 1, CreatedAt: now, UpdatedAt: now}
	if err = tx.Create(&agent).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	if err = tx.Create(&grant).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	history := WorkspaceCredentialHistory{ID: ID(), GrantID: grant.ID, Generation: 1, TokenHash: hash, CreatedAt: now}
	if err = tx.Create(&history).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: owner, TenantID: tenant, ActorID: actor, Action: "grant.create", RequestKey: key, RequestDigest: digest, ResourceID: grant.ID, ResultRevision: 1, CredentialGeneration: 1, CreatedAt: now}
	if err = tx.Create(&receipt).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	return grant, agent, true, nil
}
