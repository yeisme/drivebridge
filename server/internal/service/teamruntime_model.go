package service

import (
	"time"
)

// RuntimeJob is a durable unit of work owned by the team runtime. The lease
// epoch doubles as the monotonic fencing token: every claim increments it and
// every metadata publish must present the token it claimed with.
type RuntimeJob struct {
	ID                  string `json:"id" gorm:"primaryKey"`
	Kind                string `json:"kind" gorm:"index"`
	OriginalOperationID string `json:"original_operation_id" gorm:"index"`
	AttemptGeneration   int64  `json:"attempt_generation"`
	LeaseEpoch          int64  `json:"lease_epoch"`
	OwnerScope          string `json:"owner_scope" gorm:"index"`
	Status              string `json:"status" gorm:"index"`
	Payload             string `json:"payload,omitempty"`
	// Diagnostics retains fenced or rejected write attempts (including the
	// stale token and worker id) so a late worker's intent stays inspectable
	// without ever overwriting the new owner's state.
	Diagnostics string    `json:"diagnostics,omitempty"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (RuntimeJob) TableName() string { return "runtime_jobs" }

// OutboxEvent is the at-least-once business event written in the same
// transaction as the business rows it describes. Consumers deduplicate by
// event identity and apply idempotently per resource revision; a tombstone or
// a newer revision always wins over an older replay.
type OutboxEvent struct {
	EventID          string    `json:"event_id" gorm:"primaryKey"`
	Kind             string    `json:"kind"`
	ResourceID       string    `json:"resource_id" gorm:"index:outbox_apply"`
	ResourceRevision int64     `json:"resource_revision"`
	Tombstone        bool      `json:"tombstone"`
	SourceSequence   int64     `json:"source_sequence" gorm:"uniqueIndex"`
	OwnerScope       string    `json:"owner_scope" gorm:"index:outbox_apply"`
	Payload          string    `json:"payload,omitempty"`
	Published        bool      `json:"published" gorm:"index"`
	CreatedAt        time.Time `json:"created_at" gorm:"column:created_at"`
}

func (OutboxEvent) TableName() string { return "runtime_outbox" }

// WorkerLease records which worker currently owns a job. FencingToken is the
// job's monotonic lease epoch; ExpiresAt never proves the old worker stopped —
// external side effects are reconciled through the original operation first.
type WorkerLease struct {
	ID           string    `json:"job_id" gorm:"primaryKey"`
	WorkerID     string    `json:"worker_id"`
	FencingToken int64     `json:"fencing_token"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"index"`
	Heartbeat    time.Time `json:"heartbeat_at" gorm:"column:heartbeat_at"`
}

func (WorkerLease) TableName() string { return "runtime_leases" }

// ProjectionCheckpoint is the per-owner-scope read pointer of one projection.
// The watermark only advances after an idempotent successful apply, and never
// crosses owner scopes.
type ProjectionCheckpoint struct {
	Projection string    `json:"projection" gorm:"primaryKey"`
	OwnerScope string    `json:"owner_scope" gorm:"primaryKey"`
	Generation string    `json:"generation"`
	Watermark  int64     `json:"watermark"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (ProjectionCheckpoint) TableName() string { return "runtime_checkpoints" }

// ProjectionGeneration tracks one rebuild lifecycle of a projection: the
// frozen source watermark, the applied watermark, and the verified digest.
// Old generations stay readable for diagnostics; reads never mix them.
type ProjectionGeneration struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	Projection       string     `json:"projection" gorm:"index"`
	Generation       string     `json:"generation"`
	Phase            string     `json:"phase" gorm:"index"`
	SourceWatermark  int64      `json:"source_watermark"`
	AppliedWatermark int64      `json:"applied_watermark"`
	RowCount         int64      `json:"row_count"`
	Tombstones       int64      `json:"tombstones"`
	Digest           string     `json:"digest,omitempty"`
	Ready            bool       `json:"ready"`
	OperationID      string     `json:"operation_id" gorm:"index"`
	CreatedAt        time.Time  `json:"created_at" gorm:"column:created_at"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty" gorm:"column:activated_at"`
}

func (ProjectionGeneration) TableName() string { return "runtime_generations" }

// ProjectionRow is one materialized projection entry for a generation. A
// tombstone row marks deletion without dropping history, so out-of-order
// replays cannot resurrect a deleted resource.
type ProjectionRow struct {
	Projection string    `json:"projection" gorm:"primaryKey:projection_row_pk"`
	Generation string    `json:"generation" gorm:"primaryKey:projection_row_pk"`
	ResourceID string    `json:"resource_id" gorm:"primaryKey:projection_row_pk"`
	OwnerScope string    `json:"owner_scope" gorm:"index"`
	Revision   int64     `json:"resource_revision"`
	Tombstone  bool      `json:"tombstone"`
	Digest     string    `json:"digest"`
	Payload    string    `json:"payload,omitempty"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (ProjectionRow) TableName() string { return "runtime_projection_rows" }

// RuntimeSequence is the single-row monotonic publisher counter behind
// OutboxEvent.SourceSequence. Allocation happens inside the business
// transaction so sequence order can never contradict commit order.
type RuntimeSequence struct {
	Name string `gorm:"primaryKey"`
	Next int64
}

func (RuntimeSequence) TableName() string { return "runtime_sequences" }

// GenerationScopeProgress is the rebuild-local watermark of one owner scope
// inside one rebuild generation. Each scope resumes from its own row so one
// scope's progress can never advance past events another scope has not yet
// consumed; the generation watermark is the minimum across scopes.
type GenerationScopeProgress struct {
	Generation string    `gorm:"primaryKey"`
	OwnerScope string    `gorm:"primaryKey"`
	AppliedSeq int64     `gorm:"column:applied_seq"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

const fileCatalogueProjection = "file_catalogue"

// runtimeDialect reports whether the metadata database runs the team
// PostgreSQL deployment; SQLite keeps the single-machine writer model.
func (s *Service) runtimeDialect() string {
	if s.DB != nil && s.DB.Dialector != nil {
		return s.DB.Dialector.Name()
	}
	return ""
}
