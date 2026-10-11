package contract

import "time"

// Page is the new workspace list envelope body. Existing SDK list shapes
// remain unchanged. Unknown totals must be null, not an invented zero.
type Page[T any] struct {
	Items      []T       `json:"items"`
	NextCursor string    `json:"next_cursor"`
	ObservedAt time.Time `json:"observed_at"`
	Total      *uint64   `json:"total" jsonschema:"nullable"`
	TotalKind  string    `json:"total_kind" jsonschema:"enum=exact,enum=unknown"`
}

type CreateGroupRequest struct {
	Name string `json:"name" jsonschema:"minLength=1,maxLength=80"`
}

type RenameGroupRequest struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	Name             string `json:"name" jsonschema:"minLength=1,maxLength=80"`
}

type DeletedGroupView struct {
	GroupID string `json:"group_id"`
	Deleted bool   `json:"deleted"`
}
