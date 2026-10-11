package contract

import "time"

// SnapshotIndex is a bounded observation of headers, not creation receipts or
// a claim that any historical member is currently readable.
type SnapshotIndex struct {
	CollectionID string              `json:"collection_id"`
	Items        []SnapshotIndexItem `json:"items" jsonschema:"maxItems=1000"`
	ObservedAt   time.Time           `json:"observed_at"`
}

type SnapshotIndexItem struct {
	SnapshotID         string    `json:"snapshot_id" jsonschema:"pattern=^[a-f0-9]{48}$"`
	CollectionRevision uint64    `json:"collection_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CreatedAt          time.Time `json:"created_at"`
}
