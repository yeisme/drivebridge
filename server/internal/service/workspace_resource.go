package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Preserve application names while using the single public DTO definition.
type RegisterWorkspaceResourceRequest = contract.RegisterResourceRequest
type WorkspaceResourceView = contract.ResourceView

func resourceReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return code("not_found")
	}
	return code("database_unavailable")
}

func (s *Service) resourceFileOn(tx *gorm.DB, p Principal, id, version string) (File, error) {
	var file File
	if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
		return file, resourceReadError(err)
	}
	if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
		return File{}, code("not_found")
	}
	if _, err := s.workspaceHomeSpaceOn(tx, p, file.SpaceID); err != nil {
		return File{}, err
	}
	if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
		return File{}, err
	}
	var fixed Version
	if err := tx.Where("id = ? AND owner = ? AND file_id = ?", version, p.UserID, id).First(&fixed).Error; err != nil {
		return File{}, resourceReadError(err)
	}
	if fixed.Size < 0 {
		return File{}, code("dependency_unavailable")
	}
	// Preserve identity, not the latest head. No content or storage probe.
	file.VersionID = version
	return file, nil
}

func (s *Service) resourceViewOn(tx *gorm.DB, p Principal, row WorkspaceResourceDescriptor) (WorkspaceResourceView, error) {
	budget := 1000
	return s.resourceViewBudgetOn(tx, p, row, &budget)
}

func (s *Service) resourceViewBudgetOn(tx *gorm.DB, p Principal, row WorkspaceResourceDescriptor, budget *int) (WorkspaceResourceView, error) {
	view := WorkspaceResourceView{}
	if row.Owner != p.UserID || row.TenantID != p.TenantID {
		return view, code("not_found")
	}
	if _, err := s.workspaceHomeSpaceOn(tx, p, row.SpaceID); err != nil {
		return view, err
	}
	if row.Revision == 0 || row.Revision > 9007199254740991 {
		return view, code("dependency_unavailable")
	}
	if row.Kind != "skill" && row.Kind != "template" && row.Kind != "reference_pack" {
		return view, code("dependency_unavailable")
	}
	view = WorkspaceResourceView{DescriptorID: row.ID, SpaceID: row.SpaceID, Kind: row.Kind, DisplayName: row.DisplayName, Revision: row.Revision, CheckState: "unknown", CreatedAt: row.CreatedAt}
	if row.FileID != nil && row.VersionID != nil && row.SnapshotID == nil {
		if row.Kind == "reference_pack" {
			return WorkspaceResourceView{}, code("dependency_unavailable")
		}
		if *budget < 1 {
			return WorkspaceResourceView{}, code("query_budget_exceeded")
		}
		*budget--
		file, err := s.resourceFileOn(tx, p, *row.FileID, *row.VersionID)
		if err != nil {
			return WorkspaceResourceView{}, err
		}
		if file.SpaceID != row.SpaceID {
			return WorkspaceResourceView{}, code("dependency_unavailable")
		}
		view.FileRef = s.FileRef(file)
	} else if row.FileID == nil && row.VersionID == nil && row.SnapshotID != nil {
		var snapshot CollectionSnapshot
		if err := tx.Where("id = ? AND owner = ?", *row.SnapshotID, p.UserID).First(&snapshot).Error; err != nil {
			return WorkspaceResourceView{}, resourceReadError(err)
		}
		var collection FileCollection
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", snapshot.CollectionID, p.UserID, p.TenantID).First(&collection).Error; err != nil {
			return WorkspaceResourceView{}, resourceReadError(err)
		}
		if collection.SpaceID != row.SpaceID || snapshot.EntriesTotal < 0 || snapshot.EntriesVisible != snapshot.EntriesTotal {
			return WorkspaceResourceView{}, code("dependency_unavailable")
		}
		var entries []CollectionSnapshotEntry
		if err := tx.Where("snapshot_id = ?", snapshot.ID).Order("id").Limit(*budget + 1).Find(&entries).Error; err != nil {
			return WorkspaceResourceView{}, code("database_unavailable")
		}
		if len(entries) > *budget {
			return WorkspaceResourceView{}, code("query_budget_exceeded")
		}
		*budget -= len(entries)
		if int64(len(entries)) != snapshot.EntriesTotal {
			return WorkspaceResourceView{}, code("dependency_unavailable")
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if !entry.Visible || seen[entry.FileID] {
				return WorkspaceResourceView{}, code("dependency_unavailable")
			}
			seen[entry.FileID] = true
			file, err := s.resourceFileOn(tx, p, entry.FileID, entry.VersionID)
			if err != nil {
				return WorkspaceResourceView{}, err
			}
			if err := s.allowOn(tx, p, file.SpaceID, entry.Path, "read"); err != nil {
				return WorkspaceResourceView{}, err
			}
		}
		view.SnapshotID = snapshot.ID
	} else {
		return WorkspaceResourceView{}, code("dependency_unavailable")
	}
	return view, nil
}

func (s *Service) RegisterWorkspaceResource(p Principal, input RegisterWorkspaceResourceRequest, key string) (WorkspaceResourceView, error) {
	var out WorkspaceResourceView
	if len(key) == 0 || len(key) > 200 {
		return out, code("invalid_request")
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return out, code("invalid_request")
		}
	}
	if input.SpaceID == "" || len(input.SpaceID) > 512 || !utf8.ValidString(input.DisplayName) || strings.TrimSpace(input.DisplayName) == "" || utf8.RuneCountInString(input.DisplayName) > 120 {
		return out, code("invalid_request")
	}
	for _, r := range input.DisplayName {
		if unicode.IsControl(r) {
			return out, code("invalid_request")
		}
	}
	if input.Kind != "skill" && input.Kind != "template" && input.Kind != "reference_pack" {
		return out, code("invalid_request")
	}
	if (input.FileRef == "") == (input.SnapshotID == "") || len(input.FileRef) > 2048 || len(input.SnapshotID) > 512 || (input.Kind == "reference_pack" && input.SnapshotID == "") {
		return out, code("invalid_request")
	}
	row := WorkspaceResourceDescriptor{Owner: p.UserID, TenantID: p.TenantID, SpaceID: input.SpaceID, Kind: input.Kind, DisplayName: input.DisplayName, Revision: 1}
	if input.FileRef != "" {
		id, version, err := s.resolveRef(input.FileRef)
		if err != nil {
			return out, err
		}
		row.FileID = &id
		row.VersionID = &version
	} else {
		row.SnapshotID = &input.SnapshotID
	}
	raw, _ := json.Marshal(input)
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "resource.register", key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&row).Error; err != nil {
				return resourceReadError(err)
			}
			if receipt.ResultRevision != row.Revision {
				return code("revision_conflict")
			}
			out, err = s.resourceViewOn(tx, p, row)
			if err != nil {
				return err
			}
			if out.SpaceID != input.SpaceID || out.Kind != input.Kind || out.DisplayName != input.DisplayName || out.FileRef != input.FileRef || out.SnapshotID != input.SnapshotID {
				return code("idempotency_conflict")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		row.ID = ID()
		row.CreatedAt = time.Now().UTC()
		row.UpdatedAt = row.CreatedAt
		out, err = s.resourceViewOn(tx, p, row)
		if err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "resource.register", RequestKey: key, RequestDigest: digest, ResourceID: row.ID, ResultRevision: 1, CreatedAt: row.CreatedAt}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		if err := tx.Create(&Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.resource.register", Resource: row.ID, Outcome: "success", At: row.CreatedAt}).Error; err != nil {
			return code("audit_unavailable")
		}
		return nil
	})
	if err != nil {
		return WorkspaceResourceView{}, err
	}
	return out, nil
}

func (s *Service) WorkspaceResource(p Principal, id string) (WorkspaceResourceView, error) {
	var out WorkspaceResourceView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var row WorkspaceResourceDescriptor
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		var err error
		out, err = s.resourceViewOn(tx, p, row)
		return err
	})
	if err != nil {
		return WorkspaceResourceView{}, err
	}
	return out, nil
}
