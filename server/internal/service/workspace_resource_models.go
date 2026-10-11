package service

import "time"

// Resource descriptors reference immutable owner artifacts; no executable
// payload, provider credentials or installation state belongs here.
type WorkspaceResourceDescriptor struct {
	ID          string `gorm:"primaryKey"`
	Owner       string `gorm:"not null;index:workspace_resource_scope"`
	TenantID    string `gorm:"not null;index:workspace_resource_scope"`
	SpaceID     string `gorm:"not null;index:workspace_resource_scope"`
	FileID      *string
	VersionID   *string
	SnapshotID  *string `gorm:"check:workspace_resource_target,(file_id IS NOT NULL AND version_id IS NOT NULL AND snapshot_id IS NULL) OR (file_id IS NULL AND version_id IS NULL AND snapshot_id IS NOT NULL)"`
	Kind        string  `gorm:"not null;check:workspace_resource_kind,kind IN ('skill','template','reference_pack')"`
	DisplayName string  `gorm:"not null"`
	Revision    uint64  `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Checks are immutable facts about a descriptor revision, not permission to
// execute its contents. A new checker version creates a distinct fact.
type WorkspaceResourceCheck struct {
	ID                 string `gorm:"primaryKey"`
	Owner              string `gorm:"not null;index:workspace_resource_check_scope"`
	TenantID           string `gorm:"not null;index:workspace_resource_check_scope"`
	DescriptorID       string `gorm:"not null;uniqueIndex:workspace_resource_check_identity"`
	DescriptorRevision uint64 `gorm:"not null;uniqueIndex:workspace_resource_check_identity"`
	CheckerVersion     string `gorm:"not null;uniqueIndex:workspace_resource_check_identity"`
	State              string `gorm:"not null;check:workspace_resource_check_state,state IN ('unknown','passed','failed')"`
	// Empty on pre-checker rows; never treated as proof of a current target.
	TargetDigest string `gorm:"not null;default:''"`
	CreatedAt    time.Time
}
