package service

import "time"

// WorkspaceCredentialHistory stores hashes only. A historical hash never
// authenticates: the active Agent token hash remains the credential lookup.
type WorkspaceCredentialHistory struct {
	ID         string `gorm:"primaryKey"`
	GrantID    string `gorm:"not null;uniqueIndex:workspace_credential_generation"`
	Generation uint64 `gorm:"not null;uniqueIndex:workspace_credential_generation"`
	TokenHash  string `json:"-" gorm:"not null;uniqueIndex"`
	CreatedAt  time.Time
	RevokedAt  *time.Time
}
