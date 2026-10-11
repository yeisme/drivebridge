package contract

import (
	"net/url"
	"strconv"
)

// PageQuery parses a bounded list request without silently resetting malformed
// cursors or limits. Cursor authentication belongs to the application service.
type PageQuery struct {
	Cursor string
	Limit  int
}

func ParsePageQuery(values url.Values) (PageQuery, error) {
	result := PageQuery{Limit: 50}
	if cursors, ok := values["cursor"]; ok {
		if len(cursors) != 1 {
			return PageQuery{}, ErrInvalidRequest
		}
		result.Cursor = cursors[0]
	}
	if limits, ok := values["limit"]; ok {
		if len(limits) != 1 || limits[0] == "" {
			return PageQuery{}, ErrInvalidRequest
		}
		for _, ch := range limits[0] {
			if ch < '0' || ch > '9' {
				return PageQuery{}, ErrInvalidRequest
			}
		}
		limit, err := strconv.Atoi(limits[0])
		if err != nil || limit < 1 || limit > 200 {
			return PageQuery{}, ErrInvalidRequest
		}
		result.Limit = limit
	}
	return result, nil
}
