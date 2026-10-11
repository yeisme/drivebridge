package contract

// Request fields use presence-aware decoding even where explicit null is
// forbidden. The request schema and application validator reject null there.
type CreateGrantRequest struct {
	Name             string        `json:"name" jsonschema:"minLength=1,maxLength=120"`
	Protocol         string        `json:"protocol" jsonschema:"enum=mcp,enum=http"`
	SpaceID          string        `json:"space_id" jsonschema:"minLength=1"`
	ConnectionID     string        `json:"connection_id" jsonschema:"minLength=1"`
	Prefix           string        `json:"prefix"`
	Actions          []string      `json:"actions" jsonschema:"minItems=1,uniqueItems=true"`
	ExpiryMode       Field[string] `json:"expiry_mode,omitempty"`
	ExpiresInSeconds Field[int64]  `json:"expires_in_seconds,omitempty"`
}

type PatchGrantRequest struct {
	ExpectedRevision uint64          `json:"expected_revision"`
	Name             Field[string]   `json:"name,omitempty"`
	Prefix           Field[string]   `json:"prefix,omitempty"`
	Actions          Field[[]string] `json:"actions,omitempty"`
	Enabled          Field[bool]     `json:"enabled,omitempty"`
	ExpiryMode       Field[string]   `json:"expiry_mode,omitempty"`
	ExpiresInSeconds Field[int64]    `json:"expires_in_seconds,omitempty"`
}

type PatchOrganizationRequest struct {
	ExpectedRevision uint64        `json:"expected_revision"`
	GroupID          Field[string] `json:"group_id,omitempty"`
	Pinned           Field[bool]   `json:"pinned,omitempty"`
}
