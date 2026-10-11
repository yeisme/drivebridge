package contract

type NotebookPage struct {
	FileID       string `json:"file_id"`
	SpaceID      string `json:"space_id"`
	VersionID    string `json:"version_id"`
	Path         string `json:"path"`
	ParentFileID string `json:"parent_file_id"`
	Icon         string `json:"icon"`
	Revision     uint64 `json:"revision"`
}

type NotebookPagePatch struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	ParentFileID     string `json:"parent_file_id"`
	Icon             string `json:"icon" jsonschema:"enum=file,enum=book,enum=calendar,enum=idea,enum=project"`
}

type NotebookPageReceipt struct {
	FileID         string `json:"file_id"`
	ResultRevision uint64 `json:"result_revision"`
}

type NotebookCreate struct {
	SpaceID      string `json:"space_id" jsonschema:"minLength=1"`
	Title        string `json:"title" jsonschema:"maxLength=120"`
	ParentFileID string `json:"parent_file_id"`
	Icon         string `json:"icon" jsonschema:"enum=file,enum=book,enum=calendar,enum=idea,enum=project"`
	DailyDate    string `json:"daily_date"`
}

type NotebookCreateLookup struct {
	IdempotencyKey string `json:"idempotency_key"`
	DailyDate      string `json:"daily_date"`
}

type NotebookCreation struct {
	Found       bool          `json:"found"`
	Status      string        `json:"status"`
	OperationID string        `json:"operation_id"`
	Page        *NotebookPage `json:"page" jsonschema:"nullable"`
}
