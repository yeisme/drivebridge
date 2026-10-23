package contract

// DirectoryCreate is the human workspace request for creating one managed
// directory. The idempotency key is carried by the HTTP header.
type DirectoryCreate struct {
	SpaceID string `json:"space_id" jsonschema:"minLength=1"`
	Path    string `json:"path" jsonschema:"minLength=1"`
}

type DirectoryCreateLookup struct {
	SpaceID        string `json:"space_id" jsonschema:"minLength=1"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1"`
}

type DirectoryView struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
	Path    string `json:"path"`
}

type DirectoryCreation struct {
	Found     bool           `json:"found"`
	Status    string         `json:"status"`
	Directory *DirectoryView `json:"directory" jsonschema:"nullable"`
}
