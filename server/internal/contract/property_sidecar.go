package contract

import (
	"encoding/json"
	"time"
)

type PropertySidecarPatch struct {
	BaseVersionID      string                     `json:"base_version_id" jsonschema:"pattern=^[a-f0-9]{48}$"`
	DefinitionRevision uint64                     `json:"definition_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	ExpectedRevision   uint64                     `json:"expected_revision" jsonschema:"minimum=0,maximum=9007199254740990"`
	Updates            map[string]json.RawMessage `json:"updates"`
	RemoveKeys         []string                   `json:"remove_keys" jsonschema:"maxItems=200"`
}

type PropertySidecarLookup struct {
	VersionID      string `json:"version_id" jsonschema:"pattern=^[a-f0-9]{48}$"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=200"`
}

type PropertySidecarView struct {
	FileID             string                     `json:"file_id"`
	VersionID          string                     `json:"version_id"`
	DefinitionRevision uint64                     `json:"definition_revision" jsonschema:"minimum=0,maximum=9007199254740991"`
	Revision           uint64                     `json:"revision" jsonschema:"minimum=0,maximum=9007199254740991"`
	StorageKind        string                     `json:"storage_kind" jsonschema:"enum=sidecar"`
	Values             map[string]json.RawMessage `json:"values"`
	// Unknown stored keys are retained but never interpreted or rewritten.
	UnknownKeys []string `json:"unknown_keys" jsonschema:"maxItems=200"`
}

type PropertySidecarReceipt struct {
	FileID         string    `json:"file_id"`
	VersionID      string    `json:"version_id"`
	RequestDigest  string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64    `json:"result_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CommittedAt    time.Time `json:"committed_at"`
}
