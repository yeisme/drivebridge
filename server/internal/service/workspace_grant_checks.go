package service

import (
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Checks observe local configuration only. They neither probe storage nor
// claim that a remote Agent is reachable, and never extend the grant lifetime.
func (s *Service) workspaceGrantChecks(w http.ResponseWriter, r *http.Request, p Principal) error {
	input, err := contract.DecodeRequest[contract.RevisionRequest](r.Body)
	if err != nil {
		return err
	}
	view := contract.GrantChecksView{CheckedAt: time.Now().UTC(), Checks: []contract.GrantCheck{}, ConnectionState: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var grant WorkspaceGrant
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&grant).Error; err != nil {
			return code("not_found")
		}
		if p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
			return code("not_found")
		}
		if err := s.governanceAllow(tx, p, space.ID, grant.Prefix, "permission"); err != nil {
			return err
		}
		if grant.Revision != input.ExpectedRevision {
			return code("revision_conflict")
		}
		add := func(name string, ok bool) {
			status := "observed_error"
			if ok {
				status = "observed_ok"
			}
			view.Checks = append(view.Checks, contract.GrantCheck{Name: name, Status: status})
		}
		add("grant_active", workspaceGrantState(grant, view.CheckedAt) == "active")
		var agent Agent
		if err := tx.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
			return code("not_found")
		}
		add("identity_binding", agent.RestrictionProfile == "personal_workspace_v1" && agent.RestrictionProfile == grant.RestrictionProfile && agent.SpaceID == grant.SpaceID && agent.ConnectionID == grant.ConnectionID && agent.Prefix == grant.Prefix && !agent.Revoked && !agent.Archived && !agent.ArchiveSuspended)
		add("protocol_configured", grant.Protocol == "http" || grant.Protocol == "mcp")
		var connections int64
		if err := tx.Model(&Connection{}).Where("id = ? AND disabled = ? AND (owner = ? OR owner = ?)", grant.ConnectionID, false, p.UserID, "").Count(&connections).Error; err != nil {
			return code("database_unavailable")
		}
		add("storage_configuration", connections == 1 && space.ConnectionID == grant.ConnectionID)
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}
