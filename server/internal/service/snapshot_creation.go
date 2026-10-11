package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) snapshotCreationAdmissionOn(tx *gorm.DB, p Principal, id string, write bool) (FileCollection, error) {
	if !collectionHumanPrincipal(p) {
		return FileCollection{}, code("forbidden")
	}
	c, err := s.collectionReadAdmissionOn(tx, p, id)
	if err != nil {
		return FileCollection{}, err
	}
	if write {
		if err = s.allowOn(tx, p, c.SpaceID, "", "write"); err != nil {
			return FileCollection{}, err
		}
	}
	return c, nil
}

func (s *Service) snapshotCreationReceiptOn(tx *gorm.DB, p Principal, id, key string) (contract.SnapshotCreationReceipt, bool, error) {
	var out contract.SnapshotCreationReceipt
	var row WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "collection.snapshot", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, false, nil
	}
	if err != nil {
		return out, false, code("database_unavailable")
	}
	var snapshot CollectionSnapshot
	if err = tx.Where("id = ? AND owner = ?", row.ResourceID, p.UserID).First(&snapshot).Error; err != nil {
		return out, true, resourceReadError(err)
	}
	if snapshot.CollectionID != id {
		return out, true, code("idempotency_conflict")
	}
	digest, e := hex.DecodeString(row.RequestDigest)
	if e != nil || len(digest) != 32 || row.ResultRevision == 0 || row.ResultRevision > 9007199254740991 || snapshot.Revision != row.ResultRevision || row.CreatedAt.IsZero() || !snapshot.CreatedAt.Equal(row.CreatedAt) {
		return out, true, code("dependency_unavailable")
	}
	out = contract.SnapshotCreationReceipt{CollectionID: id, SnapshotID: snapshot.ID, RequestDigest: row.RequestDigest, CollectionRevision: row.ResultRevision, CommittedAt: row.CreatedAt}
	return out, true, nil
}

// CreateCollectionSnapshot records one original observation; retries never
// re-enumerate rule results after the original publication succeeded.
func (s *Service) CreateCollectionSnapshot(ctx context.Context, p Principal, id string, revision uint64, key string) (contract.SnapshotCreationReceipt, error) {
	var out contract.SnapshotCreationReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if revision == 0 || revision > 9007199254740991 {
		return out, code("invalid_request")
	}
	raw, _ := json.Marshal(struct {
		CollectionID     string `json:"collection_id"`
		ExpectedRevision uint64 `json:"expected_revision"`
	}{id, revision})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, err := s.snapshotCreationAdmissionOn(tx, p, id, true)
		if err != nil {
			return err
		}
		out, found, err = s.snapshotCreationReceiptOn(tx, p, id, key)
		if err != nil {
			return err
		}
		if found {
			if out.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			return nil
		}
		if c.Revision != revision {
			return code("collection_revision_conflict")
		}
		return nil
	})
	if err != nil {
		return contract.SnapshotCreationReceipt{}, err
	}
	if found {
		return out, nil
	}
	// This is the existing bounded dynamic observation, not an atomic catalogue
	// snapshot. Publication rechecks every captured version and collection revision.
	if err = ctx.Err(); err != nil {
		return out, err
	}
	read, err := s.ReadFileCollection(p, id)
	if err != nil {
		return out, err
	}
	if read.Revision != revision {
		return out, code("collection_revision_conflict")
	}
	if read.Coverage != "complete" || len(read.Items) > 30000 {
		return out, code("query_budget_exceeded")
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.snapshotCreationAdmissionOn(tx, p, id, true); err != nil {
			return err
		}
		previous, exists, err := s.snapshotCreationReceiptOn(tx, p, id, key)
		if err != nil {
			return err
		}
		if exists {
			if previous.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = previous
			return nil
		}
		snapshot, err := s.createFileSnapshotOn(tx, p, id, read)
		if err != nil {
			return err
		}
		row := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "collection.snapshot", RequestKey: key, RequestDigest: digest, ResourceID: snapshot.SnapshotID, ResultRevision: snapshot.CollectionRevision, CreatedAt: snapshot.CreatedAt}
		if err = tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		out = contract.SnapshotCreationReceipt{CollectionID: id, SnapshotID: snapshot.SnapshotID, RequestDigest: digest, CollectionRevision: snapshot.CollectionRevision, CommittedAt: snapshot.CreatedAt}
		return nil
	})
	if err != nil {
		return contract.SnapshotCreationReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupSnapshotCreation(ctx context.Context, p Principal, id, key string) (contract.SnapshotCreationReceipt, error) {
	var out contract.SnapshotCreationReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.snapshotCreationAdmissionOn(tx, p, id, false); err != nil {
			return err
		}
		var found bool
		var err error
		out, found, err = s.snapshotCreationReceiptOn(tx, p, id, key)
		if err != nil {
			return err
		}
		if !found {
			return code("not_found")
		}
		return nil
	})
	if err != nil {
		return contract.SnapshotCreationReceipt{}, err
	}
	return out, nil
}

func (s *Service) snapshotCreationHandler(lookup bool) func(http.ResponseWriter, *http.Request, Principal) error {
	return func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		var out contract.SnapshotCreationReceipt
		var err error
		if lookup {
			input, e := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
			if e != nil {
				return code("invalid_request")
			}
			out, err = s.LookupSnapshotCreation(r.Context(), p, r.PathValue("id"), input.IdempotencyKey)
		} else {
			input, e := contract.DecodeRequest[contract.SnapshotCreationRequest](http.MaxBytesReader(w, r.Body, 1024))
			if e != nil {
				return code("invalid_request")
			}
			keys := r.Header.Values("Idempotency-Key")
			if len(keys) != 1 {
				return code("invalid_request")
			}
			out, err = s.CreateCollectionSnapshot(r.Context(), p, r.PathValue("id"), input.ExpectedRevision, keys[0])
		}
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}
}
