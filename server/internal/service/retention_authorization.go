package service

import (
	"errors"

	"gorm.io/gorm"
)

// Retention execution is a human mutation, not an inherited read capability.
// Use the caller's transaction for live identity and credential checks.
func (s *Service) retentionHumanOn(tx *gorm.DB, p Principal) error {
	if p.Agent != nil {
		return code("forbidden")
	}
	return s.retentionActorOn(tx, p)
}

func (s *Service) retentionActorOn(tx *gorm.DB, p Principal) error {
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	if !s.governanceActive() && p.dataOwner() != p.UserID {
		return code("forbidden")
	}
	var user User
	if err := tx.First(&user, "id = ?", p.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("forbidden")
		}
		return code("database_unavailable")
	}
	if user.Disabled || user.Archived {
		return code("forbidden")
	}
	if err := s.governanceCredential(p, tx); err != nil {
		return err
	}
	if p.Agent != nil {
		var agent Agent
		if err := tx.First(&agent, "id = ? AND owner = ?", p.Agent.ID, p.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("forbidden")
			}
			return code("database_unavailable")
		}
		// Validate even an empty preview against the current Agent scope.
		return s.retentionFileAccessOn(tx, p, File{SpaceID: agent.SpaceID, Path: agent.Prefix}, "read")
	}
	return nil
}

func (s *Service) retentionFileDeleteOn(tx *gorm.DB, p Principal, f File) error {
	return s.retentionFileAccessOn(tx, p, f, "delete")
}

func (s *Service) retentionFileAccessOn(tx *gorm.DB, p Principal, f File, action string) error {
	var sp Space
	if err := tx.First(&sp, "id = ? AND owner = ?", f.SpaceID, p.dataOwner()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		return code("database_unavailable")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != sp.ID {
		return code("not_found")
	}
	if s.governanceActive() {
		if p.TenantID != "" && p.TenantID != s.spaceTenant(sp) {
			return code("not_found")
		}
	} else if p.TenantID != sp.TenantID {
		return code("not_found")
	}
	return s.allowOn(tx, p, sp.ID, f.Path, action)
}
