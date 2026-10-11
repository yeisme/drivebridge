package service

import "time"

// WorkspaceLabelSuggestion is a proposal bound to one authenticated identity
// and fixed version. It is not a FileLabelAssignment and cannot grant access.
type WorkspaceLabelSuggestion struct {
	ID            string   `gorm:"primaryKey"`
	Owner         string   `gorm:"not null;uniqueIndex:workspace_suggestion_key;index:workspace_suggestion_scope"`
	TenantID      string   `gorm:"not null;uniqueIndex:workspace_suggestion_key;index:workspace_suggestion_scope"`
	AgentID       string   `gorm:"not null;uniqueIndex:workspace_suggestion_key"`
	GrantID       string   `gorm:"not null"`
	FileID        string   `gorm:"not null;uniqueIndex:workspace_suggestion_key;index:workspace_suggestion_scope"`
	VersionID     string   `gorm:"not null;uniqueIndex:workspace_suggestion_key"`
	CreationKey   string   `gorm:"not null;uniqueIndex:workspace_suggestion_key"`
	RequestDigest string   `gorm:"not null"`
	Labels        []string `gorm:"serializer:json;not null"`
	State         string   `gorm:"not null;default:pending"`
	Revision      uint64   `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type WorkspaceSuggestionDecision struct {
	ID             string   `gorm:"primaryKey"`
	SuggestionID   string   `gorm:"not null;uniqueIndex:workspace_suggestion_decision"`
	Revision       uint64   `gorm:"not null;uniqueIndex:workspace_suggestion_decision"`
	HumanID        string   `gorm:"not null"`
	VersionID      string   `gorm:"not null"`
	Decision       string   `gorm:"not null"`
	AcceptedLabels []string `gorm:"serializer:json"`
	CreatedAt      time.Time
}
