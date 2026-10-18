package service

import (
	"encoding/json"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func workspaceMetadataQuery(raw, scope string) (url.Values, contract.PageQuery, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, contract.PageQuery{}, code("invalid_request")
	}
	for name, entries := range values {
		if len(entries) != 1 || (name != scope && name != "cursor" && name != "limit") {
			return nil, contract.PageQuery{}, code("invalid_request")
		}
	}
	if values.Get(scope) == "" || len(values.Get(scope)) > 512 {
		return nil, contract.PageQuery{}, code("invalid_request")
	}
	page, err := contract.ParsePageQuery(values)
	return values, page, err
}

// Cursor generations contain only currently visible facts. Hidden target
// changes cannot leak through a total or invalidate another owner's cursor.
func (s *Service) workspaceMetadataPage(p Principal, kind string, scope []string, q contract.PageQuery, views any, keys []string) (int, int, string, error) {
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit < 1 || q.Limit > 200 {
		return 0, 0, "", code("invalid_request")
	}
	raw, err := json.Marshal(views)
	if err != nil {
		return 0, 0, "", code("database_unavailable")
	}
	generation := Hash(string(raw))
	binding, _ := json.Marshal([]any{kind, p.TenantID, p.ScopeSpace, p.DataOwner, scope, q.Limit})
	start := 0
	if q.Cursor != "" {
		cursor, err := s.openOrgCursor(q.Cursor, p.UserID, string(binding), generation)
		if err != nil || cursor.Kind != kind {
			return 0, 0, "", code("cursor_expired")
		}
		found := false
		for i, key := range keys {
			if key == cursor.LastKey {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return 0, 0, "", code("cursor_expired")
		}
	}
	end := min(start+q.Limit, len(keys))
	next := ""
	if end < len(keys) {
		next, err = s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: kind, LastKey: keys[end-1]})
		if err != nil {
			return 0, 0, "", code("database_unavailable")
		}
	}
	return start, end, next, nil
}
