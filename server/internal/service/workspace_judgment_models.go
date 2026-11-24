package service

import (
	"encoding/json"
	"time"
)

// Managed judgment wiring (drivebridge-judgment-sdk-managed-execution-v1).
// The workspace keeps domain state here; the public SDK only supplies the
// provider-neutral contract and in-process transports. Snapshots and claims
// hold references and digests only, never credentials or provider payloads.

// WorkspaceJudgmentProfileSnapshot is an immutable profile revision. The same
// (owner, profile_id, revision) tuple with different content is a conflict.
type WorkspaceJudgmentProfileSnapshot struct {
	ID               string `gorm:"primaryKey"`
	Owner            string `gorm:"not null;uniqueIndex:workspace_judgment_snapshot_key"`
	ProfileID        string `gorm:"not null;uniqueIndex:workspace_judgment_snapshot_key"`
	Revision         uint64 `gorm:"not null;uniqueIndex:workspace_judgment_snapshot_key"`
	Provider         string `gorm:"not null"`
	Model            string `gorm:"not null"`
	CredentialSource string `gorm:"not null"`
	CredentialRef    string `gorm:"not null;default:''"`
	CredentialEnv    string `gorm:"not null;default:''"`
	Enabled          bool   `gorm:"not null"`
	Digest           string `gorm:"not null"`
	Profile          []byte `gorm:"serializer:json;not null"`
	CreatedAt        time.Time
}

func (WorkspaceJudgmentProfileSnapshot) TableName() string {
	return "workspace_judgment_profile_snapshots"
}

// WorkspaceJudgmentActivation is the per-owner active binding. Activation is
// an explicit application-service action; evaluation never mutates it.
type WorkspaceJudgmentActivation struct {
	Owner       string `gorm:"primaryKey"`
	ProfileID   string `gorm:"not null;default:''"`
	Revision    uint64 `gorm:"not null;default:0"`
	Digest      string `gorm:"not null;default:''"`
	Mode        string `gorm:"not null;default:'off'"`
	Generation  uint64 `gorm:"not null;default:0"`
	ActivatedBy string `gorm:"not null;default:''"`
	UpdatedAt   time.Time
}

func (WorkspaceJudgmentActivation) TableName() string { return "workspace_judgment_activations" }

// WorkspaceJudgmentAttempt is the durable claim row for one owner/project/
// request/attempt. Unfinished claims stay sticky: restart cannot prove the
// provider accepted them, so they remain unknown until reviewed.
type WorkspaceJudgmentAttempt struct {
	Key             string `gorm:"primaryKey;size:191"`
	Owner           string `gorm:"not null;uniqueIndex:workspace_judgment_attempt_key"`
	Project         string `gorm:"not null;uniqueIndex:workspace_judgment_attempt_key"`
	RequestID       string `gorm:"not null;uniqueIndex:workspace_judgment_attempt_key"`
	AttemptID       string `gorm:"not null;uniqueIndex:workspace_judgment_attempt_key"`
	InputDigest     string `gorm:"not null"`
	ProfileDigest   string `gorm:"not null"`
	State           string `gorm:"not null"`
	ErrorCode       string `gorm:"not null;default:''"`
	SubmissionState string `gorm:"not null;default:''"`
	// Result holds the neutral validated result JSON bytes exactly as the
	// SDK receipt persisted them; never a provider payload verbatim.
	Result    []byte `gorm:"type:blob"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (WorkspaceJudgmentAttempt) TableName() string { return "workspace_judgment_attempts" }

// judgmentProjectScope pins the durable-claim project namespace for the
// workspace suggestion producer. The owner is the workspace user.
const judgmentProjectScope = "drivebridge-workspace"

// judgmentRequestID derives the stable request identity for one suggestion
// production on a fixed file/version with one idempotency key.
func judgmentRequestID(owner, fileID, versionID, key string) string {
	return Hash(owner + "\x00" + fileID + "\x00" + versionID + "\x00" + key)
}

// judgmentReceiptJSON is the neutral bounded readback shape for internal
// diagnostics. It mirrors the stored receipt fields without provider bodies.
type judgmentReceiptJSON struct {
	SchemaVersion   string          `json:"schema_version"`
	Owner           string          `json:"owner"`
	Project         string          `json:"project"`
	RequestID       string          `json:"request_id"`
	AttemptID       string          `json:"attempt_id"`
	InputDigest     string          `json:"input_digest"`
	ProfileDigest   string          `json:"profile_digest"`
	State           string          `json:"state"`
	ErrorCode       string          `json:"error_code,omitempty"`
	SubmissionState string          `json:"submission_state,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
