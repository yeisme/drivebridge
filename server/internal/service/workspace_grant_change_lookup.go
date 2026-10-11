package service

import (
	"errors"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Read authorization and the original receipt in one DB snapshot. No state,
// audit or receipt is written and no current-state inference is returned.
func (s *Service) workspaceGrantChangeLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.GrantChangeLookupRequest](r.Body)
	if err != nil {
		return err
	}
	if err = input.Validate(); err != nil {
		return err
	}
	var view contract.GrantChangeReceiptView
	lookupError := func(err error) error {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		return code("database_unavailable")
	}
	err = s.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		var human User
		if err := tx.Where("id = ?", p.UserID).First(&human).Error; err != nil {
			return lookupError(err)
		}
		if human.Disabled || human.Archived {
			return code("forbidden")
		}
		var grant WorkspaceGrant
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&grant).Error; err != nil {
			return lookupError(err)
		}
		if (p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID) || (p.DataOwner != "" && p.DataOwner != grant.Owner) {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
			return lookupError(err)
		}
		if err := s.governanceAllow(tx, p, space.ID, grant.Prefix, "permission"); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ? AND resource_id = ?", p.UserID, p.TenantID, p.UserID, "grant."+input.Action, input.IdempotencyKey, grant.ID).First(&receipt).Error; err != nil {
			return lookupError(err)
		}
		if receipt.ResultRevision == 0 || receipt.CreatedAt.IsZero() {
			return code("database_unavailable")
		}
		view = contract.GrantChangeReceiptView{GrantID: grant.ID, Action: input.Action, ResultRevision: receipt.ResultRevision, CommittedAt: receipt.CreatedAt}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}
