package contract

import "time"

// Sections are independent read observations, not an atomic cross-section snapshot.
type HomeSection struct {
	State      string    `json:"state" jsonschema:"enum=ready,enum=partial,enum=unavailable"`
	Reason     string    `json:"reason"`
	ObservedAt time.Time `json:"observed_at"`
}
type HomeFileRef struct {
	FileID    string    `json:"file_id"`
	VersionID string    `json:"version_id"`
	Ref       string    `json:"ref"`
	SpaceID   string    `json:"space_id"`
	Path      string    `json:"path"`
	MIMEType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes" jsonschema:"minimum=0"`
	UpdatedAt time.Time `json:"updated_at"`
}
type HomeCollection struct {
	CollectionID string `json:"collection_id"`
	Name         string `json:"name"`
	Kind         string `json:"kind" jsonschema:"enum=manual,enum=rule"`
	Revision     uint64 `json:"revision"`
}
type HomePending struct {
	// These are submission states only, not a global count of all tasks.
	Submitted *int64 `json:"submitted" jsonschema:"nullable,minimum=0"`
	Accepted  *int64 `json:"accepted" jsonschema:"nullable,minimum=0"`
}
type HomeUsage struct {
	UsedBytes     *int64 `json:"used_bytes" jsonschema:"nullable,minimum=0"`
	ReservedBytes *int64 `json:"reserved_bytes" jsonschema:"nullable,minimum=0"`
	QuotaBytes    *int64 `json:"quota_bytes" jsonschema:"nullable,minimum=0"`
}
type HomeSections struct {
	RecentFiles   HomeSection `json:"recent_files"`
	Collections   HomeSection `json:"collections"`
	PendingCounts HomeSection `json:"pending_counts"`
	Usage         HomeSection `json:"usage"`
}
type WorkspaceHome struct {
	SpaceID       string           `json:"space_id"`
	RecentFiles   []HomeFileRef    `json:"recent_files"`
	Collections   []HomeCollection `json:"collections"`
	PendingCounts HomePending      `json:"pending_counts"`
	Usage         HomeUsage        `json:"usage"`
	Sections      HomeSections     `json:"sections"`
	ObservedAt    time.Time        `json:"observed_at"`
}
