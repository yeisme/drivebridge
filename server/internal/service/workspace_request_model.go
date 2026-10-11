package service

import "time"

// WorkspaceRequest stores durable, secret-free idempotency results. The actor
// is always derived from the authenticated principal, never request metadata.
type WorkspaceRequest struct {
	ID                   string `gorm:"primaryKey"`
	Owner                string `gorm:"not null;uniqueIndex:workspace_request_identity"`
	TenantID             string `gorm:"not null;uniqueIndex:workspace_request_identity"`
	ActorID              string `gorm:"not null;uniqueIndex:workspace_request_identity"`
	Action               string `gorm:"not null;uniqueIndex:workspace_request_identity"`
	RequestKey           string `gorm:"not null;uniqueIndex:workspace_request_identity"`
	RequestDigest        string `gorm:"not null"`
	ResourceID           string `gorm:"not null"`
	ResultRevision       uint64 `gorm:"not null"`
	CredentialGeneration uint64 `gorm:"not null;default:0"`
	CreatedAt            time.Time
}
