package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceGroupList(w http.ResponseWriter, r *http.Request, p Principal) error {
	query, err := contract.ParsePageQuery(r.URL.Query())
	if err != nil {
		return err
	}
	// Keyset pages refuse continuation after the scoped group generation changes.
	binding, _ := json.Marshal([]any{"workspace.groups.v1", p.TenantID, p.ScopeSpace, p.DataOwner, query.Limit})
	canonical := string(binding)
	result := contract.Page[contract.AgentGroupView]{Items: []contract.AgentGroupView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var revisions []struct {
			ID       string
			Revision uint64
		}
		if err := tx.Model(&WorkspaceAgentGroup{}).Select("id", "revision").Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID).Order("id ASC").Find(&revisions).Error; err != nil {
			return code("database_unavailable")
		}
		encoded, err := json.Marshal(revisions)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(encoded))
		last := ""
		if query.Cursor != "" {
			cursor, err := s.openOrgCursor(query.Cursor, p.UserID, canonical, generation)
			if err != nil {
				return code("cursor_expired")
			}
			if cursor.Kind != "workspace.groups" {
				return code("cursor_expired")
			}
			last = cursor.LastKey
		}
		q := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if last != "" {
			q = q.Where("id > ?", last)
		}
		var groups []WorkspaceAgentGroup
		if err := q.Order("id ASC").Limit(query.Limit + 1).Find(&groups).Error; err != nil {
			return code("database_unavailable")
		}
		more := len(groups) > query.Limit
		if more {
			groups = groups[:query.Limit]
		}
		for _, group := range groups {
			result.Items = append(result.Items, workspaceGroupView(group))
		}
		if more {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: canonical, CatalogueRevision: generation, Kind: "workspace.groups", LastKey: groups[len(groups)-1].ID})
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
