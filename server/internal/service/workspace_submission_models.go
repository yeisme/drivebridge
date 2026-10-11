package service

import "time"

// WorkspaceSubmission references existing file bytes and operations; it does
// not create another blob store. Creation identity is scoped to the grant.
type WorkspaceSubmission struct {
	ID                 string `gorm:"primaryKey"`
	Owner              string `gorm:"not null;uniqueIndex:workspace_submission_key;index:workspace_submission_scope"`
	TenantID           string `gorm:"not null;uniqueIndex:workspace_submission_key;index:workspace_submission_scope"`
	GrantID            string `gorm:"not null;uniqueIndex:workspace_submission_key"`
	CreationKey        string `gorm:"not null;uniqueIndex:workspace_submission_key"`
	RequestDigest      string `gorm:"not null"`
	AgentID            string `gorm:"not null"`
	SpaceID            string `gorm:"not null;index:workspace_submission_scope"`
	UploadOperationID  string `gorm:"not null;uniqueIndex"`
	FileID             *string
	VersionID          *string
	State              string `gorm:"not null;index:workspace_submission_scope"`
	Revision           uint64 `gorm:"not null"`
	ArchiveOperationID *string
	ArchivedFileRef    *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Decisions are append-only revision facts; acceptance does not archive bytes.
type WorkspaceSubmissionDecision struct {
	ID           string `gorm:"primaryKey"`
	SubmissionID string `gorm:"not null;uniqueIndex:workspace_submission_decision"`
	Revision     uint64 `gorm:"not null;uniqueIndex:workspace_submission_decision"`
	HumanID      string `gorm:"not null"`
	VersionID    string `gorm:"not null"`
	Decision     string `gorm:"not null"`
	Reason       string
	CreatedAt    time.Time
}
