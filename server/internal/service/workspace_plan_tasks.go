package service

import (
	"encoding/json"
	"errors"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Organization plans do not persist a trustworthy creator identity. Human
// confirmation is not creator attribution, nor proof that execution succeeded.
func (s *Service) workspaceOrganizationTasksOn(tx *gorm.DB, p Principal, spaces []string, agent, state string) ([]contract.TaskProjection, error) {
	views := []contract.TaskProjection{}
	if len(spaces) == 0 || agent != "" {
		return views, nil
	}
	var plans []OrganizationPlan
	if err := tx.Where("owner = ? AND tenant_id = ? AND space_id IN ?", p.UserID, p.TenantID, spaces).Order("id ASC").Limit(1001).Find(&plans).Error; err != nil {
		return nil, code("database_unavailable")
	}
	if len(plans) > 1000 {
		return nil, code("task_scan_budget_exceeded")
	}
	remaining := 1000
	for _, plan := range plans {
		if err := s.allowOn(tx, p, plan.SpaceID, "", "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				continue
			}
			return nil, err
		}
		var items []OrganizationPlanItem
		if err := tx.Where("plan_id = ?", plan.ID).Order("id ASC").Limit(remaining + 1).Find(&items).Error; err != nil {
			return nil, code("database_unavailable")
		}
		if len(items) > remaining {
			return nil, code("task_scan_budget_exceeded")
		}
		remaining -= len(items)
		visible := true
		for _, item := range items {
			if item.Owner != p.UserID {
				return nil, code("task_source_unavailable")
			}
			// Frozen source paths remain sensitive even after a file has moved.
			if err := s.allowOn(tx, p, plan.SpaceID, item.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					visible = false
					break
				}
				return nil, err
			}
			var file File
			if err := tx.Where("id = ? AND owner = ?", item.FileID, p.UserID).First(&file).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					visible = false
					break
				}
				return nil, code("database_unavailable")
			}
			// A plan can historically contain sources from more than one space.
			// Never infer authorization to those sources from its first space alone.
			var space Space
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					visible = false
					break
				}
				return nil, code("database_unavailable")
			}
			if p.ScopeSpace != "" && p.ScopeSpace != file.SpaceID {
				visible = false
				break
			}
			if file.PurgeStartedAt != nil {
				visible = false
				break
			}
			for _, path := range []string{item.Path, file.Path} {
				if err := s.allowOn(tx, p, file.SpaceID, path, "read"); err != nil {
					if workspaceVisibilityDenied(err) {
						visible = false
						break
					}
					return nil, err
				}
			}
			if !visible {
				break
			}
		}
		if !visible {
			continue
		}
		view := contract.TaskProjection{OwnerKind: "organization_plan", OwnerID: plan.ID, SpaceID: plan.SpaceID, Title: "Organization plan", ActorKind: "unknown", OwnerStatus: plan.Phase, UIState: "unknown", AllowedActions: []string{}, BlockedReason: "organization_result_unproven"}
		switch plan.Phase {
		case "previewed":
			if plan.Risk == "approval_required" {
				view.UIState = "waiting_confirmation"
			}
		case "confirmed", "executing":
			view.UIState = "running"
		case "partial":
			view.UIState = "recoverable"
		case "cancelled":
			view.UIState = "cancelled"
		}
		// Counts remain unknown: ItemsTotal counts files while item rows count
		// actions, and a done row is not independent proof of the action's effect.
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
