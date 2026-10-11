package service

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// searchWorkspaceFiles preserves the existing file-array surface. It never
// inherits legacy scopes or grants access to staging through a broad prefix.
func (s *Service) searchWorkspaceFiles(p Principal, space, query, prefix, mime, after string, trash bool) ([]File, error) {
	if p.Agent == nil || trash {
		return nil, code("forbidden")
	}
	if space == "" {
		return nil, code("invalid_space")
	}
	if prefix != "" {
		cleaned, err := cleanPath(prefix)
		if err != nil || cleaned != prefix {
			return nil, code("invalid_request")
		}
	}
	var since time.Time
	if after != "" {
		var err error
		since, err = time.Parse(time.RFC3339, after)
		if err != nil {
			return nil, code("invalid_time")
		}
	}
	out := []File{}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		grant, err := s.authorizeWorkspaceActionOn(tx, p, space, p.Agent.ConnectionID, p.Agent.Prefix, "files.read", time.Now().UTC())
		if err != nil {
			return err
		}
		if prefix == "" {
			prefix = grant.Prefix
		}
		if !contains(grant.Prefix, prefix) || workspaceStagingPath(prefix) {
			return code("forbidden")
		}
		q := tx.Where("owner = ? AND space_id = ? AND trashed = ? AND purge_started_at IS NULL", p.UserID, space, false)
		if mime != "" {
			q = q.Where("mime = ?", mime)
		}
		if !since.IsZero() {
			q = q.Where("updated >= ?", since.UTC())
		}
		var candidates []File
		if err := q.Order("updated DESC, id ASC").Find(&candidates).Error; err != nil {
			return code("database_unavailable")
		}
		for _, file := range candidates {
			if !contains(prefix, file.Path) || workspaceStagingPath(file.Path) || !strings.Contains(strings.ToLower(file.Path), strings.ToLower(query)) {
				continue
			}
			current, _, err := s.workspaceReadVersionOn(tx, p, file.ID, file.VersionID)
			if err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			out = append(out, current)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
