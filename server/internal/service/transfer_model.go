package service

import "time"

// TransferPlan freezes one cross-storage copy or migrate intent: fixed
// sources (pinned file/version refs with digests), one target and a
// conflict policy. Copying and source cleanup are separate facts with
// separate authorizations; a completed copy never claims migration done.
type TransferPlan struct {
	ID             string `json:"plan_id" gorm:"primaryKey"`
	Owner          string `json:"-" gorm:"index"`
	TenantID       string `json:"tenant_id,omitempty"`
	Mode           string `json:"mode"` // copy|migrate
	TargetSpaceID  string `json:"target_space_id"`
	TargetPath     string `json:"target_path"`
	ConflictPolicy string `json:"conflict_policy"` // fail|keep_both|overwrite
	// Revision is the optimistic plan counter: execution and cleanup carry
	// expected_revision; stale writers get plan_stale.
	Revision uint64 `json:"revision"`
	Phase    string `json:"phase"` // planned|executing|copied|partial|source_cleanup_pending|completed|cancelled
	// OperationID anchors the execution on the original task; replays and
	// recovery read it back instead of creating a second plan.
	OperationID         string    `json:"operation_id,omitempty"`
	ItemsTotal          int64     `json:"items_total"`
	CleanupAuthorized   bool      `json:"source_cleanup_authorized"`
	CrossSpaceConfirmed bool      `json:"cross_space_confirmed,omitempty"`
	Key                 string    `json:"-" gorm:"uniqueIndex:transfer_plan_key"`
	Fingerprint         string    `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (TransferPlan) TableName() string { return "transfer_plans" }

// TransferItem is one frozen source-to-target mapping with its own
// original copy operation and verification fact.
type TransferItem struct {
	ID              string `gorm:"primaryKey"`
	PlanID          string `json:"plan_id" gorm:"uniqueIndex:transfer_item_identity;index"`
	ItemKey         string `json:"item_key" gorm:"uniqueIndex:transfer_item_identity"`
	Owner           string `json:"-"`
	SourceFileID    string `json:"source_file_id"`
	SourceVersionID string `json:"source_version_id"`
	SourceSHA256    string `json:"source_sha256,omitempty"`
	TargetPath      string `json:"target_path"`
	// CopyOperationID links the original per-item transfer; recovery and
	// replays land on it, never on a second task.
	CopyOperationID string    `json:"copy_operation_id,omitempty"`
	TargetFileID    string    `json:"target_file_id,omitempty"`
	TargetVersionID string    `json:"target_version_id,omitempty"`
	TargetSHA256    string    `json:"target_sha256,omitempty"`
	Status          string    `json:"status"`                     // pending|copying|copied|paused|failed|cancelled|source_trashed|cleanup_pending
	ResultCertainty string    `json:"result_certainty,omitempty"` // confirmed|unknown
	Error           string    `json:"error,omitempty"`
	CleanupState    string    `json:"cleanup_state,omitempty"` // none|requested|trashed|pending|unsupported
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (TransferItem) TableName() string { return "transfer_items" }
