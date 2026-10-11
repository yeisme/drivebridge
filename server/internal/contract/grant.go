package contract

import "time"

// GrantView is the public projection, not a persistence model. Nullable expiry
// fields are required on the wire; absence must not imply unlimited access.
type GrantView struct {
	GrantID              string     `json:"grant_id"`
	AgentID              string     `json:"agent_id"`
	Name                 string     `json:"name"`
	Protocol             string     `json:"protocol" jsonschema:"enum=mcp,enum=http"`
	SpaceID              string     `json:"space_id"`
	ConnectionID         string     `json:"connection_id"`
	Prefix               string     `json:"prefix"`
	Actions              []string   `json:"actions"`
	RestrictionProfile   string     `json:"restriction_profile" jsonschema:"enum=personal_workspace_v1"`
	Revision             uint64     `json:"revision"`
	State                string     `json:"state" jsonschema:"enum=active,enum=disabled,enum=expired,enum=revoked"`
	ExpiryMode           string     `json:"expiry_mode" jsonschema:"enum=never,enum=duration"`
	ExpiresAt            *time.Time `json:"expires_at" jsonschema:"nullable"`
	IssuedAt             time.Time  `json:"issued_at"`
	ExpiryChangedAt      *time.Time `json:"expiry_changed_at" jsonschema:"nullable"`
	CredentialGeneration uint64     `json:"credential_generation"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastSeenAt           *time.Time `json:"last_seen_at,omitempty"`
	// Present only for explicitly requested organization projections.
	Organization *AgentOrganizationView `json:"organization,omitempty"`
}

// AgentGroupView and AgentOrganizationView are independent of grant revisions.
type AgentGroupView struct {
	GroupID   string    `json:"group_id"`
	Name      string    `json:"name"`
	Revision  uint64    `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AgentOrganizationView struct {
	AgentID   string    `json:"agent_id"`
	GroupID   *string   `json:"group_id" jsonschema:"nullable"`
	Pinned    bool      `json:"pinned"`
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}
