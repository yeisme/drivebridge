package contract

import (
	"time"

	"github.com/yeisme/drivebridge-server/pkg/personalstate"
)

type FavoriteCreate = personalstate.FavoriteCreate
type FavoriteDelete = personalstate.FavoriteDelete

type FavoriteView struct {
	FavoriteID string `json:"favorite_id"`
	SpaceID    string `json:"space_id"`
	TargetKind string `json:"target_kind"`
	TargetRef  string `json:"target_ref"`
	Revision   uint64 `json:"revision"`
	Deleted    bool   `json:"deleted"`
}

type FavoriteList = Page[FavoriteView]

type FavoriteReceipt struct {
	FavoriteID     string    `json:"favorite_id"`
	SpaceID        string    `json:"space_id"`
	ResultRevision uint64    `json:"result_revision"`
	Deleted        bool      `json:"deleted"`
	RequestDigest  string    `json:"request_digest"`
	CommittedAt    time.Time `json:"committed_at"`
}

type FavoriteLookup struct {
	SpaceID        string `json:"space_id"`
	Action         string `json:"action" jsonschema:"enum=create,enum=delete"`
	IdempotencyKey string `json:"idempotency_key"`
}

type FileLinkRebuild struct {
	VersionID string `json:"version_id"`
}

type FileLinkView struct {
	FileRef    string `json:"file_ref"`
	TargetRef  string `json:"target_ref"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Anchor     string `json:"anchor"`
	Stale      bool   `json:"stale"`
}

type FileLinkList struct {
	FileID     string         `json:"file_id"`
	VersionID  string         `json:"version_id"`
	Indexed    bool           `json:"indexed"`
	Stale      bool           `json:"stale"`
	Items      []FileLinkView `json:"items"`
	NextCursor string         `json:"next_cursor"`
	ObservedAt time.Time      `json:"observed_at"`
	Total      *uint64        `json:"total" jsonschema:"nullable"`
	TotalKind  string         `json:"total_kind" jsonschema:"enum=exact,enum=unknown"`
}

type FileLinkReceipt struct {
	FileID        string    `json:"file_id"`
	VersionID     string    `json:"version_id"`
	RequestDigest string    `json:"request_digest"`
	CommittedAt   time.Time `json:"committed_at"`
}

type FileLinkLookup struct {
	VersionID      string `json:"version_id"`
	IdempotencyKey string `json:"idempotency_key"`
}
