package contract

// LabelSuggestionView records a proposal, never a mutation of manual labels.
type LabelSuggestionView struct {
	SuggestionID string   `json:"suggestion_id"`
	Revision     uint64   `json:"revision"`
	FileID       string   `json:"file_id"`
	VersionID    string   `json:"version_id"`
	AgentID      string   `json:"agent_id"`
	Labels       []string `json:"labels"`
	CreationKey  string   `json:"creation_key"`
	State        string   `json:"state" jsonschema:"enum=pending,enum=accepted,enum=rejected"`
}

type SuggestionDecisionRequest struct {
	ExpectedRevision uint64   `json:"expected_revision"`
	BaseVersionID    string   `json:"base_version_id" jsonschema:"minLength=1"`
	Decision         string   `json:"decision" jsonschema:"enum=accept,enum=reject"`
	Labels           []string `json:"labels,omitempty" jsonschema:"maxItems=100"`
}
