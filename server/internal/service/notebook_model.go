package service

import "time"

// Page relationships belong to files, never to their paths or Markdown bytes.
type NotebookPageMetadata struct {
	FileID       string `gorm:"primaryKey"`
	Owner        string `gorm:"not null;index:notebook_page_scope"`
	TenantID     string `gorm:"not null;index:notebook_page_scope"`
	SpaceID      string `gorm:"not null;index:notebook_page_scope"`
	ParentFileID string `gorm:"index"`
	Icon         string
	Revision     uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Durable intent links the existing upload operation to its metadata. No
// credentials or document body are duplicated in this journal.
type NotebookCreationIntent struct {
	ID           string `gorm:"primaryKey"`
	Identity     string `gorm:"not null;uniqueIndex"`
	Owner        string
	TenantID     string
	SpaceID      string
	Digest       string
	Title        string
	Path         string
	ParentFileID string
	Icon         string
	CreatedAt    time.Time
}
