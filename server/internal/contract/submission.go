package contract

// CreateSubmissionRequest cannot choose a destination or impersonate a grant.
// The application derives staging identity from the authenticated principal.
type CreateSubmissionRequest struct {
	Name      string `json:"name" jsonschema:"minLength=1,maxLength=255"`
	MIMEType  string `json:"mime_type" jsonschema:"minLength=1"`
	SizeBytes int64  `json:"size_bytes" jsonschema:"minimum=0"`
	SHA256    string `json:"sha256" jsonschema:"pattern=^[a-f0-9]{64}$"`
	Caller    string `json:"caller,omitempty" jsonschema:"maxLength=120"`
	TaskRef   string `json:"task_ref,omitempty" jsonschema:"maxLength=120"`
}

type SubmissionUploadView struct {
	SubmissionID string `json:"submission_id"`
	Revision     uint64 `json:"revision"`
	OperationID  string `json:"operation_id"`
	Purpose      string `json:"purpose" jsonschema:"enum=inbox_staging"`
	PartSize     int64  `json:"part_size" jsonschema:"minimum=1"`
	PartPath     string `json:"part_path"`
}

type IdempotencyLookupRequest struct {
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=200"`
}

type LabelSuggestionRequest struct {
	VersionID string   `json:"version_id" jsonschema:"minLength=1"`
	Labels    []string `json:"labels" jsonschema:"minItems=1,maxItems=100"`
}
