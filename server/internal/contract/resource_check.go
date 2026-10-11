package contract

import "time"

const ResourceIntegrityChecker = "utf8-integrity-v1"
const ResourceIntegrityScope = "fixed_bytes_sha256_utf8_no_nul"

// Identity is descriptor + revision + the server-allowlisted checker. A lookup
// of that identity never executes a new check.
type ResourceCheckRequest struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CheckerVersion   string `json:"checker_version" jsonschema:"enum=utf8-integrity-v1"`
}

// Passed is historical encoding/integrity evidence, NOT safety, skill format
// conformance, installation permission or a live storage health check.
type ResourceIntegrityReceipt struct {
	CheckID            string    `json:"check_id"`
	DescriptorID       string    `json:"descriptor_id"`
	DescriptorRevision uint64    `json:"descriptor_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CheckerVersion     string    `json:"checker_version" jsonschema:"enum=utf8-integrity-v1"`
	CheckScope         string    `json:"check_scope" jsonschema:"enum=fixed_bytes_sha256_utf8_no_nul"`
	State              string    `json:"state" jsonschema:"enum=passed,enum=failed"`
	CheckedAt          time.Time `json:"checked_at"`
}
