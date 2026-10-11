package service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func collectionCreationDigest(q CreateFileCollectionRequest) string {
	rule := ""
	if q.Kind == "rule" {
		rule = string(q.Rule)
	}
	// Keep the legacy byte-level identity, including its JSON field order.
	raw, _ := json.Marshal(struct {
		Space string `json:"space"`
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Scope string `json:"scope"`
		Rule  string `json:"rule"`
	}{q.SpaceID, q.Name, q.Kind, q.Scope, rule})
	return Hash(string(raw))
}

func collectionHumanPrincipal(p Principal) bool {
	return p.UserID != "" && p.Agent == nil && p.ServiceIdentity == nil && !p.ExternalRead && p.ShareID == "" && p.AccessGrantHash == "" && (p.DataOwner == "" || p.DataOwner == p.UserID)
}

func (s *Service) collectionCreationLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
	if r.URL.RawQuery != "" {
		return code("invalid_request")
	}
	input, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
	if err != nil {
		return code("invalid_request")
	}
	out, err := s.LookupCollectionCreation(r.Context(), p, input.IdempotencyKey)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, out)
	return nil
}

// LookupCollectionCreation reads persisted facts and never creates or repairs a row.
func (s *Service) LookupCollectionCreation(ctx context.Context, p Principal, key string) (contract.CollectionCreationReceipt, error) {
	var out contract.CollectionCreationReceipt
	if key == "" || len(key) > 200 {
		return out, code("invalid_request")
	}
	for _, ch := range key {
		if ch < 33 || ch > 126 {
			return out, code("invalid_request")
		}
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !collectionHumanPrincipal(p) {
			return code("forbidden")
		}
		if err := savedQueryActorOn(tx, p); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "collection.create", key).First(&receipt).Error; err != nil {
			return resourceReadError(err)
		}
		c, err := s.collectionReadAdmissionOn(tx, p, receipt.ResourceID)
		if err != nil {
			return err
		}
		if err = s.allowOn(tx, p, c.SpaceID, "", "read"); err != nil {
			return err
		}
		if receipt.ResultRevision != 1 || receipt.CreatedAt.IsZero() || c.Revision < 1 || c.Revision > 9007199254740991 {
			return code("dependency_unavailable")
		}
		// Member revisions may advance. The persisted creation identity must not.
		digest := collectionCreationDigest(CreateFileCollectionRequest{SpaceID: c.SpaceID, Name: c.Name, Kind: c.Kind, Scope: c.Scope, Rule: json.RawMessage(c.Rule)})
		if (c.Kind != "manual" && c.Kind != "rule") || (c.Kind == "manual" && c.Rule != "") || c.KeyID != key || c.Fingerprint != receipt.RequestDigest || digest != receipt.RequestDigest || !c.CreatedAt.Equal(receipt.CreatedAt) {
			return code("idempotency_conflict")
		}
		out = contract.CollectionCreationReceipt{CollectionID: c.ID, SpaceID: c.SpaceID, RequestDigest: receipt.RequestDigest, ResultRevision: 1, CommittedAt: receipt.CreatedAt}
		return nil
	})
	if err != nil {
		return contract.CollectionCreationReceipt{}, err
	}
	return out, nil
}
