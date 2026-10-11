package service

import "time"

// FileThread is one discussion thread anchored to a file version. The
// anchor never moves implicitly: a new version of the file gets a new
// thread, and an anchored thread whose original version was cleaned up
// stays readable with an anchor_version_unavailable marker instead of
// silently re-anchoring to the latest version.
type FileThread struct {
	ID         string     `json:"thread_id" gorm:"primaryKey"`
	Owner      string     `json:"-" gorm:"index"`
	TenantID   string     `json:"tenant_id,omitempty"`
	FileID     string     `json:"file_id" gorm:"index"`
	VersionID  string     `json:"version_id,omitempty"` // empty = file-level anchor
	Seq        int64      `json:"seq" gorm:"index"`     // stable per-file ordering
	AuthorID   string     `json:"author_id"`
	AuthorKind string     `json:"author_kind"` // user|agent|bot
	State      string     `json:"state"`       // open|resolved
	Revision   uint64     `json:"revision"`    // optimistic counter for edits
	ResolvedBy string     `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (FileThread) TableName() string { return "file_threads" }

// ThreadMessage is one plain-text contribution (<=10,000 characters). A
// deleted message keeps its audit placeholder row; the body is never
// rendered or interpreted as HTML anywhere.
type ThreadMessage struct {
	ID         string     `json:"message_id" gorm:"primaryKey"`
	ThreadID   string     `json:"thread_id" gorm:"index"`
	Seq        int64      `json:"seq" gorm:"index"`
	AuthorID   string     `json:"author_id"`
	AuthorKind string     `json:"author_kind"`
	Body       string     `json:"body"`
	Revision   uint64     `json:"revision"`
	Deleted    bool       `json:"deleted"`
	DeletedBy  string     `json:"deleted_by,omitempty"`
	EditedAt   *time.Time `json:"edited_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (ThreadMessage) TableName() string { return "thread_messages" }

// PrivateNote is the per-author private note on one file. Only the author
// can read it while they still hold file permission; publishing is an
// explicit copy into a team thread and never mutates this record.
type PrivateNote struct {
	FileID    string    `gorm:"primaryKey"`
	AuthorID  string    `gorm:"primaryKey;index"`
	Body      string    `json:"body"`
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (PrivateNote) TableName() string { return "private_notes" }

// DiscussionRevision is the monotonic per-file sequence behind stable
// keyset ordering of threads and messages.
type DiscussionRevision struct {
	FileID string `gorm:"primaryKey"`
	Next   int64
}

func (DiscussionRevision) TableName() string { return "discussion_revisions" }

const maxThreadBody = 10000

// threadBodyValid enforces plain-text boundaries: no control characters
// beyond tab and newline, and a hard length cap.
func threadBodyValid(body string) bool {
	if body == "" || len(body) > maxThreadBody {
		return false
	}
	for _, r := range body {
		if r < 0x20 && r != '\t' && r != '\n' {
			return false
		}
	}
	return true
}
