package service

import (
	"encoding/hex"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// workspaceGrantRotate delivers a freshly generated credential only after
// state, receipt and audit commit. Replayed requests return metadata only.
func (s *Service) workspaceGrantRotate(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.RevisionRequest](r.Body)
	if err != nil {
		return err
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
	var response any
	err = s.DB.Transaction(func(tx *gorm.DB) error {
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
		if grant.Protocol != "mcp" && grant.Protocol != "http" {
			return code("forbidden")
		}
		material, err := secureRandomBytes(32)
		if err != nil {
			return code("internal_error")
		}
		secret := "db_" + hex.EncodeToString(material)
		now := time.Now().UTC()
		_, issued, err := rotateWorkspaceRequestOn(tx, p.UserID, p.TenantID, p.UserID, grant.ID, keys[0], input.ExpectedRevision, Hash(secret), now)
		if err != nil {
			return err
		}
		id := grant.ID
		grant = WorkspaceGrant{}
		if err := tx.Where("id = ?", id).First(&grant).Error; err != nil {
			return code("database_unavailable")
		}
		var agent Agent
		if err := tx.Where("id = ? AND owner = ?", grant.AgentID, p.UserID).First(&agent).Error; err != nil {
			return code("not_found")
		}
		view := workspaceGrantView(grant, agent, now)
		if !issued {
			response = contract.GrantReplayView{Grant: view, Credential: contract.ReplayCredential{State: "not_replayable"}}
			return nil
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.grant.rotate", Resource: grant.ID, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		response = contract.GrantIssuedView{Grant: view, Credential: contract.IssuedCredential{Secret: secret, State: "issued"}}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, response)
	return nil
}
