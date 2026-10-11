package service

import "time"

// AnalyticsSnapshot is one service-generated export manifest: the frozen
// scope, fields and watermarks of an explicitly authorized analytical
// copy. The manifest never carries credentials, temporary URLs or file
// bodies; consumers get a separate data file reference only.
type AnalyticsSnapshot struct {
	ID               string `json:"snapshot_id" gorm:"primaryKey"`
	Owner            string `json:"-" gorm:"index"`
	TenantID         string `json:"tenant_id,omitempty"`
	Dataset          string `json:"dataset"`
	SchemaVersion    string `json:"schema_version"`
	Generation       string `json:"generation,omitempty"`
	SourceWatermark  int64  `json:"source_watermark,omitempty"`
	AppliedWatermark int64  `json:"applied_watermark,omitempty"`
	SpaceScope       string `json:"space_scope,omitempty"`
	Fields           string `json:"fields"`
	Rows             int64  `json:"rows"`
	ContentSHA256    string `json:"content_sha256,omitempty"`
	// DataKey points at the generated export file inside the deployment
	// state directory; it is never a caller-supplied path.
	// KeyID/Fingerprint carry the creation idempotency scoped to the
	// owner: the unique index is (owner, key_id) and a same-key replay
	// with a different business fingerprint is refused.
	DataKey     string    `json:"-"`
	KeyID       string    `json:"-" gorm:"uniqueIndex:analytics_snapshot_key;index:analytics_snapshot_owner_key,priority:2"`
	Fingerprint string    `json:"-"`
	Status      string    `json:"status"` // building|ready|expired|failed
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AnalyticsSnapshot) TableName() string { return "analytics_snapshots" }
