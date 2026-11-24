package service

import (
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// ReadFileRef resolves a DriveBridge file reference to the pinned file and
// version after applying the caller's read authority. It never follows the
// current head and never probes remote storage.
func (s *Service) ReadFileRef(p Principal, ref string) (File, Version, error) {
	var file File
	var version Version
	id, versionID, err := s.resolveRef(ref)
	if err != nil {
		return file, version, err
	}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if p.UserID == "" {
			return code("forbidden")
		}
		if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
			return resourceReadError(err)
		}
		if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
			return code("not_found")
		}
		if _, err := s.workspaceHomeSpaceOn(tx, p, file.SpaceID); err != nil {
			return err
		}
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND owner = ? AND file_id = ?", versionID, p.UserID, id).First(&version).Error; err != nil {
			return resourceReadError(err)
		}
		if version.Size < 0 {
			return code("dependency_unavailable")
		}
		file.VersionID = version.ID
		return nil
	})
	if err != nil {
		return File{}, Version{}, err
	}
	return file, version, nil
}

// ReadWorkspaceGrant returns the redacted, read-only grant projection for a
// human owner. Credentials and token material are never part of this DTO.
func (s *Service) ReadWorkspaceGrant(p Principal, id string) (contract.GrantView, error) {
	var out contract.GrantView
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return out, code("forbidden")
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := tx.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
			return code("forbidden")
		}
		var grant WorkspaceGrant
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&grant).Error; err != nil {
			return code("not_found")
		}
		if p.ScopeSpace != "" && p.ScopeSpace != grant.SpaceID {
			return code("not_found")
		}
		if p.DataOwner != "" && p.DataOwner != grant.Owner {
			return code("not_found")
		}
		var agent Agent
		if err := tx.Where("id = ? AND owner = ?", grant.AgentID, grant.Owner).First(&agent).Error; err != nil {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", grant.SpaceID, grant.Owner, grant.TenantID).First(&space).Error; err != nil {
			return code("not_found")
		}
		if err := s.governanceAllow(tx, p, space.ID, grant.Prefix, "read"); err != nil {
			return err
		}
		now := time.Now().UTC()
		if (grant.Protocol != "mcp" && grant.Protocol != "http") || workspaceGrantState(grant, now) == "invalid" {
			return code("forbidden")
		}
		out = workspaceGrantView(grant, agent, now)
		return nil
	})
	if err != nil {
		return contract.GrantView{}, err
	}
	return out, nil
}
