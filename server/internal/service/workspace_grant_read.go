package service

import (
	"net/http"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func workspaceGrantView(g WorkspaceGrant, a Agent, now time.Time) contract.GrantView {
	return contract.GrantView{GrantID: g.ID, AgentID: g.AgentID, Name: a.Name, Protocol: g.Protocol, SpaceID: g.SpaceID, ConnectionID: g.ConnectionID, Prefix: g.Prefix, Actions: strings.Split(g.Actions, ","), RestrictionProfile: g.RestrictionProfile, Revision: g.Revision, State: workspaceGrantState(g, now), ExpiryMode: g.ExpiryMode, ExpiresAt: g.ExpiresAt, IssuedAt: g.IssuedAt, ExpiryChangedAt: g.ExpiryChangedAt, CredentialGeneration: g.CredentialGeneration, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

// WorkspaceGrantGet is human-only; Agent credentials cannot inspect grants.
func (s *Service) workspaceGrantGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	var owner User
	if err := s.DB.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
		return code("forbidden")
	}
	var grant WorkspaceGrant
	if err := s.DB.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&grant).Error; err != nil {
		return code("not_found")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID {
		return code("not_found")
	}
	if p.DataOwner != "" && p.DataOwner != grant.Owner {
		return code("not_found")
	}
	var agent Agent
	if err := s.DB.Where("id = ? AND owner = ?", grant.AgentID, grant.Owner).First(&agent).Error; err != nil {
		return code("not_found")
	}
	var space Space
	if err := s.DB.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, grant.Owner, grant.TenantID).First(&space).Error; err != nil {
		return code("not_found")
	}
	if err := s.governanceAllow(s.DB, p, space.ID, grant.Prefix, "read"); err != nil {
		return err
	}
	now := time.Now().UTC()
	if (grant.Protocol != "mcp" && grant.Protocol != "http") || workspaceGrantState(grant, now) == "invalid" {
		return code("forbidden")
	}
	respond(w, workspaceGrantView(grant, agent, now))
	return nil
}
