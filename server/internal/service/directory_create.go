package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func directoryView(row Directory) contract.DirectoryView {
	return contract.DirectoryView{ID: row.ID, SpaceID: row.SpaceID, Path: row.Path}
}

func (s *Service) createDirectoryHTTP(w http.ResponseWriter, r *http.Request, p Principal) error {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) == 0 {
		// Keep the legacy Finder path unchanged for callers that do not opt into
		// the receipt contract yet.
		var q Directory
		if err := decode(r, &q); err != nil {
			return err
		}
		cleaned, err := cleanPath(q.Path)
		if err != nil {
			return err
		}
		q.Path = cleaned
		if _, err := s.space(p, q.SpaceID); err != nil {
			return err
		}
		if err := s.allow(p, q.SpaceID, q.Path, "write"); err != nil {
			return err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := pathAvailable(s.DB, p.dataOwner(), q.SpaceID, q.Path, ""); err != nil {
			return err
		}
		q.ID, q.Owner = ID(), p.dataOwner()
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := teamWritable(tx, q.SpaceID); e != nil {
				return e
			}
			return tx.Create(&q).Error
		}); err != nil {
			return code("name_conflict")
		}
		respond(w, q)
		return nil
	}
	if len(keys) != 1 {
		return code("invalid_request")
	}
	q, err := contract.DecodeRequest[contract.DirectoryCreate](http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		return code("invalid_request")
	}
	out, err := s.createDirectory(p, q, keys[0])
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	respond(w, out)
	return nil
}

func (s *Service) createDirectory(p Principal, q contract.DirectoryCreate, key string) (contract.DirectoryCreation, error) {
	var out contract.DirectoryCreation
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	pathValue, err := cleanPath(q.Path)
	if err != nil {
		return out, err
	}
	q.Path = pathValue
	if _, err = s.space(p, q.SpaceID); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(q)
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.allowOn(tx, p, q.SpaceID, q.Path, "write"); err != nil {
			return err
		}
		if err := teamWritable(tx, q.SpaceID); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "directory.create", key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			var existing Directory
			if err := tx.Where("id = ? AND owner = ? AND space_id = ?", receipt.ResourceID, p.dataOwner(), q.SpaceID).First(&existing).Error; err != nil {
				return resourceReadError(err)
			}
			out = contract.DirectoryCreation{Found: true, Status: "completed"}
			view := directoryView(existing)
			out.Directory = &view
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if err := pathAvailable(tx, p.dataOwner(), q.SpaceID, q.Path, ""); err != nil {
			return err
		}
		row := Directory{ID: ID(), Owner: p.dataOwner(), SpaceID: q.SpaceID, Path: q.Path}
		if err := tx.Create(&row).Error; err != nil {
			return code("name_conflict")
		}
		now := time.Now().UTC()
		if err := tx.Create(&WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "directory.create", RequestKey: key, RequestDigest: digest, ResourceID: row.ID, ResultRevision: 1, CreatedAt: now}).Error; err != nil {
			return code("database_unavailable")
		}
		out = contract.DirectoryCreation{Found: true, Status: "completed"}
		view := directoryView(row)
		out.Directory = &view
		return nil
	})
	return out, err
}

func (s *Service) lookupDirectoryHTTP(w http.ResponseWriter, r *http.Request, p Principal) error {
	q, err := contract.DecodeRequest[contract.DirectoryCreateLookup](http.MaxBytesReader(w, r.Body, 2048))
	if err != nil {
		return code("invalid_request")
	}
	if err := collectionChangeKey(q.IdempotencyKey); err != nil {
		return err
	}
	if _, err := s.space(p, q.SpaceID); err != nil {
		return err
	}
	var receipt WorkspaceRequest
	if err := s.DB.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "directory.create", q.IdempotencyKey).First(&receipt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respond(w, contract.DirectoryCreation{Status: "not_found"})
			return nil
		}
		return code("database_unavailable")
	}
	var row Directory
	if err := s.DB.Where("id = ? AND owner = ? AND space_id = ?", receipt.ResourceID, p.dataOwner(), q.SpaceID).First(&row).Error; err != nil {
		return resourceReadError(err)
	}
	view := directoryView(row)
	respond(w, contract.DirectoryCreation{Found: true, Status: "completed", Directory: &view})
	return nil
}
