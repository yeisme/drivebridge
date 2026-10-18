// Package personalstate owns personal target bindings and local reading state.
// It stores references, never target content or target access rights.
package personalstate

import "time"

// Favorite preserves the existing server schema. Local reading state uses its
// own database/table; extracting this type does not migrate cloud favorites.
type Favorite struct {
	ID         string `gorm:"primaryKey"`
	Owner      string `gorm:"not null;uniqueIndex:workspace_favorite_identity"`
	TenantID   string `gorm:"not null;uniqueIndex:workspace_favorite_identity"`
	SpaceID    string `gorm:"not null;uniqueIndex:workspace_favorite_identity"`
	TargetKind string `gorm:"not null;uniqueIndex:workspace_favorite_identity;check:workspace_favorite_kind,target_kind IN ('file','directory','tag','collection')"`
	TargetRef  string `gorm:"not null;uniqueIndex:workspace_favorite_identity"`
	Revision   uint64 `gorm:"not null"`
	// Deleted is a binding tombstone: revisions never reset on remove/re-add.
	Deleted   bool `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Favorite) TableName() string { return "workspace_favorites" }

// FavoriteCreate identifies a logical target, never its mutable head version.
type FavoriteCreate struct {
	SpaceID          string `json:"space_id"`
	TargetKind       string `json:"target_kind"`
	TargetRef        string `json:"target_ref"`
	ExpectedRevision uint64 `json:"expected_revision"`
}

type FavoriteDelete struct {
	ExpectedRevision uint64 `json:"expected_revision"`
}
