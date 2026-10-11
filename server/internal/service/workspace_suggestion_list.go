package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSuggestionList(w http.ResponseWriter, r *http.Request, p Principal) error {
	values := r.URL.Query()
	page, err := contract.ParsePageQuery(values)
	if err != nil {
		return err
	}
	for key, vs := range values {
		if key != "version_id" && key != "cursor" && key != "limit" {
			return code("invalid_request")
		}
		if len(vs) != 1 {
			return code("invalid_request")
		}
	}
	versionID := values.Get("version_id")
	if versionID == "" {
		return code("invalid_request")
	}
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	result := contract.Page[contract.LabelSuggestionView]{Items: []contract.LabelSuggestionView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var file File
		if err := tx.Where("id = ? AND owner = ?", r.PathValue("id"), p.UserID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
			return code("not_found")
		}
		var version Version
		if err := tx.Where("id = ? AND owner = ? AND file_id = ?", versionID, p.UserID, file.ID).First(&version).Error; err != nil {
			return code("not_found")
		}
		agentID := ""
		if p.Agent != nil {
			grant, err := s.authorizeWorkspaceActionOn(tx, p, file.SpaceID, version.ConnectionID, file.Path, "labels.suggest", result.ObservedAt)
			if err != nil {
				return err
			}
			agentID = grant.AgentID
		} else {
			if err := workspaceGroupHumanOn(tx, p); err != nil {
				return err
			}
			var space Space
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
				return code("not_found")
			}
			if p.ScopeSpace != "" && p.ScopeSpace != space.ID {
				return code("not_found")
			}
			if err := s.governanceAllow(tx, p, space.ID, file.Path, "read"); err != nil {
				return err
			}
		}
		q := tx.Where("owner = ? AND tenant_id = ? AND file_id = ? AND version_id = ?", p.UserID, p.TenantID, file.ID, version.ID)
		if agentID != "" {
			q = q.Where("agent_id = ?", agentID)
		}
		var suggestions []WorkspaceLabelSuggestion
		if err := q.Order("agent_id ASC, id ASC").Find(&suggestions).Error; err != nil {
			return code("database_unavailable")
		}
		binding, _ := json.Marshal([]any{"workspace.suggestions.v1", p.TenantID, p.ScopeSpace, p.DataOwner, agentID, file.ID, version.ID, page.Limit})
		raw, err := json.Marshal(suggestions)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.suggestions" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range suggestions {
				if item.ID == cursor.LastKey {
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
		if end > len(suggestions) {
			end = len(suggestions)
		}
		for _, item := range suggestions[start:end] {
			result.Items = append(result.Items, contract.LabelSuggestionView{SuggestionID: item.ID, Revision: item.Revision, FileID: item.FileID, VersionID: item.VersionID, AgentID: item.AgentID, Labels: item.Labels, CreationKey: item.CreationKey, State: item.State})
		}
		if end < len(suggestions) {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.suggestions", LastKey: suggestions[end-1].ID})
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
