package service

import "time"

// CloudRoot binds one externally observable scope (a drive, a folder or an
// object prefix) inside a cloud connection. Roots are created by the owning
// human through the application service; scans, cursors and observations
// never cross a root, and narrowing or rebinding a root bumps Generation so
// every outstanding cursor is invalidated at once.
type CloudRoot struct {
	ID           string `json:"root_id" gorm:"primaryKey"`
	Owner        string `json:"-" gorm:"index"`
	TenantID     string `json:"tenant_id,omitempty"`
	ConnectionID string `json:"connection_id" gorm:"index"`
	Provider     string `json:"provider"` // onedrive|gdrive|s3
	// RootRef is the provider-native root identifier ("root" item id, drive
	// folder id, or a validated object prefix). It is resolved against the
	// current authorization on every use; it never accepts caller URLs.
	RootRef    string `json:"root_ref"`
	Name       string `json:"name,omitempty"`
	Status     string `json:"status"` // bound|observed|drifted|disconnected
	Generation uint64 `json:"generation"`
	// DeltaCursor is the server-held provider change cursor for roots whose
	// provider exposes a native change stream. It is opaque to callers.
	DeltaCursor        string     `json:"-"`
	LastScanAt         *time.Time `json:"last_scan_at,omitempty"`
	LastScanCoverage   string     `json:"last_scan_coverage,omitempty"` // complete|partial|unknown
	LastScanObserved   int64      `json:"last_scan_observed,omitempty"`
	LastScanTombstoned int64      `json:"last_scan_tombstoned,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (CloudRoot) TableName() string { return "cloud_roots" }

// ExternalEntry is one observed provider item inside a CloudRoot. It is an
// external observation, never a managed File/Version: ObservedRevision is the
// provider's own revision fact (ETag/revision/VersionId) and only an explicit
// verified import fills the managed reference columns. Provider identity is
// the (root, provider item) pair: pages that repeat an item after drift
// update the same row instead of creating duplicates. Absence from one
// incomplete page is never deletion evidence; only a provider-confirmed
// removal or a complete-scan absence writes TombstonedAt.
type ExternalEntry struct {
	ID               string     `gorm:"primaryKey"`
	RootID           string     `json:"root_id" gorm:"uniqueIndex:external_entry_identity;index"`
	ProviderItemID   string     `json:"provider_item_id" gorm:"uniqueIndex:external_entry_identity"`
	Owner            string     `json:"-"`
	ConnectionID     string     `json:"connection_id"`
	ParentRef        string     `json:"parent_ref,omitempty"`
	Path             string     `json:"path"`
	Kind             string     `json:"kind"` // file|directory|native_document
	Size             int64      `json:"size_bytes" gorm:"column:size_bytes"`
	MIME             string     `json:"mime_type,omitempty" gorm:"column:mime_type"`
	ModifiedAt       *time.Time `json:"modified_at,omitempty"`
	ObservedRevision string     `json:"observed_revision"`
	// VersionStrength records what the observation can honestly promise:
	// observed_only (provider revision seen, no verified bytes) or
	// sha256_verified (a completed import verified this exact revision's
	// bytes). ETag/mtime are change detection only, never digests.
	VersionStrength string `json:"version_strength"`
	// ContentDigest carries the verified SHA-256 when strength is
	// sha256_verified; empty otherwise.
	ContentDigest string `json:"content_sha256,omitempty" gorm:"column:content_sha256"`
	// appear only after an explicit verified import; observations alone
	// never create them.
	ManagedFileID    string     `json:"managed_file_id,omitempty"`
	ManagedVersionID string     `json:"managed_version_id,omitempty"`
	TombstonedAt     *time.Time `json:"tombstoned_at,omitempty"`
	ObservedAt       time.Time  `json:"observed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (ExternalEntry) TableName() string { return "external_entries" }
