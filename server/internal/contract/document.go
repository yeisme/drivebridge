package contract

const DocumentMaxBytes = 2 << 20

// Document editing is a human-only projection of the existing version upload.
// Empty text is valid; identity and body presence are checked at the boundary.
type SaveDocumentRequest struct {
	BaseVersionID string `json:"base_version_id" jsonschema:"minLength=1"`
	Text          string `json:"text" jsonschema:"maxLength=2097152"`
}

type DocumentCapability struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Editable  bool   `json:"editable"`
	Reason    string `json:"reason"`
	MaxBytes  int    `json:"max_bytes"`
}

type DocumentSaveView struct {
	FileID      string `json:"file_id"`
	VersionID   string `json:"version_id"`
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
	FileRef     string `json:"file_ref"`
}

type DocumentLookupView struct {
	Found bool              `json:"found"`
	Save  *DocumentSaveView `json:"save" jsonschema:"nullable"`
}
