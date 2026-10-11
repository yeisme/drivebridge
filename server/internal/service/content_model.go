package service

import "time"

// ContentPolicy is the human-configured processing policy for one space:
// what may be submitted to content processing, in which prefixes/mime
// types, with which layers and budgets, and whether egress to the content
// owner is allowed. Created disabled by default; no policy, no processing.
type ContentPolicy struct {
	ID              string    `json:"policy_id" gorm:"primaryKey"`
	Owner           string    `json:"-" gorm:"uniqueIndex:content_policy_space"`
	SpaceID         string    `json:"space_id" gorm:"uniqueIndex:content_policy_space"`
	PolicyRevision  uint64    `json:"policy_revision"`
	Prefixes        string    `json:"prefixes,omitempty"`
	MimeAllowlist   string    `json:"mime_allowlist,omitempty"`
	RequestedLayers []byte    `json:"requested_layers,omitempty"`
	MaxItems        int64     `json:"max_items,omitempty"`
	MaxBytes        int64     `json:"max_bytes,omitempty"`
	BudgetRef       string    `json:"budget_ref,omitempty"`
	EgressScope     string    `json:"egress_scope,omitempty"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (ContentPolicy) TableName() string { return "content_policies" }

// ContentBinding links one pinned managed version to a content-owner
// project for a stated purpose. The binding state only projects the
// content owner's processing facts; it never replaces the file's own
// Operation. A version change or tombstone stops new consumption; late
// results never attach to the new version.
type ContentBinding struct {
	ID                string `json:"binding_id" gorm:"primaryKey"`
	Owner             string `json:"-" gorm:"index"`
	TenantID          string `json:"tenant_id,omitempty"`
	FileID            string `json:"file_id"`
	VersionID         string `json:"version_id"`
	ManjuConnectionID string `json:"manju_connection_id"`
	ProjectRef        string `json:"project_ref"`
	Purpose           string `json:"purpose"`
	State             string `json:"state"` // not_processed|processing|searchable|failed|stale|unsupported
	PolicyRevision    uint64 `json:"policy_revision"`
	IndexGeneration   string `json:"index_generation,omitempty"`
	OperationID       string `json:"operation_id,omitempty"`
	// ProcessKey/ProcessFingerprint persist the processing idempotency:
	// the same key with the same intent replays the original operation,
	// a different intent under the key conflicts, and a failed attempt
	// leaves the key free for an explicit retry.
	ProcessKey         string    `json:"-"`
	ProcessFingerprint string    `json:"-"`
	Error              string    `json:"error,omitempty"`
	Key                string    `json:"-" gorm:"uniqueIndex:content_binding_key"`
	Fingerprint        string    `json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (ContentBinding) TableName() string { return "content_bindings" }
