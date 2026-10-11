package service

import "time"

// WorkspaceFileLinkProjection is rebuildable evidence from one fixed version.
// TargetRef is a reference, never permission or a cached target title/body.
// Readers must authorize the target separately before resolving its metadata.
type WorkspaceFileLinkProjection struct {
	ID        string `gorm:"primaryKey"`
	Owner     string `gorm:"not null;index:workspace_link_scope"`
	TenantID  string `gorm:"not null;index:workspace_link_scope"`
	SpaceID   string `gorm:"not null;index:workspace_link_scope"`
	FileID    string `gorm:"not null;uniqueIndex:workspace_link_identity"`
	VersionID string `gorm:"not null;uniqueIndex:workspace_link_identity"`
	TargetRef string `gorm:"not null;uniqueIndex:workspace_link_identity"`
	Anchor    string `gorm:"not null;uniqueIndex:workspace_link_identity"`
	CreatedAt time.Time
}

// An explicit generation distinguishes a parsed document with zero links from
// a version not indexed yet. Rebuilds replace only derived rows, never content.
type WorkspaceFileLinkIndex struct {
	ID        string `gorm:"primaryKey"`
	Owner     string `gorm:"not null;index"`
	TenantID  string `gorm:"not null"`
	SpaceID   string `gorm:"not null"`
	FileID    string `gorm:"not null;uniqueIndex:workspace_link_index_identity"`
	VersionID string `gorm:"not null;uniqueIndex:workspace_link_index_identity"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
