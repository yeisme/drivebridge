package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceInvocationList(w http.ResponseWriter, r *http.Request, p Principal) error {
	q, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		return code("invalid_request")
	}
	for key, values := range q {
		if len(values) != 1 {
			return code("invalid_request")
		}
		switch key {
		case "space_id", "agent_id", "result", "started_after", "started_at_or_after", "started_before", "order", "cursor", "limit":
		default:
			return code("invalid_request")
		}
	}
	for _, key := range []string{"space_id", "agent_id"} {
		if q.Has(key) && (q.Get(key) == "" || len(q.Get(key)) > 512) {
			return code("invalid_request")
		}
	}
	switch q.Get("result") {
	case "", "succeeded", "partial", "failed", "denied", "unknown":
	default:
		return code("invalid_request")
	}
	if q.Has("order") && q.Get("order") != "started_desc" {
		return code("invalid_request")
	}
	if q.Has("started_after") && q.Has("started_at_or_after") {
		return code("invalid_request")
	}
	bounds := map[string]time.Time{}
	for _, key := range []string{"started_after", "started_at_or_after", "started_before"} {
		if q.Has(key) {
			value, err := time.Parse(time.RFC3339Nano, q.Get(key))
			if err != nil {
				return code("invalid_request")
			}
			bounds[key] = value.UTC()
		}
	}
	lowerKey := "started_after"
	if q.Has("started_at_or_after") {
		lowerKey = "started_at_or_after"
	}
	after, before := bounds[lowerKey], bounds["started_before"]
	if q.Has(lowerKey) && q.Has("started_before") && !after.Before(before) {
		return code("invalid_request")
	}
	page, err := contract.ParsePageQuery(q)
	if err != nil {
		return err
	}
	result := contract.InvocationPage{Page: contract.Page[contract.InvocationView]{Items: []contract.InvocationView{}, TotalKind: "unknown", ObservedAt: time.Now().UTC()}, Coverage: "best_effort"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		query := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if p.ScopeSpace != "" {
			query = query.Where("space_id = ?", p.ScopeSpace)
		}
		for _, key := range []string{"space_id", "agent_id", "result"} {
			if q.Get(key) != "" {
				switch key {
				case "space_id":
					query = query.Where("space_id = ?", q.Get(key))
				case "agent_id":
					query = query.Where("agent_id = ?", q.Get(key))
				case "result":
					query = query.Where("result = ?", q.Get(key))
				}
			}
		}
		// Compare instants after reading the bounded candidate set; SQLite rows
		// may contain different offset representations. Never compare raw strings.
		var rows []WorkspaceInvocation
		if err := query.Order("id ASC").Limit(1001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 1000 {
			return code("query_budget_exceeded")
		}
		views := []contract.InvocationView{}
		for _, row := range rows {
			if q.Has("started_after") && !row.StartedAt.After(after) || q.Has("started_at_or_after") && row.StartedAt.Before(after) || q.Has("started_before") && !row.StartedAt.Before(before) {
				continue
			}
			var space Space
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", row.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return code("database_unavailable")
			}
			var grant WorkspaceGrant
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", row.GrantID, p.UserID, p.TenantID).First(&grant).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return code("dependency_unavailable")
				}
				return code("database_unavailable")
			}
			if grant.AgentID != row.AgentID || grant.SpaceID != row.SpaceID {
				return code("dependency_unavailable")
			}
			var agent Agent
			if err := tx.Where("id = ? AND owner = ?", row.AgentID, p.UserID).First(&agent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return code("dependency_unavailable")
				}
				return code("database_unavailable")
			}
			if agent.SpaceID != grant.SpaceID || agent.ConnectionID != grant.ConnectionID || agent.RestrictionProfile != "personal_workspace_v1" {
				return code("dependency_unavailable")
			}
			// Revoking an Agent is not revoking its human owner's historical read.
			if err := s.allowOn(tx, p, space.ID, grant.Prefix, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if (row.Protocol != "http" && row.Protocol != "mcp") || (row.Result != "unknown" && row.Result != "denied") || row.StartedAt.IsZero() || (row.FinishedAt != nil && row.FinishedAt.Before(row.StartedAt)) {
				return code("dependency_unavailable")
			}
			views = append(views, contract.InvocationView{InvocationID: row.ID, AgentID: row.AgentID, GrantID: row.GrantID, Protocol: row.Protocol, Action: row.Action, ResourceRefs: []string{}, Result: row.Result, ErrorCode: row.ErrorCode, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, RequestID: row.RequestID})
		}
		if q.Get("order") == "started_desc" {
			sort.Slice(views, func(i, j int) bool {
				if views[i].StartedAt.Equal(views[j].StartedAt) {
					return views[i].InvocationID < views[j].InvocationID
				}
				return views[i].StartedAt.After(views[j].StartedAt)
			})
		}
		raw, err := json.Marshal(views)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		// Preserve the legacy cursor domain and encoding for unchanged queries.
		domain := "workspace.invocations.v1"
		if q.Has("started_at_or_after") {
			domain = "workspace.invocations.half-open.v1"
		}
		if q.Get("order") == "started_desc" {
			domain += ".started-desc"
		}
		binding, _ := json.Marshal([]any{domain, p.TenantID, p.ScopeSpace, p.DataOwner, q.Get("space_id"), q.Get("agent_id"), q.Get("result"), q.Has(lowerKey), after, q.Has("started_before"), before, page.Limit})
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.invocations" {
				return code("cursor_expired")
			}
			found := false
			for index, view := range views {
				if view.InvocationID == cursor.LastKey {
					start = index + 1
					found = true
					break
				}
			}
			if !found {
				return code("cursor_expired")
			}
		}
		end := start + page.Limit
		if end > len(views) {
			end = len(views)
		}
		result.Items = append(result.Items, views[start:end]...)
		if end < len(views) {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.invocations", LastKey: views[end-1].InvocationID})
			if err != nil {
				return err
			}
			result.NextCursor = cursor
		}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, result)
	return nil
}
