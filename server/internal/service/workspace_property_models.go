package service

import "time"

// WorkspacePropertyCatalog fences whole-dictionary edits, including empty lists.
// Existing rows without a catalog require explicit migration, never a guessed revision.
type WorkspacePropertyCatalog struct {
	SpaceID   string `gorm:"primaryKey"`
	Owner     string `gorm:"not null"`
	TenantID  string `gorm:"not null"`
	Revision  uint64 `gorm:"not null"`
	UpdatedAt time.Time
}

// Definitions are space-owned metadata. Key/type migrations belong to an
// explicit application workflow, not an implicit column rewrite.
type WorkspacePropertyDefinition struct {
	ID          string `gorm:"primaryKey"`
	Owner       string `gorm:"not null;index:workspace_property_scope"`
	TenantID    string `gorm:"not null;index:workspace_property_scope"`
	SpaceID     string `gorm:"not null;uniqueIndex:workspace_property_key;index:workspace_property_scope"`
	Key         string `gorm:"not null;uniqueIndex:workspace_property_key"`
	DisplayName string `gorm:"not null"`
	Group       string
	Type        string `gorm:"not null;check:workspace_property_type,type IN ('text','number','date','checkbox','list','reference')"`
	Revision    uint64 `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkspacePropertySidecar owns human metadata for one fixed binary version.
// It is not a Markdown/frontmatter projection and never writes provider bytes.
type WorkspacePropertySidecar struct {
	ID                 string `gorm:"primaryKey"`
	Owner              string `gorm:"not null"`
	TenantID           string `gorm:"not null"`
	SpaceID            string `gorm:"not null"`
	FileID             string `gorm:"not null;uniqueIndex:workspace_sidecar_version"`
	VersionID          string `gorm:"not null;uniqueIndex:workspace_sidecar_version"`
	Revision           uint64 `gorm:"not null"`
	DefinitionRevision uint64 `gorm:"not null"`
	ValuesJSON         string `gorm:"not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Projections never replace File/Version authority. Values are JSON text so
// unknown fields/types can be retained without lossy coercion by the reader.
type WorkspaceFilePropertyProjection struct {
	ID                 string `gorm:"primaryKey"`
	Owner              string `gorm:"not null;index:workspace_property_projection_scope"`
	TenantID           string `gorm:"not null;index:workspace_property_projection_scope"`
	SpaceID            string `gorm:"not null;index:workspace_property_projection_scope"`
	FileID             string `gorm:"not null;uniqueIndex:workspace_property_projection_key"`
	VersionID          string `gorm:"not null;uniqueIndex:workspace_property_projection_key"`
	DefinitionRevision uint64 `gorm:"not null;uniqueIndex:workspace_property_projection_key"`
	Key                string `gorm:"not null;uniqueIndex:workspace_property_projection_key"`
	StorageKind        string `gorm:"not null;check:workspace_property_storage,storage_kind IN ('frontmatter','sidecar')"`
	ValueJSON          string `gorm:"not null"`
	Revision           uint64 `gorm:"not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
