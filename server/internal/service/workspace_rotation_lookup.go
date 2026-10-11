package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// workspaceRotationLookup returns the original generation, not a credential
// and not an instruction to rotate again. It never mutates the grant.
func (s *Service) workspaceRotationLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
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
	var grant WorkspaceGrant
	if err = s.DB.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&grant).Error; err != nil {
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
	var receipt WorkspaceRequest
	if err = s.DB.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ? AND resource_id = ?", p.UserID, p.TenantID, p.UserID, "grant.rotate", input.IdempotencyKey, grant.ID).First(&receipt).Error; err != nil {
		return code("not_found")
	}
	if receipt.CredentialGeneration == 0 {
		return code("not_found")
	}
	respond(w, contract.RotationReplayView{GrantID: grant.ID, CredentialGeneration: receipt.CredentialGeneration, Credential: contract.ReplayCredential{State: "not_replayable"}})
	return nil
}
