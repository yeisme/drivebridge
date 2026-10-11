package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSubmissionList(w http.ResponseWriter, r *http.Request, p Principal) error {
	values := r.URL.Query()
	page, err := contract.ParsePageQuery(values)
	if err != nil {
		return err
	}
	for key, vs := range values {
		switch key {
		case "space_id", "state", "agent_id", "cursor", "limit":
		default:
			return code("invalid_request")
		}
		if len(vs) != 1 {
			return code("invalid_request")
		}
	}
	switch values.Get("state") {
	case "", "uploading", "submitted", "accepted", "rejected", "archiving", "archived":
	default:
		return code("invalid_request")
	}
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	result := contract.Page[contract.SubmissionView]{Items: []contract.SubmissionView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		agentID := ""
		if p.Agent != nil {
			grant, err := s.authorizeWorkspaceActionOn(tx, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, "inbox.submit", result.ObservedAt)
			if err != nil {
				return err
			}
			agentID = grant.AgentID
			if values.Get("agent_id") != "" && values.Get("agent_id") != agentID {
				return code("forbidden")
			}
		} else if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		q := tx.Model(&WorkspaceSubmission{}).Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if agentID != "" {
			q = q.Where("agent_id = ?", agentID)
		}
		if p.ScopeSpace != "" {
			q = q.Where("space_id = ?", p.ScopeSpace)
		}
		if value := values.Get("space_id"); value != "" {
			q = q.Where("space_id = ?", value)
		}
		if value := values.Get("state"); value != "" {
			q = q.Where("state = ?", value)
		}
		if value := values.Get("agent_id"); value != "" {
			q = q.Where("agent_id = ?", value)
		}
		var ids []string
		if err := q.Order("id ASC").Pluck("id", &ids).Error; err != nil {
			return code("database_unavailable")
		}
		items := []contract.SubmissionView{}
		for _, id := range ids {
			view, err := s.workspaceSubmissionViewOn(tx, p, id)
			if err != nil {
				if safeCode(err) == "not_found" || safeCode(err) == "forbidden" || safeCode(err) == "approval_required" {
					continue
				}
				return err
			}
			items = append(items, view)
		}
		binding, _ := json.Marshal([]any{"workspace.submissions.v1", p.TenantID, p.ScopeSpace, p.DataOwner, agentID, values.Get("space_id"), values.Get("state"), values.Get("agent_id"), page.Limit})
		raw, err := json.Marshal(items)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.submissions" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range items {
				if item.SubmissionID == cursor.LastKey {
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
		result.Items = append(result.Items, items[start:end]...)
		if end < len(items) {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.submissions", LastKey: items[end-1].SubmissionID})
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
