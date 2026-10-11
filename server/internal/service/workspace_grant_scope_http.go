package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceGrantScopePatch(w http.ResponseWriter, r *http.Request, p Principal, input contract.PatchGrantRequest) error {
	action := "grant.scope"
	if input.Name.Present || input.Enabled.Present || input.ExpiryMode.Present {
		action = "grant.patch"
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" || len(keys[0]) > 200 {
		return code("invalid_request")
	}
	for _, ch := range keys[0] {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	id := r.PathValue("id")
	payload, err := json.Marshal(struct {
		ID    string
		Input contract.PatchGrantRequest
	}{id, input})
	if err != nil {
		return code("invalid_request")
	}
	digest := Hash(string(payload))
	var view contract.GrantView
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var grant WorkspaceGrant
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&grant).Error; err != nil {
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
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, action, keys[0]).First(&receipt).Error
		now := time.Now().UTC()
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
		} else {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return code("database_unavailable")
			}
			grant, err = s.patchWorkspaceGrantFieldsOn(tx, p, id, input, now)
			if err != nil {
				return err
			}
			receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: action, RequestKey: keys[0], RequestDigest: digest, ResourceID: id, ResultRevision: grant.Revision, CreatedAt: now}
			if err := tx.Create(&receipt).Error; err != nil {
				return code("database_unavailable")
			}
			audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace." + action, Resource: id, Outcome: "success", At: now}
			if err := tx.Create(&audit).Error; err != nil {
				return code("audit_unavailable")
			}
		}
		var agent Agent
		if err := tx.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
			return code("not_found")
		}
		view = workspaceGrantView(grant, agent, now)
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}
