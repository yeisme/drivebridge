package contract

import (
	"encoding/json"
	"time"
)

// RawMessage preserves the released creation identity. Consumers must not
// reformat an original rule when calculating its digest or retrying a request.
type CreateFileCollectionRequest struct {
	SpaceID string          `json:"space_id"`
	Name    string          `json:"name" jsonschema:"minLength=1,maxLength=200"`
	Kind    string          `json:"kind" jsonschema:"enum=manual,enum=rule"`
	Scope   string          `json:"scope" jsonschema:"enum=private,enum=team"`
	Rule    json.RawMessage `json:"rule,omitempty"`
}

// FileCollectionView is a current observation, NOT the original creation receipt.
type FileCollectionView struct {
	CollectionID string     `json:"collection_id"`
	SpaceID      string     `json:"space_id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind" jsonschema:"enum=manual,enum=rule"`
	Scope        string     `json:"scope" jsonschema:"enum=private,enum=team"`
	Rule         string     `json:"rule,omitempty"`
	RuleRevision uint64     `json:"rule_revision,omitempty"`
	Revision     uint64     `json:"revision"`
	ComputedAt   *time.Time `json:"computed_at,omitempty"`
	MembersTotal int64      `json:"members_total" jsonschema:"minimum=0"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CollectionRule is scoped to its containing collection. A supplied space_ids
// must equal that one space; this cross-field authorization is enforced by the
// application, not by a portable JSON Schema. Limits are enumeration page sizes.
type CollectionRule struct {
	Mode     string            `json:"mode" jsonschema:"enum=exact"`
	SpaceIDs []string          `json:"space_ids,omitempty" jsonschema:"minItems=1,maxItems=1"`
	Filters  []json.RawMessage `json:"filters,omitempty" jsonschema:"maxItems=64"`
	Sort     []json.RawMessage `json:"sort,omitempty" jsonschema:"maxItems=1"`
	Limit    int               `json:"limit,omitempty" jsonschema:"minimum=0,maximum=200"`
	Cursor   string            `json:"cursor,omitempty" jsonschema:"maxLength=0"`
}

type CollectionRuleFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type CollectionRuleSort struct {
	Field     string `json:"field" jsonschema:"enum=updated_at,enum=name,enum=path_prefix,enum=size_bytes"`
	Direction string `json:"direction" jsonschema:"enum=asc,enum=desc"`
}
