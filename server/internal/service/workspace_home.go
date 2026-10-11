package service

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceHomeSpaceOn(tx *gorm.DB, p Principal, id string) (Space, error) {
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return Space{}, err
	}
	if p.ScopeSpace != "" && p.ScopeSpace != id {
		return Space{}, code("not_found")
	}
	var space Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&space).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return space, code("not_found")
		}
		return space, code("database_unavailable")
	}
	return space, s.allowOn(tx, p, id, "", "read")
}

func (s *Service) workspaceHome(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	for key, values := range q {
		if len(values) != 1 || (key != "space_id" && key != "recent_limit") {
			return code("invalid_request")
		}
	}
	id := q.Get("space_id")
	if id == "" || len(id) > 512 {
		return code("invalid_request")
	}
	limit := 6
	if q.Has("recent_limit") {
		n, err := strconv.Atoi(q.Get("recent_limit"))
		if err != nil || n < 1 || n > 20 {
			return code("invalid_request")
		}
		limit = n
	}
	result := contract.WorkspaceHome{SpaceID: id, RecentFiles: []contract.HomeFileRef{}, Collections: []contract.HomeCollection{}}
	// Each section has its own read transaction: a failed table does not poison
	// the remaining sections on PostgreSQL. None writes domain state or probes storage.
	run := func(section *contract.HomeSection, read func(*gorm.DB, Space) error) error {
		section.State = "ready"
		section.ObservedAt = time.Now().UTC()
		var authErr error
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			space, err := s.workspaceHomeSpaceOn(tx, p, id)
			if err != nil {
				authErr = err
				return err
			}
			return read(tx, space)
		})
		if authErr != nil {
			return authErr
		}
		if err != nil {
			section.State = "unavailable"
			section.Reason = "source_unavailable"
			if safeCode(err) == "query_budget_exceeded" {
				section.Reason = "scan_budget_exceeded"
			}
		}
		return nil
	}
	if err := run(&result.Sections.RecentFiles, func(tx *gorm.DB, space Space) error {
		var files []File
		if err := tx.Where("owner = ? AND space_id = ? AND trashed = ? AND purge_started_at IS NULL", p.UserID, id, false).Order("updated DESC, id ASC").Limit(1001).Find(&files).Error; err != nil {
			return err
		}
		// File.Updated uses the persisted column name `updated`, not updated_at.
		if len(files) > 1000 {
			return code("query_budget_exceeded")
		}
		visible := []contract.HomeFileRef{}
		for _, file := range files {
			if workspaceStagingPath(file.Path) {
				continue
			}
			if err := s.allowOn(tx, p, id, file.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			var version Version
			if err := tx.Where("id = ? AND owner = ? AND file_id = ?", file.VersionID, p.UserID, file.ID).First(&version).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					result.Sections.RecentFiles.State = "partial"
					result.Sections.RecentFiles.Reason = "fixed_version_unavailable"
					continue
				}
				return err
			}
			if version.Size < 0 || version.Size != file.Size {
				result.Sections.RecentFiles.State = "partial"
				result.Sections.RecentFiles.Reason = "fixed_version_unavailable"
				continue
			}
			visible = append(visible, contract.HomeFileRef{FileID: file.ID, VersionID: version.ID, Ref: s.FileRef(file), SpaceID: id, Path: file.Path, MIMEType: version.MIME, SizeBytes: version.Size, UpdatedAt: file.Updated})
			if len(visible) == limit {
				break
			}
		}
		result.RecentFiles = visible
		return nil
	}); err != nil {
		return err
	}
	if err := run(&result.Sections.Collections, func(tx *gorm.DB, space Space) error {
		var collections []FileCollection
		if err := tx.Where("owner = ? AND tenant_id = ? AND space_id = ?", p.UserID, p.TenantID, id).Order("updated_at DESC, id ASC").Limit(21).Find(&collections).Error; err != nil {
			return err
		}
		if len(collections) > 20 {
			collections = collections[:20]
			result.Sections.Collections.State = "partial"
			result.Sections.Collections.Reason = "collection_limit"
		}
		views := []contract.HomeCollection{}
		for _, collection := range collections {
			if collection.Kind != "manual" && collection.Kind != "rule" {
				return code("invalid_operation")
			}
			views = append(views, contract.HomeCollection{CollectionID: collection.ID, Name: collection.Name, Kind: collection.Kind, Revision: collection.Revision})
		}
		result.Collections = views
		return nil
	}); err != nil {
		return err
	}
	if err := run(&result.Sections.PendingCounts, func(tx *gorm.DB, space Space) error {
		var submissions []WorkspaceSubmission
		if err := tx.Where("owner = ? AND tenant_id = ? AND space_id = ? AND state IN ?", p.UserID, p.TenantID, id, []string{"submitted", "accepted"}).Order("id").Limit(1001).Find(&submissions).Error; err != nil {
			return err
		}
		if len(submissions) > 1000 {
			return code("query_budget_exceeded")
		}
		var submitted, accepted int64
		for _, sub := range submissions {
			var op Operation
			if err := tx.Where("id = ? AND owner = ?", sub.UploadOperationID, p.UserID).First(&op).Error; err != nil {
				return err
			}
			if op.SpaceID != id || op.WorkspaceGrantID != sub.GrantID || op.Purpose != "inbox_staging" || op.AgentID != sub.AgentID {
				return code("invalid_operation")
			}
			if err := s.allowOn(tx, p, id, op.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if sub.State == "submitted" {
				submitted++
			} else {
				accepted++
			}
		}
		result.PendingCounts = contract.HomePending{Submitted: &submitted, Accepted: &accepted}
		result.Sections.PendingCounts.State = "partial"
		result.Sections.PendingCounts.Reason = "submission_states_only"
		return nil
	}); err != nil {
		return err
	}
	if err := run(&result.Sections.Usage, func(tx *gorm.DB, space Space) error {
		if space.Used < 0 || space.Reserved < 0 || space.MaxBytes < 0 {
			return code("invalid_operation")
		}
		result.Usage = contract.HomeUsage{UsedBytes: &space.Used, ReservedBytes: &space.Reserved, QuotaBytes: &space.MaxBytes}
		return nil
	}); err != nil {
		return err
	}
	// A failed transaction must never leave staged section values in the response.
	if result.Sections.RecentFiles.State == "unavailable" {
		result.RecentFiles = []contract.HomeFileRef{}
	}
	if result.Sections.Collections.State == "unavailable" {
		result.Collections = []contract.HomeCollection{}
	}
	if result.Sections.PendingCounts.State == "unavailable" {
		result.PendingCounts = contract.HomePending{}
	}
	if result.Sections.Usage.State == "unavailable" {
		result.Usage = contract.HomeUsage{}
	}
	// Recheck human/space admission before returning independent observations;
	// this does not claim an atomic cross-section or future-authorization snapshot.
	if err := s.DB.Transaction(func(tx *gorm.DB) error { _, err := s.workspaceHomeSpaceOn(tx, p, id); return err }); err != nil {
		return err
	}
	result.ObservedAt = time.Now().UTC()
	respond(w, result)
	return nil
}
