package service

import (
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// workspaceGrantRevoke commits the grant revision, request receipt and audit
// together. It does not revoke or mutate any other Agent identity.
func (s *Service) workspaceGrantRevoke(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.RevisionRequest](r.Body)
	if err != nil {
		return err
	}
	return s.workspaceGrantStateChange(w, r, p, input.ExpectedRevision, "revoke", nil)
}

func (s *Service) workspaceGrantPatch(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.PatchGrantRequest](r.Body)
	if err != nil {
		return err
	}
	if err = input.Validate(); err != nil {
		return err
	}
	if input.Name.Present && !input.Prefix.Present && !input.Actions.Present && !input.Enabled.Present && !input.ExpiryMode.Present {
		return s.workspaceGrantStateChange(w, r, p, input.ExpectedRevision, "rename", nil, input.Name.Value)
	}
	if (input.Prefix.Present || input.Actions.Present) && !input.Name.Present && !input.Enabled.Present && !input.ExpiryMode.Present {
		return s.workspaceGrantScopePatch(w, r, p, input)
	}
	if input.Name.Present || input.Prefix.Present || input.Actions.Present || (input.Enabled.Present && input.ExpiryMode.Present) {
		return s.workspaceGrantScopePatch(w, r, p, input)
	}
	if input.ExpiryMode.Present {
		expiry := contract.ExpiryInput{ExpiryMode: input.ExpiryMode, ExpiresInSeconds: input.ExpiresInSeconds}
		return s.workspaceGrantStateChange(w, r, p, input.ExpectedRevision, "expiry", &expiry)
	}
	if !input.Enabled.Present {
		return code("invalid_request")
	}
	action := "disable"
	if input.Enabled.Value {
		action = "enable"
	}
	return s.workspaceGrantStateChange(w, r, p, input.ExpectedRevision, action, nil)
}

func (s *Service) workspaceGrantStateChange(w http.ResponseWriter, r *http.Request, p Principal, expected uint64, action string, expiry *contract.ExpiryInput, names ...string) error {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" || len(keys[0]) > 200 {
		return code("invalid_request")
	}
	for _, ch := range keys[0] {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	var view contract.GrantView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var human User
		if err := tx.Where("id = ?", p.UserID).First(&human).Error; err != nil || human.Disabled || human.Archived {
			return code("forbidden")
		}
		var grant WorkspaceGrant
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&grant).Error; err != nil {
			return code("not_found")
		}
		if (p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID) || (p.DataOwner != "" && p.DataOwner != grant.Owner) {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
			return code("not_found")
		}
		if err := s.governanceAllow(tx, p, space.ID, grant.Prefix, "permission"); err != nil {
			return err
		}
		now := time.Now().UTC()
		previousRevision := grant.Revision
		var changeErr error
		if action == "rename" && len(names) == 1 {
			_, changeErr = changeWorkspaceGrantNameOn(tx, p.UserID, p.TenantID, p.UserID, grant.ID, keys[0], expected, names[0], now)
		} else if expiry != nil {
			_, changeErr = changeWorkspaceExpiryOn(tx, p.UserID, p.TenantID, p.UserID, grant.ID, keys[0], expected, *expiry, now)
		} else {
			_, changeErr = changeWorkspaceGrantRequestOn(tx, p.UserID, p.TenantID, p.UserID, grant.ID, keys[0], expected, action, now)
		}
		if changeErr != nil {
			return changeErr
		}
		grantID := grant.ID
		grant = WorkspaceGrant{}
		if err := tx.Where("id = ?", grantID).First(&grant).Error; err != nil {
			return code("database_unavailable")
		}
		var agent Agent
		if err := tx.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
			return code("not_found")
		}
		if grant.Protocol != "mcp" && grant.Protocol != "http" {
			return code("forbidden")
		}
		// A durable per-request audit is inserted only on the first application.
		if previousRevision == expected {
			audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.grant." + action, Resource: grant.ID, Outcome: "success", At: now}
			if err := tx.Create(&audit).Error; err != nil {
				return code("audit_unavailable")
			}
		}
		view = workspaceGrantView(grant, agent, now)
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, view)
	return nil
}
