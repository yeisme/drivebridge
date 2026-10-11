package contract

import "time"

// SubmissionView keeps submission progress distinct from its upload and archive
// operations. Optional references appear only once the owner has produced them.
type SubmissionView struct {
	// Display-only additions. File metadata comes from the original upload, not
	// the mutable head. AgentName is a current owner-assigned name, not a claim.
	FileName            string                  `json:"file_name,omitempty"`
	MIMEType            string                  `json:"mime_type,omitempty"`
	SizeBytes           *int64                  `json:"size_bytes,omitempty" jsonschema:"minimum=0"`
	AgentName           string                  `json:"agent_name,omitempty"`
	SubmissionID        string                  `json:"submission_id"`
	Revision            uint64                  `json:"revision"`
	SpaceID             string                  `json:"space_id"`
	AgentID             string                  `json:"agent_id,omitempty"`
	GrantID             string                  `json:"grant_id,omitempty"`
	SourceKind          string                  `json:"source_kind" jsonschema:"enum=agent,enum=human_import"`
	SourceClaim         string                  `json:"source_claim,omitempty"`
	FileRef             string                  `json:"file_ref,omitempty"`
	VersionID           string                  `json:"version_id,omitempty"`
	UploadOperationRef  string                  `json:"upload_operation_ref"`
	State               string                  `json:"state" jsonschema:"enum=uploading,enum=submitted,enum=accepted,enum=rejected,enum=archiving,enum=archived"`
	Decision            *SubmissionDecisionView `json:"decision,omitempty"`
	ArchiveOperationRef string                  `json:"archive_operation_ref,omitempty"`
	ArchivedFileRef     string                  `json:"archived_file_ref,omitempty"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
}

type SubmissionDecisionView struct {
	VersionID string    `json:"version_id"`
	Decision  string    `json:"decision" jsonschema:"enum=accept,enum=reject"`
	Reason    string    `json:"reason,omitempty"`
	DecidedAt time.Time `json:"decided_at"`
}

type SubmissionDecisionRequest struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	VersionID        string `json:"version_id" jsonschema:"minLength=1"`
	Decision         string `json:"decision" jsonschema:"enum=accept,enum=reject"`
	Reason           string `json:"reason,omitempty" jsonschema:"maxLength=500"`
}

type ArchiveSubmissionRequest struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	VersionID        string `json:"version_id" jsonschema:"minLength=1"`
	TargetSpaceID    string `json:"target_space_id" jsonschema:"minLength=1"`
	TargetPrefix     string `json:"target_prefix"`
}
