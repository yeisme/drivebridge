package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func collectionChangeKey(key string) error {
	if key == "" || len(key) > 200 {
		return code("invalid_request")
	}
	for _, ch := range key {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	return nil
}

func (s *Service) collectionMembersChangeAdmissionOn(tx *gorm.DB, p Principal, id string) error {
	if !collectionHumanPrincipal(p) {
		return code("forbidden")
	}
	if _, err := s.collectionReadAdmissionOn(tx, p, id); err != nil {
		return err
	}
	return nil
}

func collectionMembersReceipt(row WorkspaceRequest, id string) (contract.CollectionMembersReceipt, error) {
	digest, err := hex.DecodeString(row.RequestDigest)
	if row.ResourceID != id {
		return contract.CollectionMembersReceipt{}, code("idempotency_conflict")
	}
	if err != nil || len(digest) != 32 || row.ResultRevision < 2 || row.ResultRevision > 9007199254740991 || row.CreatedAt.IsZero() {
		return contract.CollectionMembersReceipt{}, code("dependency_unavailable")
	}
	return contract.CollectionMembersReceipt{CollectionID: row.ResourceID, RequestDigest: row.RequestDigest, ResultRevision: row.ResultRevision, CommittedAt: row.CreatedAt}, nil
}

func (s *Service) ChangeCollectionMembers(ctx context.Context, p Principal, id string, q contract.CollectionMembersChangeRequest, key string) (contract.CollectionMembersReceipt, error) {
	var out contract.CollectionMembersReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if q.ExpectedRevision == 0 || q.ExpectedRevision >= 9007199254740991 || q.Include == nil || q.Exclude == nil || len(q.Include)+len(q.Exclude) > 10000 {
		return out, code("invalid_request")
	}
	seen := map[string]bool{}
	for _, ids := range [][]string{q.Include, q.Exclude} {
		for _, id := range ids {
			raw, err := hex.DecodeString(id)
			if err != nil || len(raw) != 24 || hex.EncodeToString(raw) != id || seen[id] {
				return out, code("invalid_request")
			}
			seen[id] = true
		}
	}
	raw, _ := json.Marshal(struct {
		CollectionID string                                  `json:"collection_id"`
		Request      contract.CollectionMembersChangeRequest `json:"request"`
	}{id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.collectionMembersChangeAdmissionOn(tx, p, id); err != nil {
			return err
		}
		var row WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "collection.members", key).First(&row).Error
		if err == nil {
			if row.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out, err = collectionMembersReceipt(row, id)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		view, err := s.setCollectionMembersOn(tx, p, id, SetCollectionMembersRequest{ExpectedRevision: q.ExpectedRevision, Include: q.Include, Exclude: q.Exclude})
		if err != nil {
			return err
		}
		row = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "collection.members", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: view.Revision, CreatedAt: time.Now().UTC()}
		if err = tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		out, err = collectionMembersReceipt(row, id)
		return err
	})
	if err != nil {
		return contract.CollectionMembersReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupCollectionMembersChange(ctx context.Context, p Principal, id, key string) (contract.CollectionMembersReceipt, error) {
	var out contract.CollectionMembersReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.collectionMembersChangeAdmissionOn(tx, p, id); err != nil {
			return err
		}
		var row WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "collection.members", key).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		var err error
		out, err = collectionMembersReceipt(row, id)
		return err
	})
	if err != nil {
		return contract.CollectionMembersReceipt{}, err
	}
	return out, nil
}

func (s *Service) collectionMembersChangeHandler(lookup bool) func(http.ResponseWriter, *http.Request, Principal) error {
	return func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		var out contract.CollectionMembersReceipt
		var err error
		if lookup {
			input, e := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
			if e != nil {
				return code("invalid_request")
			}
			out, err = s.LookupCollectionMembersChange(r.Context(), p, r.PathValue("id"), input.IdempotencyKey)
		} else {
			input, e := contract.DecodeRequest[contract.CollectionMembersChangeRequest](http.MaxBytesReader(w, r.Body, 1024*1024))
			if e != nil {
				return code("invalid_request")
			}
			keys := r.Header.Values("Idempotency-Key")
			if len(keys) != 1 {
				return code("invalid_request")
			}
			out, err = s.ChangeCollectionMembers(r.Context(), p, r.PathValue("id"), input, keys[0])
		}
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}
}
