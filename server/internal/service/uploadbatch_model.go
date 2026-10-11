package service

import "time"

// UploadBatch organizes original upload Operations for one enumeration. It
// never replaces them: every item references the Operation that moved its
// bytes, and the batch key is bound to actor, tenant, space and conflict
// policy so a lost creation response replays onto the same identity.
type UploadBatch struct {
	ID             string `json:"id" gorm:"primaryKey"`
	Owner          string `json:"-" gorm:"uniqueIndex:upload_batch_key"`
	TenantID       string `json:"tenant_id,omitempty" gorm:"index"`
	SpaceID        string `json:"space_id"`
	ConflictPolicy string `json:"conflict_policy"`
	Key            string `json:"-" gorm:"uniqueIndex:upload_batch_key"`
	Fingerprint    string `json:"-"`
	// Revision is the batch-level optimistic counter: every item page, seal
	// and control decision bumps it so stale readers can detect plan_stale.
	Revision uint64 `json:"batch_revision"`
	Phase    string `json:"phase"` // enumerating|sealed|transferring|paused|cancelled|completed
	// OperationID is the batch task's own anchor operation (kind
	// upload_batch); read-back and recovery flow through that original ID.
	OperationID string     `json:"operation_id"`
	SealedAt    *time.Time `json:"sealed_at,omitempty"`
	ItemsTotal  *int64     `json:"items_total,omitempty"`
	BytesTotal  int64      `json:"bytes_total,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (UploadBatch) TableName() string { return "upload_batches" }

// UploadBatchItem is one enumerated source entry. ItemKey is the durable
// identity: relative path plus source fingerprint, so re-selecting a file
// with different content is a different item (content_mismatch on replay
// against the original one). Empty directories are explicit IsDir entries.
type UploadBatchItem struct {
	ID              string    `gorm:"primaryKey"`
	BatchID         string    `json:"batch_id" gorm:"uniqueIndex:upload_batch_item;index"`
	ItemKey         string    `json:"item_key" gorm:"uniqueIndex:upload_batch_item"`
	Owner           string    `json:"-"`
	CaseFolded      string    `json:"-"` // lower-cased relative path for backend name-space collisions
	RelativePath    string    `json:"relative_path"`
	IsDir           bool      `json:"is_dir,omitempty"`
	Size            int64     `json:"size_bytes"`
	SHA256          string    `json:"sha256,omitempty"`
	FileID          string    `json:"file_id,omitempty"` // new_version policy target
	ExpectedVersion string    `json:"expected_version,omitempty"`
	OperationID     string    `json:"operation_id,omitempty"` // the original per-item Operation
	Status          string    `json:"status"`                 // pending|transferring|completed|failed|cancelled
	Error           string    `json:"error,omitempty"`
	Certainty       string    `json:"result_certainty,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (UploadBatchItem) TableName() string { return "upload_batch_items" }

// UploadSession records the durable transfer protocol of one upload
// operation: which client part size the protocol agreed on, how the backend
// reassembles those parts, and the session generation used when an explicit
// recovery rebuilds a transfer session under the original Operation.
type UploadSession struct {
	ID              string `gorm:"primaryKey"`
	Owner           string `json:"-"`
	OperationID     string `json:"operation_id" gorm:"uniqueIndex"`
	BatchID         string `json:"batch_id,omitempty" gorm:"index"`
	Protocol        string `json:"protocol"`
	PartSize        int64  `json:"part_size"`
	BackendPartSize int64  `json:"backend_part_size"`
	// Generation increments only when an explicit recovery rebuilds the
	// backend session under the same Operation; it never rekeys the client
	// part numbering space.
	Generation int64
	// ExpiresAt is the backend transfer session window (S3 multipart must
	// refused with transfer_session_expired until the original actor's
	// explicit resume rebuilds the session under this same Operation.
	ExpiresAt      *time.Time `json:"-"`
	Durability     string
	SourceReselect bool      `json:"source_reselection_required"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (UploadSession) TableName() string { return "upload_sessions" }

const uploadBatchMaxPage = 500

// uploadBatchPolicy validates the batch conflict policy vocabulary. The
// default is fail: an existing file at the target path is an error unless the
// caller explicitly chose keep_both or supplied file_id+expected_version.
func uploadBatchPolicyValid(p string) bool {
	return p == "fail" || p == "keep_both" || p == "new_version"
}
