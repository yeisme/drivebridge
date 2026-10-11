package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func workspaceTaskBindingOn(tx *gorm.DB, p Principal, op Operation) error {
	if op.Purpose == "" && op.WorkspaceGrantID == "" {
		return nil
	}
	if op.Purpose != "inbox_staging" || op.WorkspaceGrantID == "" {
		return code("task_source_unavailable")
	}
	var sub WorkspaceSubmission
	if err := tx.Where("upload_operation_id = ? AND owner = ? AND tenant_id = ?", op.ID, p.UserID, p.TenantID).First(&sub).Error; err != nil {
		return code("task_source_unavailable")
	}
	if op.Kind != "upload" || op.ActorID != p.UserID || op.AgentID == "" || sub.AgentID != op.AgentID || sub.GrantID != op.WorkspaceGrantID || sub.SpaceID != op.SpaceID {
		return code("task_source_unavailable")
	}
	return nil
}

func (s *Service) workspaceTaskDetail(w http.ResponseWriter, r *http.Request, p Principal) error {
	if r.PathValue("kind") != "operation" || r.PathValue("id") == "" || len(r.PathValue("id")) > 512 || len(r.URL.Query()) != 0 {
		return code("invalid_request")
	}
	var detail contract.OperationTaskDetail
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var op Operation
		err := tx.Where("id = ? AND owner = ?", r.PathValue("id"), p.UserID).First(&op).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		if err != nil {
			return code("database_unavailable")
		}
		if p.ScopeSpace != "" && p.ScopeSpace != op.SpaceID {
			return code("not_found")
		}
		var space Space
		err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", op.SpaceID, p.UserID, p.TenantID).First(&space).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		if err != nil {
			return code("database_unavailable")
		}
		if err := s.allowOn(tx, p, op.SpaceID, op.Path, "read"); err != nil {
			return err
		}
		if err := workspaceTaskBindingOn(tx, p, op); err != nil {
			return err
		}
		progress, err := operationTaskProgressOn(tx, op)
		if err != nil {
			return err
		}
		detail = contract.OperationTaskDetail{Task: operationTaskProjection(op, progress), Outputs: []contract.TaskOutputRef{}, OutputState: "not_projected"}
		detail.Task.ObservedAt = time.Now().UTC()
		if op.Kind != "upload" {
			return nil
		}
		detail.OutputState = "not_published"
		if op.Status != "completed" {
			return nil
		}
		// The original operation may remain readable after its result is removed
		// or moved. File authorization is independently checked at its current path.
		detail.OutputState = "unavailable"
		file, version, found, err := operationUploadResultOn(tx, op)
		if err != nil {
			return err
		}
		if !found || file.Trashed {
			return nil
		}
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				return nil
			}
			return err
		}
		detail.Outputs = append(detail.Outputs, contract.TaskOutputRef{FileID: file.ID, VersionID: version.ID, SpaceID: file.SpaceID})
		detail.OutputState = "available"
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, detail)
	return nil
}
