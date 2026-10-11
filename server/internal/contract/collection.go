package contract

import "time"

// CollectionCreationReceipt reports the original creation, never current members.
type CollectionCreationReceipt struct {
	CollectionID   string    `json:"collection_id"`
	SpaceID        string    `json:"space_id"`
	RequestDigest  string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64    `json:"result_revision" jsonschema:"minimum=1,maximum=1"`
	CommittedAt    time.Time `json:"committed_at"`
}
