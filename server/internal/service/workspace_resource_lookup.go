package service

import (
	"encoding/json"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Lookup is not a replay. It cannot allocate a descriptor or advance a check.
func (s *Service) workspaceResourceLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
	if r.URL.RawQuery != "" {
		return code("invalid_request")
	}
	input, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
	if err != nil {
		return code("invalid_request")
	}
	key := input.IdempotencyKey
	if key == "" || len(key) > 200 {
		return code("invalid_request")
	}
	for _, ch := range key {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	var out contract.ResourceRegistrationReceipt
	err = s.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "resource.register", key).First(&receipt).Error; err != nil {
			return resourceReadError(err)
		}
		if receipt.ResultRevision == 0 || receipt.ResultRevision > 9007199254740991 || receipt.CreatedAt.IsZero() {
			return code("dependency_unavailable")
		}
		var row WorkspaceResourceDescriptor
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		if row.Revision != receipt.ResultRevision {
			return code("revision_conflict")
		}
		view, err := s.resourceViewOn(tx, p, row)
		if err != nil {
			return err
		}
		// Only unchanged creation facts may be returned as the original receipt.
		// A mutable current view or matching name alone is never sufficient.
		original := contract.RegisterResourceRequest{SpaceID: view.SpaceID, FileRef: view.FileRef, SnapshotID: view.SnapshotID, Kind: view.Kind, DisplayName: view.DisplayName}
		raw, err := json.Marshal(original)
		if err != nil {
			return code("dependency_unavailable")
		}
		if Hash(string(raw)) != receipt.RequestDigest {
			return code("idempotency_conflict")
		}
		out = contract.ResourceRegistrationReceipt{Resource: view, ResultRevision: receipt.ResultRevision, CommittedAt: receipt.CreatedAt}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, out)
	return nil
}
