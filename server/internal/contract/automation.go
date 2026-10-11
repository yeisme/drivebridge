package contract

import "time"

// Additive, human-only management DTOs. Legacy rule and run shapes are unchanged.
type AutomationManagement struct {
	Available bool `json:"available"`
}
type AutomationChangeRequest struct {
	Action           string `json:"action" jsonschema:"enum=add_label"`
	ActionDetail     string `json:"action_detail" jsonschema:"minLength=1,maxLength=210"`
	Prefix           string `json:"prefix" jsonschema:"maxLength=1024"`
	PolicyID         string `json:"policy_id,omitempty"`
	BudgetItems      int64  `json:"budget_items" jsonschema:"minimum=1,maximum=200"`
	Enabled          bool   `json:"enabled"`
	ClearPause       bool   `json:"clear_pause"`
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=0,maximum=9007199254740991"`
}
type AutomationExecuteRequest struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	PreviewDigest    string `json:"preview_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
}
type AutomationPreviewItem struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
}
type AutomationPreview struct {
	RuleID          string                  `json:"rule_id"`
	SpaceID         string                  `json:"space_id"`
	Revision        uint64                  `json:"revision"`
	Checkpoint      int64                   `json:"checkpoint"`
	Scanned         int                     `json:"scanned"`
	Items           []AutomationPreviewItem `json:"items"`
	Coverage        string                  `json:"coverage" jsonschema:"enum=complete,enum=partial"`
	PreviewDigest   string                  `json:"preview_digest"`
	Executable      bool                    `json:"executable"`
	PolicyMaxBytes  int64                   `json:"policy_max_bytes"`
	PolicyExpiresAt *time.Time              `json:"policy_expires_at" jsonschema:"nullable"`
}
type AutomationReceipt struct {
	RuleID         string    `json:"rule_id"`
	SpaceID        string    `json:"space_id"`
	Action         string    `json:"action" jsonschema:"enum=automation.change,enum=automation.execute"`
	RequestDigest  string    `json:"request_digest"`
	ResultRevision uint64    `json:"result_revision"`
	CommittedAt    time.Time `json:"committed_at"`
}
