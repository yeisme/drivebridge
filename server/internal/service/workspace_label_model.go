package service

import "time"

// The catalog CAS covers stable dictionary identity and presentation only.
// FileLabelAssignment remains the sole truth for labels assigned to files.
type WorkspaceLabelCatalog struct {
	SpaceID string `gorm:"primaryKey"`
	Namespace string `gorm:"primaryKey"`
	Owner string
	TenantID string
	Revision uint64
	UpdatedAt time.Time
}

type WorkspaceLabelDefinition struct {
	ID string `gorm:"primaryKey"`
	SpaceID string `gorm:"uniqueIndex:workspace_label_identity"`
	Namespace string `gorm:"uniqueIndex:workspace_label_identity"`
	Key string `gorm:"uniqueIndex:workspace_label_identity"`
	Owner string
	TenantID string
	DisplayName string
	Group string
	Revision uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}
