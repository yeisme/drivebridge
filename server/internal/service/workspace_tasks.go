package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Only a human's own operations are admitted in this initial owner adapter.
// Other owner kinds and Agent credential routes remain unavailable.
func (s *Service) workspaceTasks(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	for key, values := range q {
		if len(values) != 1 {
			return code("invalid_request")
		}
		switch key {
		case "kind", "space_id", "agent_id", "state", "cursor", "limit":
		default:
			return code("invalid_request")
		}
	}
	if q.Get("kind") != "operation" && q.Get("kind") != "upload_batch" && q.Get("kind") != "organization_plan" && q.Get("kind") != "transfer_plan" && q.Get("kind") != "automation_intent" {
		return code("invalid_request")
	}
	for _, key := range []string{"space_id", "agent_id"} {
		if q.Has(key) && (q.Get(key) == "" || len(q.Get(key)) > 512) {
			return code("invalid_request")
		}
	}
	switch q.Get("state") {
	case "", "waiting_confirmation", "running", "recoverable", "completed", "failed", "cancelled", "unknown":
	default:
		return code("invalid_request")
	}
	page, err := contract.ParsePageQuery(q)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result := contract.Page[contract.TaskProjection]{Items: []contract.TaskProjection{}, ObservedAt: now, TotalKind: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var spaces []Space
		sq := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if p.ScopeSpace != "" {
			sq = sq.Where("id = ?", p.ScopeSpace)
		}
		if q.Has("space_id") {
			sq = sq.Where("id = ?", q.Get("space_id"))
		}
		if err := sq.Find(&spaces).Error; err != nil {
			return code("database_unavailable")
		}
		ids := make([]string, 0, len(spaces))
		for _, space := range spaces {
			ids = append(ids, space.ID)
		}
		var operations []Operation
		if len(ids) > 0 && q.Get("kind") == "operation" {
			query := tx.Where("owner = ? AND space_id IN ?", p.UserID, ids)
			if q.Has("agent_id") {
				query = query.Where("agent_id = ?", q.Get("agent_id"))
			}
			if err := query.Order("id ASC").Limit(1001).Find(&operations).Error; err != nil {
				return code("database_unavailable")
			}
			if len(operations) > 1000 {
				return code("task_scan_budget_exceeded")
			}
		}
		items := []contract.TaskProjection{}
		for _, op := range operations {
			if err := s.allowOn(tx, p, op.SpaceID, op.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			// Dedicated submission facts are checked against their own owner binding;
			// a staging operation alone does not prove a valid submitted resource.
			if err := workspaceTaskBindingOn(tx, p, op); err != nil {
				return err
			}
			progress, err := operationTaskProgressOn(tx, op)
			if err != nil {
				return err
			}
			view := operationTaskProjection(op, progress)
			if q.Get("state") != "" && q.Get("state") != view.UIState {
				continue
			}
			items = append(items, view)
		}
		if q.Get("kind") == "upload_batch" {
			items, err = s.workspaceBatchTasksOn(tx, p, ids, q.Get("agent_id"), q.Get("state"))
			if err != nil {
				return err
			}
		}
		if q.Get("kind") == "organization_plan" {
			items, err = s.workspaceOrganizationTasksOn(tx, p, ids, q.Get("agent_id"), q.Get("state"))
			if err != nil {
				return err
			}
		}
		if q.Get("kind") == "transfer_plan" {
			items, err = s.workspaceTransferTasksOn(tx, p, ids, q.Get("agent_id"), q.Get("state"))
			if err != nil {
				return err
			}
		}
		if q.Get("kind") == "automation_intent" {
			items, err = s.workspaceAutomationTasksOn(tx, p, ids, q.Get("agent_id"), q.Get("state"))
			if err != nil {
				return err
			}
		}
		// ObservedAt is intentionally absent from these hashed facts. Re-reading
		// an unchanged source must not expire every cursor just because time passed.
		raw, err := json.Marshal(items)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		binding, _ := json.Marshal([]any{"workspace.tasks." + q.Get("kind") + ".v1", p.TenantID, p.ScopeSpace, p.DataOwner, q.Get("space_id"), q.Get("agent_id"), q.Get("state"), page.Limit})
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.tasks" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range items {
				if item.OwnerID == cursor.LastKey {
					start = i + 1
					found = true
					break
				}
			}
			if !found {
				return code("cursor_expired")
			}
		}
		end := start + page.Limit
		if end > len(items) {
			end = len(items)
		}
		for _, item := range items[start:end] {
			item.ObservedAt = now
			result.Items = append(result.Items, item)
		}
		if end < len(items) {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.tasks", LastKey: items[end-1].OwnerID})
			if err != nil {
				return code("database_unavailable")
			}
			result.NextCursor = cursor
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, result)
	return nil
}

func operationTaskProjection(op Operation, progress contract.TaskProgress) contract.TaskProjection {
	view := contract.TaskProjection{Progress: progress, OwnerKind: "operation", OwnerID: op.ID, SpaceID: op.SpaceID, Title: op.Path, ActorKind: "unknown", OwnerStatus: op.Status, UIState: "unknown", AllowedActions: []string{}, BlockedReason: "owner_actions_not_projected"}
	if op.AgentID != "" {
		agent := op.AgentID
		view.AgentID = &agent
		view.ActorKind = "agent"
	} else if op.ActorID != "" {
		view.ActorKind = "human"
	}
	if op.Purpose == "inbox_staging" {
		view.Title = "Agent submission"
	}
	switch op.Status {
	case "completed":
		view.UIState = "completed"
	case "cancelled":
		view.UIState = "cancelled"
	case "failed", "interrupted", "paused", "awaiting_upload":
		view.UIState = "recoverable"
	case "running", "queued", "pending", "transferring", "verifying":
		view.UIState = "running"
	}
	if op.Kind == "upload_batch" && op.Status == "completed" {
		// The legacy anchor means processing stopped, not all items succeeded.
		view.UIState = "unknown"
		view.BlockedReason = "batch_projection_required"
	}
	if op.Kind == "transfer_plan" && op.Status == "completed" {
		view.UIState = "unknown"
		view.BlockedReason = "transfer_projection_required"
	}
	// Transferred is a source byte counter, not necessarily a network send or
	// durable/read-back fact (e.g. blob reuse and server-side copies). Leave the
	// three byte meanings unknown until an owner-specific proof is available.
	// The token covers public facts plus source update time, never private keys,
	// fingerprints, caller-supplied task refs or errors.
	revision, _ := json.Marshal(struct {
		View    contract.TaskProjection
		Updated time.Time
	}{view, op.Updated})
	view.Revision = "sha256:" + Hash(string(revision))
	return view
}
