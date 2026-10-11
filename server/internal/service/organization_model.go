package service

import "time"

// FileLabelAssignment is one label in one namespace on one file. The three
// namespaces carry different authority: manual is written only by an
// explicit human action, rule only by a pre-authorized rule, suggested only
// by model output. Manual decisions are never overwritten by re-runs.
type FileLabelAssignment struct {
	ID        string `json:"id" gorm:"primaryKey"`
	Owner     string `json:"-" gorm:"uniqueIndex:file_label_identity"`
	FileID    string `json:"file_id" gorm:"uniqueIndex:file_label_identity;index"`
	Namespace string `json:"namespace" gorm:"uniqueIndex:file_label_identity"` // manual|rule|suggested
	Label     string `json:"label" gorm:"uniqueIndex:file_label_identity"`
	// AuthorKind records who wrote the row (human|rule|model); accepting a
	// suggestion converts it into a manual row with a fresh source record.
	AuthorKind   string    `json:"author_kind"`
	ActorID      string    `json:"-"`
	AcceptedFrom string    `json:"accepted_from,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (FileLabelAssignment) TableName() string { return "file_label_assignments" }

// FileCollection is a virtual organization unit. kind=manual holds explicit
// members that follow the files' current version; kind=rule stores the exact
// versioned conditions plus manual include/exclude overrides. Collections
// never hold Manju subject/asset revisions.
type FileCollection struct {
	ID       string `json:"collection_id" gorm:"primaryKey"`
	Owner    string `json:"-" gorm:"index"`
	TenantID string `json:"tenant_id,omitempty"`
	SpaceID  string `json:"space_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`  // manual|rule
	Scope    string `json:"scope"` // private|team
	// Rule is the canonical JSON of the versioned rule conditions; empty
	// for manual collections.
	Rule         string `json:"rule,omitempty"`
	RuleRevision uint64 `json:"rule_revision,omitempty"`
	// KeyID/Fingerprint carry the creation idempotency: same key plus same
	// business inputs replays the original collection.
	KeyID       string `json:"-"`
	Fingerprint string `json:"-"`
	// Revision is the optimistic counter: member edits and rule changes
	// bump it so stale writers get collection_revision_conflict.
	Revision   uint64     `json:"revision"`
	ComputedAt *time.Time `json:"computed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (FileCollection) TableName() string { return "file_collections" }

// CollectionMember is one explicit member binding of a collection: an
// include or exclude decision on a file id.
type CollectionMember struct {
	ID           string    `gorm:"primaryKey"`
	CollectionID string    `json:"collection_id" gorm:"uniqueIndex:collection_member_identity;index"`
	FileID       string    `json:"file_id" gorm:"uniqueIndex:collection_member_identity"`
	Owner        string    `json:"-"`
	Decision     string    `json:"decision"` // include|exclude
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (CollectionMember) TableName() string { return "collection_members" }

// CollectionSnapshot freezes one collection read: each entry pins the exact
// file/version it captured. Snapshots grant no read permission and never
// expand access; reads re-authorize every entry.
type CollectionSnapshot struct {
	ID             string    `json:"snapshot_id" gorm:"primaryKey"`
	CollectionID   string    `json:"collection_id" gorm:"index"`
	Owner          string    `json:"-" gorm:"index"`
	Revision       uint64    `json:"collection_revision"`
	EntriesTotal   int64     `json:"entries_total"`
	EntriesVisible int64     `json:"entries_visible"`
	CreatedAt      time.Time `json:"created_at"`
}

func (CollectionSnapshot) TableName() string { return "collection_snapshots" }

// CollectionSnapshotEntry is one frozen member of a snapshot.
type CollectionSnapshotEntry struct {
	ID         string    `gorm:"primaryKey"`
	SnapshotID string    `json:"snapshot_id" gorm:"index;uniqueIndex:snapshot_entry_identity"`
	FileID     string    `json:"file_id" gorm:"uniqueIndex:snapshot_entry_identity"`
	VersionID  string    `json:"version_id"`
	Path       string    `json:"path"`
	SHA256     string    `json:"sha256,omitempty"`
	Visible    bool      `json:"visible"`
	CreatedAt  time.Time `json:"created_at"`
}

func (CollectionSnapshotEntry) TableName() string { return "collection_snapshot_entries" }

// SavedFileQuery stores one exact query for later re-execution. It stores
// allowed fields only, never permissions; every read re-authorizes.
type SavedFileQuery struct {
	ID        string    `json:"query_id" gorm:"primaryKey"`
	Owner     string    `json:"-" gorm:"index"`
	Name      string    `json:"name"`
	Query     string    `json:"query"` // canonical JSON of the typed query
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SavedFileQuery) TableName() string { return "saved_file_queries" }
