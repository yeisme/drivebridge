package service

import (
	"encoding/hex"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceGrantCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.CreateGrantRequest](r.Body)
	if err != nil {
		return err
	}
	if err = input.Validate(); err != nil {
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
		if (p.ScopeSpace != "" && p.ScopeSpace != input.SpaceID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", input.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
			return code("not_found")
		}
		if input.ConnectionID != space.ConnectionID {
			return code("forbidden")
		}
		if err := s.governanceAllow(tx, p, space.ID, input.Prefix, "permission"); err != nil {
			return err
		}
		for _, action := range input.Actions {
			permission := "write"
			if action == "files.read" {
				permission = "read"
			}
			if err := s.governanceAllow(tx, p, space.ID, input.Prefix, permission); err != nil {
				return err
			}
		}
		material, err := secureRandomBytes(32)
		if err != nil {
			return code("internal_error")
		}
		secret := "db_" + hex.EncodeToString(material)
		now := time.Now().UTC()
		grant, agent, issued, err := createWorkspaceGrantOn(tx, p.UserID, p.TenantID, p.UserID, keys[0], Hash(secret), input, now)
		if err != nil {
			return err
		}
		view := workspaceGrantView(grant, agent, now)
		if !issued {
			response = contract.GrantReplayView{Grant: view, Credential: contract.ReplayCredential{State: "not_replayable"}}
			return nil
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.grant.create", Resource: grant.ID, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		response = contract.GrantIssuedView{Grant: view, Credential: contract.IssuedCredential{Secret: secret, State: "issued"}}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	respond(w, response)
	return nil
}
