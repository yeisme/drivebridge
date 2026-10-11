package contract

import "time"

type SnapshotCreationRequest struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
}

// SnapshotCreationReceipt is a publication fact, not current entry visibility.
type SnapshotCreationReceipt struct {
	CollectionID       string    `json:"collection_id"`
	SnapshotID         string    `json:"snapshot_id"`
	RequestDigest      string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	CollectionRevision uint64    `json:"collection_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CommittedAt        time.Time `json:"committed_at"`
}
