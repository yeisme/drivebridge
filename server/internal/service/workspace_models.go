package service

import (
	"gorm.io/gorm"
	"time"
)

// WorkspaceGrant is separate from legacy Agent expiry semantics. No credential
// is activated merely by inserting this record; application gates are required.
type WorkspaceGrant struct {
	ID                   string `gorm:"primaryKey"`
	AgentID              string `gorm:"not null;uniqueIndex"`
	Owner                string `gorm:"not null;index:workspace_grant_scope"`
	TenantID             string `gorm:"not null;index:workspace_grant_scope"`
	SpaceID              string `gorm:"not null;index:workspace_grant_scope"`
	ConnectionID         string `gorm:"not null"`
	Protocol             string `gorm:"not null;default:mcp"`
	Prefix               string
	Actions              string `gorm:"not null"`
	RestrictionProfile   string `gorm:"not null"`
	ExpiryMode           string `gorm:"not null;check:workspace_expiry_valid,(expiry_mode = 'never' AND expires_at IS NULL) OR (expiry_mode = 'duration' AND expires_at IS NOT NULL)"`
	ExpiresAt            *time.Time
	IssuedAt             time.Time `gorm:"not null"`
	ExpiryChangedAt      *time.Time
	Disabled             bool   `gorm:"not null;default:false"`
	Revoked              bool   `gorm:"not null;default:false"`
	Revision             uint64 `gorm:"not null"`
	CredentialGeneration uint64 `gorm:"not null"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Group names are unique inside a human's tenant scope, never globally.
type WorkspaceAgentGroup struct {
	ID             string `gorm:"primaryKey"`
	Owner          string `gorm:"not null;uniqueIndex:workspace_group_name"`
	TenantID       string `gorm:"not null;uniqueIndex:workspace_group_name"`
	NormalizedName string `gorm:"not null;uniqueIndex:workspace_group_name"`
	Name           string `gorm:"not null"`
	Revision       uint64 `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type WorkspaceAgentOrganization struct {
	ID        string  `gorm:"primaryKey"`
	Owner     string  `gorm:"not null;uniqueIndex:workspace_agent_org"`
	TenantID  string  `gorm:"not null;uniqueIndex:workspace_agent_org"`
	AgentID   string  `gorm:"not null;uniqueIndex:workspace_agent_org"`
	GroupID   *string `gorm:"index"`
	Pinned    bool    `gorm:"not null;default:false"`
	Revision  uint64  `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// This migration is additive. Its caller owns the database lifecycle.
func migrateWorkspaceModels(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&WorkspaceLabelCatalog{}, &WorkspaceLabelDefinition{}); err != nil {
			return err
		}
		return tx.AutoMigrate(&WorkspaceGrant{}, &WorkspaceAgentGroup{}, &WorkspaceAgentOrganization{}, &WorkspaceSubmission{}, &WorkspaceSubmissionDecision{}, &WorkspaceRequest{}, &WorkspaceCredentialHistory{}, &WorkspaceLabelSuggestion{}, &WorkspaceSuggestionDecision{}, &WorkspacePropertyCatalog{}, &WorkspacePropertyDefinition{}, &WorkspacePropertySidecar{}, &WorkspaceFilePropertyProjection{}, &WorkspaceFileLinkProjection{}, &WorkspaceFileLinkIndex{}, &WorkspaceResourceDescriptor{}, &WorkspaceResourceCheck{}, &WorkspaceFavorite{}, &WorkspaceInvocation{}, &NotebookPageMetadata{}, &NotebookCreationIntent{})
	})
}
