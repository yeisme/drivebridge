package contract

// ArchiveSubmissionView exposes the original operation even before its target
// version is published. A missing target ref is not a successful archive.
type ArchiveSubmissionView struct {
	OperationRef string  `json:"operation_ref"`
	FileRef      *string `json:"file_ref" jsonschema:"nullable"`
}
