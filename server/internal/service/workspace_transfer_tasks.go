package service

import (
	"encoding/json"
	"errors"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceTransferTasksOn(tx *gorm.DB, p Principal, spaces []string, agent, state string) ([]contract.TaskProjection, error) {
	if err := s.transferGate(); err != nil {
		return nil, err
	}
	views := []contract.TaskProjection{}
	// An execution anchor identifies its executor, not the plan's creator.
	if len(spaces) == 0 || agent != "" {
		return views, nil
	}
	var plans []TransferPlan
	if err := tx.Where("owner = ? AND tenant_id = ? AND target_space_id IN ?", p.UserID, p.TenantID, spaces).Order("id ASC").Limit(1001).Find(&plans).Error; err != nil {
		return nil, code("database_unavailable")
	}
	if len(plans) > 1000 {
		return nil, code("task_scan_budget_exceeded")
	}
	remaining := 1000
	for _, plan := range plans {
		if err := s.allowOn(tx, p, plan.TargetSpaceID, plan.TargetPath, "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				continue
			}
			return nil, err
		}
		if plan.Mode != "copy" && plan.Mode != "migrate" {
			return nil, code("task_source_unavailable")
		}
		if plan.OperationID != "" {
			var anchor Operation
			if err := tx.Where("id = ? AND owner = ?", plan.OperationID, p.UserID).First(&anchor).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, code("task_source_unavailable")
				}
				return nil, code("database_unavailable")
			}
			if anchor.Kind != "transfer_plan" || anchor.SpaceID != plan.TargetSpaceID || anchor.Path != plan.TargetPath || anchor.Purpose != "" || anchor.WorkspaceGrantID != "" {
				return nil, code("task_source_unavailable")
			}
		}
		var items []TransferItem
		if err := tx.Where("plan_id = ?", plan.ID).Order("id ASC").Limit(remaining + 1).Find(&items).Error; err != nil {
			return nil, code("database_unavailable")
		}
		if len(items) > remaining {
			return nil, code("task_scan_budget_exceeded")
		}
		remaining -= len(items)
		if plan.ItemsTotal < 0 || plan.ItemsTotal != int64(len(items)) {
			return nil, code("task_source_unavailable")
		}
		visible := true
		for _, item := range items {
			if item.Owner != p.UserID {
				return nil, code("task_source_unavailable")
			}
			if err := s.allowOn(tx, p, plan.TargetSpaceID, item.TargetPath, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					visible = false
					break
				}
				return nil, err
			}
			refs := []struct{ file, version string }{{item.SourceFileID, item.SourceVersionID}}
			if item.TargetFileID != "" || item.TargetVersionID != "" {
				refs = append(refs, struct{ file, version string }{item.TargetFileID, item.TargetVersionID})
			}
			for index, ref := range refs {
				if ref.file == "" || ref.version == "" {
					return nil, code("task_source_unavailable")
				}
				var file File
				if err := tx.Where("id = ? AND owner = ?", ref.file, p.UserID).First(&file).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						visible = false
						break
					}
					return nil, code("database_unavailable")
				}
				if index == 1 && file.SpaceID != plan.TargetSpaceID {
					return nil, code("task_source_unavailable")
				}
				var space Space
				if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						visible = false
						break
					}
					return nil, code("database_unavailable")
				}
				if (p.ScopeSpace != "" && p.ScopeSpace != file.SpaceID) || file.PurgeStartedAt != nil {
					visible = false
					break
				}
				if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
					if workspaceVisibilityDenied(err) {
						visible = false
						break
					}
					return nil, err
				}
				var version Version
				if err := tx.Where("id = ? AND file_id = ? AND owner = ?", ref.version, file.ID, p.UserID).First(&version).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						visible = false
						break
					}
					return nil, code("database_unavailable")
				}
			}
			if !visible {
				break
			}
		}
		if !visible {
			continue
		}
		view := contract.TaskProjection{OwnerKind: "transfer_plan", OwnerID: plan.ID, SpaceID: plan.TargetSpaceID, Title: "Transfer plan: " + plan.Mode, ActorKind: "unknown", OwnerStatus: plan.Phase, UIState: "unknown", AllowedActions: []string{}, BlockedReason: "transfer_result_unproven"}
		switch plan.Phase {
		case "planned":
			view.UIState = "waiting_confirmation"
		case "executing":
			view.UIState = "running"
		case "partial":
			view.UIState = "recoverable"
		case "source_cleanup_pending":
			view.UIState = "waiting_confirmation"
			view.BlockedReason = "source_cleanup_pending"
		case "cancelled":
			view.UIState = "cancelled"
		}
		// Copied is not migration completion; neither plan phase nor anchor status
		// proves all effects. Keep progress null until independent result proof.
		raw, err := json.Marshal(struct {
			View     contract.TaskProjection
			Revision uint64
		}{view, plan.Revision})
		if err != nil {
			return nil, code("database_unavailable")
		}
		view.Revision = "sha256:" + Hash(string(raw))
		if state == "" || state == view.UIState {
			views = append(views, view)
		}
	}
	return views, nil
}
