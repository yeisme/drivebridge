package contract

import "time"

// CollectionMemberDecisions is complete or unavailable, never a filtered edit base.
type CollectionMemberDecisions struct {
	CollectionID string                     `json:"collection_id"`
	Revision     uint64                     `json:"revision" jsonschema:"minimum=1,maximum=9007199254740990"`
	Items        []CollectionMemberDecision `json:"items" jsonschema:"maxItems=10000"`
}

type CollectionMemberDecision struct {
	FileID   string `json:"file_id" jsonschema:"pattern=^[a-f0-9]{48}$"`
	SpaceID  string `json:"space_id"`
	Path     string `json:"path"`
	Decision string `json:"decision" jsonschema:"enum=include,enum=exclude"`
}

// CollectionMembersChangeRequest replaces explicit decisions, not evaluated rule results.
type CollectionMembersChangeRequest struct {
	ExpectedRevision uint64   `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740990"`
	Include          []string `json:"include" jsonschema:"maxItems=10000"`
	Exclude          []string `json:"exclude" jsonschema:"maxItems=10000"`
}

type CollectionMembersReceipt struct {
	CollectionID   string    `json:"collection_id"`
	RequestDigest  string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64    `json:"result_revision" jsonschema:"minimum=2,maximum=9007199254740991"`
	CommittedAt    time.Time `json:"committed_at"`
}
