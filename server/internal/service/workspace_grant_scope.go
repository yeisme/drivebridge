package service

import (
	"math"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// patchWorkspaceGrantScopeOn changes both stored scope bindings in the caller's
// transaction. The caller must add its idempotency receipt and audit.
func (s *Service) patchWorkspaceGrantScopeOn(tx *gorm.DB, p Principal, id string, input contract.PatchGrantRequest, now time.Time) (WorkspaceGrant, error) {
	return s.patchWorkspaceGrantFieldsOn(tx, p, id, input, now)
}

func (s *Service) patchWorkspaceGrantFieldsOn(tx *gorm.DB, p Principal, id string, input contract.PatchGrantRequest, now time.Time) (WorkspaceGrant, error) {
	var grant WorkspaceGrant
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return grant, err
	}
	if err := input.Validate(); err != nil {
		return grant, err
	}
	if (!input.Prefix.Present && !input.Actions.Present && !input.Name.Present && !input.Enabled.Present && !input.ExpiryMode.Present) || input.ExpectedRevision == math.MaxUint64 {
		return grant, code("invalid_request")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&grant).Error; err != nil {
		return grant, code("not_found")
	}
	if grant.Revision != input.ExpectedRevision {
		return grant, code("revision_conflict")
	}
	state := workspaceGrantState(grant, now)
	if state != "active" && state != "disabled" {
		return grant, code("forbidden")
	}
	var sp Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, p.UserID, p.TenantID).First(&sp).Error; err != nil {
		return grant, code("not_found")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != sp.ID {
		return grant, code("not_found")
	}
	if err := s.governanceAllow(tx, p, sp.ID, grant.Prefix, "permission"); err != nil {
		return grant, err
	}
	prefix := grant.Prefix
	if input.Prefix.Present {
		prefix = input.Prefix.Value
	}
	if prefix != "" {
		cleaned, err := cleanPath(prefix)
		if err != nil || cleaned != prefix {
			return grant, code("invalid_request")
		}
	}
	if workspaceStagingPath(prefix) {
		return grant, code("forbidden")
	}
	if err := s.governanceAllow(tx, p, sp.ID, prefix, "permission"); err != nil {
		return grant, err
	}
	actions := grant.Actions
	if input.Actions.Present {
		actions = strings.Join(input.Actions.Value, ",")
	}
	for _, action := range strings.Split(actions, ",") {
		permission := "write"
		switch action {
		case "files.read":
			permission = "read"
		case "labels.suggest", "inbox.submit":
		default:
			return grant, code("forbidden")
		}
		if err := s.governanceAllow(tx, p, sp.ID, prefix, permission); err != nil {
			return grant, err
		}
	}
	var agent Agent
	if err := tx.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
		return grant, code("not_found")
	}
	if agent.RestrictionProfile != grant.RestrictionProfile || agent.SpaceID != grant.SpaceID || agent.ConnectionID != grant.ConnectionID || agent.Prefix != grant.Prefix || agent.Revoked || agent.Archived {
		return grant, code("forbidden")
	}
	updates := map[string]any{"prefix": prefix, "actions": actions, "revision": input.ExpectedRevision + 1, "updated_at": now}
	if input.Enabled.Present {
		updates["disabled"] = !input.Enabled.Value
	}
	if input.ExpiryMode.Present {
		mode, seconds, err := (contract.ExpiryInput{ExpiryMode: input.ExpiryMode, ExpiresInSeconds: input.ExpiresInSeconds}).Resolve(true)
		if err != nil {
			return grant, err
		}
		var deadline *time.Time
		if mode == "duration" {
			value := now.Add(time.Duration(seconds) * time.Second)
			deadline = &value
		}
		updates["expiry_mode"], updates["expires_at"], updates["expiry_changed_at"] = mode, deadline, now
	}
	result := tx.Model(&WorkspaceGrant{}).Where("id = ? AND revision = ?", id, input.ExpectedRevision).Updates(updates)
	if result.Error != nil {
		return grant, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return grant, code("revision_conflict")
	}
	if input.Prefix.Present || input.Name.Present {
		agentUpdates := map[string]any{"prefix": prefix}
		if input.Name.Present {
			agentUpdates["name"] = input.Name.Value
		}
		result = tx.Model(&Agent{}).Where("id = ? AND owner = ? AND prefix = ?", agent.ID, p.UserID, agent.Prefix).Updates(agentUpdates)
		if result.Error != nil {
			return grant, code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return grant, code("revision_conflict")
		}
	}
	grant = WorkspaceGrant{}
	if err := tx.Where("id = ?", id).First(&grant).Error; err != nil {
		return grant, code("database_unavailable")
	}
	return grant, nil
}
