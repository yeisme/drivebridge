package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Intent.ActorID records the principal user, including Agent-on-behalf-of
// requests. Without a credential-kind fact it cannot identify a human creator.
func (s *Service) workspaceAutomationTasksOn(tx *gorm.DB, p Principal, spaces []string, agent, state string) ([]contract.TaskProjection, error) {
	views := []contract.TaskProjection{}
	if len(spaces) == 0 || agent != "" {
		return views, nil
	}
	allowed := map[string]bool{}
	for _, id := range spaces {
		allowed[id] = true
	}
	var intents []AutomationIntent
	if err := tx.Where("owner = ?", p.UserID).Order("id ASC").Limit(1001).Find(&intents).Error; err != nil {
		return nil, code("database_unavailable")
	}
	if len(intents) > 1000 {
		return nil, code("task_scan_budget_exceeded")
	}
	for _, intent := range intents {
		var file File
		if err := tx.Where("id = ? AND owner = ?", intent.FileID, p.UserID).First(&file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, code("database_unavailable")
		}
		if !allowed[file.SpaceID] || file.PurgeStartedAt != nil {
			continue
		}
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				continue
			}
			return nil, err
		}
		if err := s.allowOn(tx, p, file.SpaceID, intent.Scope, "read"); err != nil {
			if workspaceVisibilityDenied(err) {
				continue
			}
			return nil, err
		}
		var rule AutomationRule
		if err := tx.Where("id = ? AND owner = ?", intent.RuleID, p.UserID).First(&rule).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, code("task_source_unavailable")
			}
			return nil, code("database_unavailable")
		}
		if rule.SpaceID != file.SpaceID || intent.RuleRevision == 0 || rule.Revision < intent.RuleRevision || intent.ActorID == "" {
			return nil, code("task_source_unavailable")
		}
		// A changed rule is not the historical payload. Only compare its scope
		// at the same revision; do not turn a later pause or edit into intent state.
		if rule.Revision == intent.RuleRevision && rule.Prefix != intent.Scope {
			return nil, code("task_source_unavailable")
		}
		var event OutboxEvent
		if err := tx.Where("event_id = ? AND owner_scope = ?", intent.EventID, p.UserID).First(&event).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, code("task_source_unavailable")
			}
			return nil, code("database_unavailable")
		}
		if event.ResourceID != file.ID || event.Kind != "file_catalogue.upsert" || event.Tombstone {
			return nil, code("task_source_unavailable")
		}
		view := contract.TaskProjection{OwnerKind: "automation_intent", OwnerID: intent.ID, SpaceID: file.SpaceID, Title: "Automation intent", ActorKind: "unknown", OwnerStatus: intent.Status, UIState: "unknown", AllowedActions: []string{}, BlockedReason: "automation_result_unproven"}
		switch intent.Status {
		case "pending":
			view.UIState = "running"
		case "paused", "failed":
			view.UIState = "recoverable"
		case "cancelled":
			view.UIState = "cancelled"
		}
		// Completed status is an owner observation, not independent proof of the
		// rule's historical effects. No labels, raw payloads or operation links leak.
		raw, err := json.Marshal(struct {
			View    contract.TaskProjection
			Updated time.Time
		}{view, intent.UpdatedAt})
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
