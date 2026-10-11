package service

import (
	"encoding/json"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Bounds apply to the whole request, not per batch. No partial aggregate is
// published when visibility or source completeness cannot be established.
func (s *Service) workspaceBatchTasksOn(tx *gorm.DB, p Principal, spaces []string, agent, state string) ([]contract.TaskProjection, error) {
	views := []contract.TaskProjection{}
	if len(spaces) == 0 {
		return views, nil
	}
	var batches []UploadBatch
	if err := tx.Where("owner = ? AND tenant_id = ? AND space_id IN ?", p.UserID, p.TenantID, spaces).Order("id ASC").Limit(1001).Find(&batches).Error; err != nil {
		return nil, code("database_unavailable")
	}
	if len(batches) > 1000 {
		return nil, code("task_scan_budget_exceeded")
	}
	remaining := 1000
	for _, batch := range batches {
		if err := s.allowOn(tx, p, batch.SpaceID, "", "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				continue
			}
			return nil, err
		}
		var anchor Operation
		if err := tx.Where("id = ? AND owner = ?", batch.OperationID, p.UserID).First(&anchor).Error; err != nil {
			return nil, code("task_source_unavailable")
		}
		if anchor.Kind != "upload_batch" || anchor.SpaceID != batch.SpaceID || anchor.Purpose != "" || anchor.WorkspaceGrantID != "" {
			return nil, code("task_source_unavailable")
		}
		if agent != "" && anchor.AgentID != agent {
			continue
		}
		var items []UploadBatchItem
		if err := tx.Where("batch_id = ? AND owner = ?", batch.ID, p.UserID).Order("id ASC").Limit(remaining + 1).Find(&items).Error; err != nil {
			return nil, code("database_unavailable")
		}
		if len(items) > remaining {
			return nil, code("task_scan_budget_exceeded")
		}
		remaining -= len(items)
		visible := true
		for _, item := range items {
			if err := s.allowOn(tx, p, batch.SpaceID, item.RelativePath, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					visible = false
					break
				}
				return nil, err
			}
		}
		if !visible {
			continue
		}
		var succeeded, failed, pending, cancelled int64
		uncertain := false
		for _, item := range items {
			switch item.Status {
			case "completed":
				if item.Certainty != "confirmed" || item.OperationID == "" || item.IsDir {
					uncertain = true
					continue
				}
				var op Operation
				if err := tx.Where("id = ? AND owner = ?", item.OperationID, p.UserID).First(&op).Error; err != nil {
					return nil, code("task_source_unavailable")
				}
				if op.Kind != "upload" || op.BatchID != batch.ID || op.ItemKey != item.ItemKey || op.SpaceID != batch.SpaceID || op.Status != "completed" || op.Size != item.Size || op.SHA256 != item.SHA256 {
					uncertain = true
					continue
				}
				_, _, found, err := operationUploadResultOn(tx, op)
				if err != nil {
					return nil, err
				}
				if !found {
					uncertain = true
					continue
				}
				succeeded++
			case "failed":
				failed++
			case "pending", "transferring":
				pending++
			case "cancelled":
				cancelled++
			default:
				uncertain = true
			}
		}
		progress := contract.TaskProgress{Succeeded: &succeeded, Failed: &failed, Pending: &pending}
		if batch.ItemsTotal != nil {
			if *batch.ItemsTotal < 0 || *batch.ItemsTotal != int64(len(items)) {
				return nil, code("task_source_unavailable")
			}
			total := *batch.ItemsTotal
			progress.Total = &total
		}
		if uncertain {
			progress.Succeeded = nil
		}
		view := operationTaskProjection(anchor, progress)
		view.OwnerKind = "upload_batch"
		view.OwnerID = batch.ID
		view.Title = "Upload batch"
		view.OwnerStatus = batch.Phase
		view.UIState = "unknown"
		view.BlockedReason = "owner_actions_not_projected"
		view.Revision = ""
		switch batch.Phase {
		case "enumerating", "sealed", "transferring":
			view.UIState = "running"
		case "paused":
			view.UIState = "recoverable"
		case "cancelled":
			view.UIState = "cancelled"
		case "completed":
			if uncertain || progress.Total == nil || anchor.Status != "completed" {
				view.UIState = "unknown"
			} else if failed > 0 || pending > 0 || cancelled > 0 {
				view.UIState = "recoverable"
			} else if succeeded == *progress.Total {
				view.UIState = "completed"
			}
		}
		// Includes the computed public counts so progress-only changes invalidate
		// cursors even when the batch anchor was not updated.
		// Hash only the public projection and owner revision, not source digests,
		// item keys, untrusted error payloads or private request metadata.
		raw, err := json.Marshal(struct {
			View          contract.TaskProjection
			BatchRevision uint64
		}{view, batch.Revision})
		if err != nil {
			return nil, code("database_unavailable")
		}
		view.Revision = "sha256:" + Hash(string(raw))
		if state == "" || view.UIState == state {
			views = append(views, view)
		}
	}
	return views, nil
}
