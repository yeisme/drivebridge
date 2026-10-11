package service

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// authorizeWorkspaceActionOn is a transaction-bound gate for dedicated
// workspace actions. It deliberately does not enable legacy endpoints or
// authenticate new credentials. Callers must also check the concrete resource
// and, for uploads, the persisted operation purpose and grant identity.
func (s *Service) authorizeWorkspaceActionOn(db *gorm.DB, p Principal, spaceID, connectionID, resourcePath, action string, now time.Time) (WorkspaceGrant, error) {
	var denied WorkspaceGrant
	switch action {
	case "files.read", "labels.suggest", "inbox.submit":
	default:
		return denied, code("forbidden")
	}
	if action != "inbox.submit" && workspaceStagingPath(resourcePath) {
		return denied, code("forbidden")
	}
	if p.Agent == nil || p.Agent.ID == "" || p.Agent.TokenHash == "" || spaceID == "" || connectionID == "" || p.UserID == "" || p.ShareID != "" || p.AccessGrantHash != "" || p.ServiceIdentity != nil {
		return denied, code("forbidden")
	}
	var owner User
	if err := db.Where(&User{ID: p.UserID}).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
		return denied, code("forbidden")
	}
	var space Space
	if err := db.Where(&Space{ID: spaceID, Owner: p.UserID}).First(&space).Error; err != nil || space.TenantID != p.TenantID || space.ConnectionID != connectionID {
		return denied, code("forbidden")
	}
	var agent Agent
	if err := db.Where(&Agent{ID: p.Agent.ID, Owner: p.UserID}).First(&agent).Error; err != nil {
		return denied, code("forbidden")
	}
	if agent.RestrictionProfile != "personal_workspace_v1" || p.Agent.RestrictionProfile != agent.RestrictionProfile || agent.Revoked || agent.Archived || agent.ArchiveSuspended || agent.TokenHash != p.Agent.TokenHash {
		return denied, code("forbidden")
	}
	var grant WorkspaceGrant
	if err := db.Where(&WorkspaceGrant{AgentID: agent.ID}).First(&grant).Error; err != nil {
		return denied, code("forbidden")
	}
	if grant.Owner != p.UserID || grant.TenantID != p.TenantID || grant.SpaceID != spaceID || grant.ConnectionID != connectionID || workspaceGrantState(grant, now) != "active" {
		return denied, code("forbidden")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != spaceID {
		return denied, code("forbidden")
	}
	if p.DataOwner != "" && p.DataOwner != grant.Owner {
		return denied, code("forbidden")
	}
	if agent.SpaceID != grant.SpaceID || agent.ConnectionID != grant.ConnectionID || agent.Prefix != grant.Prefix {
		return denied, code("forbidden")
	}
	if resourcePath != "" {
		if cleaned, err := cleanPath(resourcePath); err != nil || cleaned != resourcePath {
			return denied, code("forbidden")
		}
	}
	if grant.Prefix != "" {
		if cleaned, err := cleanPath(grant.Prefix); err != nil || cleaned != grant.Prefix {
			return denied, code("forbidden")
		}
	}
	if !contains(grant.Prefix, resourcePath) {
		return denied, code("forbidden")
	}
	allowed := false
	for _, candidate := range strings.Split(grant.Actions, ",") {
		switch candidate {
		case "files.read", "labels.suggest", "inbox.submit":
		default:
			return denied, code("forbidden")
		}
		if candidate == action {
			allowed = true
		}
	}
	if !allowed {
		return denied, code("forbidden")
	}
	// Suggestions and submissions need the human's write capability, while
	// reads retain the existing governance read policy.
	permission := "write"
	if action == "files.read" {
		permission = "read"
	}
	if err := s.governanceAllow(db, p, spaceID, resourcePath, permission); err != nil {
		return denied, err
	}
	return grant, nil
}
