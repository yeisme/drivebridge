package service

import (
	"errors"
	"gorm.io/gorm"
)

// Collection metadata is visible only inside the caller's active owner,
// tenant and space scope, including when the collection has no members.
func (s *Service) collectionReadSpace(p Principal, id string) error {
	return s.collectionReadSpaceOn(s.DB, p, id)
}

func (s *Service) collectionReadSpaceOn(tx *gorm.DB, p Principal, id string) error {
	if p.ScopeSpace != "" && p.ScopeSpace != id {
		return code("not_found")
	}
	var space Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.dataOwner(), p.TenantID).First(&space).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		return code("database_unavailable")
	}
	return nil
}

func (s *Service) collectionReadAdmission(p Principal, id string) (FileCollection, error) {
	return s.collectionReadAdmissionOn(s.DB, p, id)
}

func (s *Service) collectionReadAdmissionOn(tx *gorm.DB, p Principal, id string) (FileCollection, error) {
	var collection FileCollection
	if p.UserID == "" {
		return collection, code("unauthorized")
	}
	var user User
	if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return collection, code("forbidden")
		}
		return collection, code("database_unavailable")
	}
	if user.Disabled || user.Archived {
		return collection, code("forbidden")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.dataOwner(), p.TenantID).First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return collection, code("not_found")
		}
		return collection, code("database_unavailable")
	}
	if err := s.collectionReadSpaceOn(tx, p, collection.SpaceID); err != nil {
		return collection, err
	}
	prefix := ""
	if p.Agent != nil {
		var current Agent
		if err := tx.Where("id = ? AND owner = ?", p.Agent.ID, p.UserID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return collection, code("forbidden")
			}
			return collection, code("database_unavailable")
		}
		// Do not demand access to the space root from a valid prefix-scoped
		// reader. The ordinary allow path still checks current grant validity,
		// and each result is independently checked at its actual file path.
		if current.SpaceID != collection.SpaceID {
			return collection, code("forbidden")
		}
		prefix = current.Prefix
	}
	return collection, s.allowOn(tx, p, collection.SpaceID, prefix, "read")
}
