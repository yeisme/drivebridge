package contract

import "time"

// Workspace hold writes are human-only and independently feature-gated.
type WorkspaceHoldCreate struct {
	VersionID string `json:"version_id"`
	Purpose string `json:"purpose" jsonschema:"enum=fixed_delivery,enum=legal_hold,enum=transfer_window"`
	SourceKind string `json:"source_kind" jsonschema:"enum=share,enum=operation"`
	SourceRef string `json:"source_ref"`
	ExpiresAt string `json:"expires_at"`
}

type WorkspaceHoldRelease struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740990"`
}

type WorkspaceHoldLookup struct {
	Action string `json:"action" jsonschema:"enum=create,enum=release"`
	IdempotencyKey string `json:"idempotency_key"`
}

type WorkspaceHoldView struct {
	HoldID string `json:"hold_id"`
	Revision uint64 `json:"revision"`
	State string `json:"state" jsonschema:"enum=active,enum=released,enum=inactive_source_or_expiry"`
	FileID string `json:"file_id"`
	VersionID string `json:"version_id"`
	Purpose string `json:"purpose"`
	SourceKind string `json:"source_kind"`
	SourceRef string `json:"source_ref"`
	ExpiresAt string `json:"expires_at"`
	AllowedActions []string `json:"allowed_actions"`
}

type WorkspaceHoldList struct {
	Items []WorkspaceHoldView `json:"items"`
	NextCursor string `json:"next_cursor"`
	ObservedAt time.Time `json:"observed_at"`
	Total *int64 `json:"total" jsonschema:"nullable"`
	TotalKind string `json:"total_kind" jsonschema:"enum=unknown,enum=exact"`
	ManagementEnabled bool `json:"management_enabled"`
}

// A receipt is the immutable original commit, never the hold's current state.
type WorkspaceHoldReceipt struct {
	HoldID string `json:"hold_id"`
	FileID string `json:"file_id"`
	Action string `json:"action" jsonschema:"enum=create,enum=release"`
	ResultRevision uint64 `json:"result_revision"`
	RequestDigest string `json:"request_digest"`
	CommittedAt time.Time `json:"committed_at"`
}
