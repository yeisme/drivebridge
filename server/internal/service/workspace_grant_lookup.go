package service

import (
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) workspaceGrantLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](r.Body)
	if err != nil {
		return err
	}
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return code("invalid_request")
	}
	var owner User
	if err = s.DB.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
		return code("forbidden")
	}
	var receipt WorkspaceRequest
	if err = s.DB.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "grant.create", input.IdempotencyKey).First(&receipt).Error; err != nil {
		return code("not_found")
	}
	var grant WorkspaceGrant
	if err = s.DB.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&grant).Error; err != nil {
		return code("not_found")
	}
	if (p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID) || (p.DataOwner != "" && p.DataOwner != grant.Owner) {
		return code("not_found")
	}
	var space Space
	if err = s.DB.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
		return code("not_found")
	}
	if err = s.governanceAllow(s.DB, p, space.ID, grant.Prefix, "permission"); err != nil {
		return err
	}
	var agent Agent
	if err = s.DB.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
		return code("not_found")
	}
	now := time.Now().UTC()
	if workspaceGrantState(grant, now) == "invalid" {
		return code("forbidden")
	}
	respond(w, contract.GrantReplayView{Grant: workspaceGrantView(grant, agent, now), Credential: contract.ReplayCredential{State: "not_replayable"}})
	return nil
}
