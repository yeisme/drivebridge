package bridge

import (
	"errors"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

// GuardSource is private application data for a native host. Root is never a
// renderer projection; each subsequent read still requires a fresh guard lease.
type GuardSource struct {
	Guard ManagedGuard
	Root  string
}

func (s *Service) GuardSources(instance, after string, limit int) ([]GuardSource, string, error) {
	if limit < 1 || limit > 200 {
		return nil, "", errors.New("request_invalid")
	}
	var rows []ManagedGuard
	if err := s.DB.Where("instance_id = ? AND state IN ? AND id > ?", instance, []string{GuardActive, GuardHidden}, after).Order("id").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", errors.New("guard_unavailable")
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]GuardSource, 0, len(rows))
	for _, row := range rows {
		var space Space
		if err := s.DB.First(&space, "id = ?", row.SpaceID).Error; err != nil {
			return nil, "", errors.New("guard_unavailable")
		}
		if !guardScopesCover(row.Scopes, []string{string(desktopcontract.ScopeMetadata)}) {
			return nil, "", errors.New("managed_scope_unverifiable")
		}
		result = append(result, GuardSource{Guard: row, Root: space.LocalRoot})
	}
	return result, next, nil
}
