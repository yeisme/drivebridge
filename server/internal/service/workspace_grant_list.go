package service

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceGrantList(w http.ResponseWriter, r *http.Request, p Principal) error {
	values := r.URL.Query()
	page, err := contract.ParsePageQuery(values)
	if err != nil {
		return err
	}
	for key, vs := range values {
		switch key {
		case "space_id", "state", "q", "cursor", "limit", "group_id", "ungrouped", "pinned", "order":
		default:
			return code("invalid_request")
		}
		if len(vs) != 1 {
			return code("invalid_request")
		}
	}
	for _, key := range []string{"ungrouped", "pinned"} {
		if values.Has(key) && values.Get(key) != "true" && values.Get(key) != "false" {
			return code("invalid_request")
		}
	}
	if values.Has("group_id") && (values.Get("group_id") == "" || len(values.Get("group_id")) > 512 || values.Has("ungrouped")) {
		return code("invalid_request")
	}
	if values.Has("order") && values.Get("order") != "pinned" {
		return code("invalid_request")
	}
	organized := values.Has("group_id") || values.Has("ungrouped") || values.Has("pinned") || values.Has("order")
	state := values.Get("state")
	switch state {
	case "", "active", "disabled", "expired", "revoked":
	default:
		return code("invalid_request")
	}
	search := strings.ToLower(strings.TrimSpace(values.Get("q")))
	if len(search) > 480 {
		return code("invalid_request")
	}
	bindingValues := []any{"workspace.grants.v1", p.TenantID, p.ScopeSpace, p.DataOwner, values.Get("space_id"), state, search, page.Limit}
	if organized {
		bindingValues = append(bindingValues, "organization.v1", values.Get("group_id"), values.Get("ungrouped"), values.Get("pinned"), values.Get("order"))
	}
	binding, _ := json.Marshal(bindingValues)
	now := time.Now().UTC()
	result := contract.Page[contract.GrantView]{Items: []contract.GrantView{}, ObservedAt: now, TotalKind: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var spaces []Space
		sq := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if p.ScopeSpace != "" {
			sq = sq.Where("id = ?", p.ScopeSpace)
		}
		if values.Get("space_id") != "" {
			sq = sq.Where("id = ?", values.Get("space_id"))
		}
		if err := sq.Find(&spaces).Error; err != nil {
			return code("database_unavailable")
		}
		ids := make([]string, 0, len(spaces))
		for _, sp := range spaces {
			ids = append(ids, sp.ID)
		}
		var grants []WorkspaceGrant
		if len(ids) > 0 {
			if err := tx.Where("owner = ? AND tenant_id = ? AND space_id IN ?", p.UserID, p.TenantID, ids).Order("id ASC").Find(&grants).Error; err != nil {
				return code("database_unavailable")
			}
		}
		agentIDs := make([]string, 0, len(grants))
		for _, g := range grants {
			agentIDs = append(agentIDs, g.AgentID)
		}
		var agents []Agent
		if len(agentIDs) > 0 {
			if err := tx.Where("owner = ? AND id IN ?", p.UserID, agentIDs).Find(&agents).Error; err != nil {
				return code("database_unavailable")
			}
		}
		byID := make(map[string]Agent, len(agents))
		for _, a := range agents {
			byID[a.ID] = a
		}
		organizations := map[string]WorkspaceAgentOrganization{}
		if organized && len(agentIDs) > 0 {
			var rows []WorkspaceAgentOrganization
			if err := tx.Where("owner = ? AND tenant_id = ? AND agent_id IN ?", p.UserID, p.TenantID, agentIDs).Find(&rows).Error; err != nil {
				return code("database_unavailable")
			}
			for _, row := range rows {
				organizations[row.AgentID] = row
			}
		}
		items := []contract.GrantView{}
		visibleOrganizations := []contract.AgentOrganizationView{}
		for _, g := range grants {
			a, ok := byID[g.AgentID]
			if !ok {
				continue
			}
			current := workspaceGrantState(g, now)
			if current == "invalid" || (g.Protocol != "http" && g.Protocol != "mcp") {
				continue
			}
			if state != "" && state != current {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(a.Name), search) {
				continue
			}
			if err := s.governanceAllow(tx, p, g.SpaceID, g.Prefix, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if organized {
				// Explicit organization projections require the same access as
				// /agent-organizations; default grant listing keeps its old policy.
				if a.Archived {
					continue
				}
				if err := s.governanceAllow(tx, p, g.SpaceID, g.Prefix, "write"); err != nil {
					if workspaceVisibilityDenied(err) {
						continue
					}
					return err
				}
				org := organizations[a.ID]
				org.AgentID = a.ID
				if values.Has("group_id") && (org.GroupID == nil || *org.GroupID != values.Get("group_id")) {
					continue
				}
				if values.Has("ungrouped") && (org.GroupID == nil) != (values.Get("ungrouped") == "true") {
					continue
				}
				if values.Has("pinned") && org.Pinned != (values.Get("pinned") == "true") {
					continue
				}
				visibleOrganizations = append(visibleOrganizations, workspaceOrganizationView(org))
			}
			view := workspaceGrantView(g, a, now)
			if organized {
				organization := visibleOrganizations[len(visibleOrganizations)-1]
				view.Organization = &organization
			}
			items = append(items, view)
		}
		if values.Get("order") == "pinned" {
			sort.SliceStable(items, func(i, j int) bool {
				left, right := organizations[items[i].AgentID].Pinned, organizations[items[j].AgentID].Pinned
				if left != right {
					return left
				}
				return items[i].GrantID < items[j].GrantID
			})
		}
		var generationInput any = items
		if organized {
			generationInput = []any{items, visibleOrganizations}
		}
		raw, err := json.Marshal(generationInput)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.grants" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range items {
				if item.GrantID == cursor.LastKey {
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
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.grants", LastKey: items[end-1].GrantID})
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
