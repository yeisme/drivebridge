package service

import (
	"errors"

	"gorm.io/gorm"
)

// Every hold entry point resolves the current file/space, not a historical
// source path or a cached principal. Trashed bytes may still need protection.
func (s *Service) holdFileOn(tx *gorm.DB, p Principal, id, version string, human bool) (File, error) {
	var f File
	var err error
	if human {
		err = s.retentionHumanOn(tx, p)
	} else {
		err = s.retentionActorOn(tx, p)
	}
	if err != nil {
		return f, err
	}
	if err = tx.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&f).Error; err != nil {
		return f, resourceReadError(err)
	}
	if err = s.retentionFileAccessOn(tx, p, f, "read"); err != nil {
		return File{}, err
	}
	if workspaceStagingPath(f.Path) || f.ReviewPending {
		return File{}, code("not_found")
	}
	if version != "" {
		var v Version
		if err = tx.Where("id = ? AND file_id = ? AND owner = ?", version, f.ID, f.Owner).First(&v).Error; err != nil {
			return File{}, resourceReadError(err)
		}
	}
	return f, nil
}

// Source IDs are not authority: bind the exact target and current source
// space tenant, and require the action used to manage that source. Legacy
// Agents retain creation only for their own ordinary write operation.
func (s *Service) holdSourceOn(tx *gorm.DB, p Principal, f File, version, kind, ref string, active bool) (string, error) {
	if kind != "" && kind != "share" && kind != "operation" {
		return "", code("invalid_request")
	}
	if kind == "" || kind == "share" {
		var source Share
		err := tx.Where("id = ? AND owner = ?", ref, f.Owner).First(&source).Error
		if err == nil {
			if p.Agent != nil {
				return "", code("forbidden")
			}
			if source.SpaceID != f.SpaceID || !contains(source.Path, f.Path) {
				return "", code("invalid_request")
			}
			if err = s.retentionFileAccessOn(tx, p, File{SpaceID: source.SpaceID, Path: source.Path}, "manage"); err != nil {
				return "", err
			}
			if err = s.retentionFileAccessOn(tx, p, f, "manage"); err != nil {
				return "", err
			}
			if active && !shareActive(source) {
				return "", code("invalid_request")
			}
			switch source.Mode {
			case "fixed":
				var item ShareItem
				if err = tx.Where("share_id = ? AND file_id = ? AND version_id = ?", source.ID, f.ID, version).First(&item).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return "", code("invalid_request")
					}
					return "", code("database_unavailable")
				}
			case "live":
				if active && (version != f.VersionID || f.Trashed) {
					return "", code("invalid_request")
				}
			default:
				return "", code("invalid_request")
			}
			return "share", nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", code("database_unavailable")
		}
		if kind == "share" {
			return "", code("invalid_request")
		}
	}
	var op Operation
	if err := tx.Where("id = ? AND owner = ?", ref, f.Owner).First(&op).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", code("invalid_request")
		}
		return "", code("database_unavailable")
	}
	if op.SpaceID != f.SpaceID || op.FileID != f.ID || op.VersionID != version || op.Purpose != "" || op.WorkspaceGrantID != "" {
		return "", code("invalid_request")
	}
	if active && op.Status != "completed" {
		return "", code("invalid_request")
	}
	if s.governanceActive() && op.ActorID != "" && op.ActorID != p.UserID {
		return "", code("forbidden")
	}
	action := "manage"
	if p.Agent != nil {
		if op.AgentID != p.Agent.ID {
			return "", code("forbidden")
		}
		action = "write"
	}
	if err := s.retentionFileAccessOn(tx, p, File{SpaceID: op.SpaceID, Path: op.Path}, action); err != nil {
		return "", err
	}
	if err := s.retentionFileAccessOn(tx, p, f, action); err != nil {
		return "", err
	}
	return "operation", nil
}

func (s *Service) holdRowOn(tx *gorm.DB, p Principal, id string, human bool) (RetentionHold, File, error) {
	var h RetentionHold
	if human {
		if err := s.retentionHumanOn(tx, p); err != nil {
			return h, File{}, err
		}
	} else if err := s.retentionActorOn(tx, p); err != nil {
		return h, File{}, err
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.dataOwner(), p.TenantID).First(&h).Error; err != nil {
		return h, File{}, resourceReadError(err)
	}
	f, err := s.holdFileOn(tx, p, h.FileID, h.VersionID, human)
	return h, f, err
}
