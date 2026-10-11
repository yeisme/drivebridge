package service

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Personal organization is projected from existing Agent identities, including
// virtual revision-zero rows. No organization records are created by reads.
func (s *Service) workspaceOrganizationList(w http.ResponseWriter, r *http.Request, p Principal) error {
	values := r.URL.Query()
	page, err := contract.ParsePageQuery(values)
	if err != nil {
		return err
	}
	for key, vs := range values {
		switch key {
		case "space_id", "group_id", "ungrouped", "pinned", "cursor", "limit":
		default:
			return code("invalid_request")
		}
		if len(vs) != 1 {
			return code("invalid_request")
		}
	}
	for _, key := range []string{"ungrouped", "pinned"} {
		if vs, ok := values[key]; ok && vs[0] != "true" && vs[0] != "false" {
			return code("invalid_request")
		}
	}
	if values.Get("group_id") != "" && values.Has("ungrouped") {
		return code("invalid_request")
	}
	binding, _ := json.Marshal([]any{"workspace.organizations.v1", p.TenantID, p.ScopeSpace, p.DataOwner, values.Get("space_id"), values.Get("group_id"), values.Get("ungrouped"), values.Get("pinned"), page.Limit})
	result := contract.Page[contract.AgentOrganizationView]{Items: []contract.AgentOrganizationView{}, TotalKind: "unknown", ObservedAt: time.Now().UTC()}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var spaces []Space
		q := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if p.ScopeSpace != "" {
			q = q.Where("id = ?", p.ScopeSpace)
		}
		if values.Get("space_id") != "" {
			q = q.Where("id = ?", values.Get("space_id"))
		}
		if err := q.Find(&spaces).Error; err != nil {
			return code("database_unavailable")
		}
		ids := make([]string, 0, len(spaces))
		for _, sp := range spaces {
			ids = append(ids, sp.ID)
		}
		var agents []Agent
		if len(ids) > 0 {
			if err := tx.Where("owner = ? AND space_id IN ? AND archived = ?", p.UserID, ids, false).Find(&agents).Error; err != nil {
				return code("database_unavailable")
			}
		}
		var stored []WorkspaceAgentOrganization
		if err := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID).Find(&stored).Error; err != nil {
			return code("database_unavailable")
		}
		organizations := make(map[string]WorkspaceAgentOrganization, len(stored))
		for _, org := range stored {
			organizations[org.AgentID] = org
		}
		items := make([]contract.AgentOrganizationView, 0, len(agents))
		for _, agent := range agents {
			// Apply visibility before sorting/pagination and cursor revision hashing.
			if err := s.governanceAllow(tx, p, agent.SpaceID, agent.Prefix, "write"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			org, ok := organizations[agent.ID]
			if !ok {
				org = WorkspaceAgentOrganization{AgentID: agent.ID}
			}
			if group := values.Get("group_id"); group != "" && (org.GroupID == nil || *org.GroupID != group) {
				continue
			}
			if values.Has("ungrouped") && (org.GroupID == nil) != (values.Get("ungrouped") == "true") {
				continue
			}
			if values.Has("pinned") && org.Pinned != (values.Get("pinned") == "true") {
				continue
			}
			items = append(items, workspaceOrganizationView(org))
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Pinned != items[j].Pinned {
				return items[i].Pinned
			}
			return items[i].AgentID < items[j].AgentID
		})
		encoded, err := json.Marshal(items)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(encoded))
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil {
				return code("cursor_expired")
			}
			if cursor.Kind != "workspace.organizations" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range items {
				if item.AgentID == cursor.LastKey {
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
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, LastKey: items[end-1].AgentID, Kind: "workspace.organizations"})
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
