package contract

import "time"

// PropertyMigrationPreview always covers the complete bounded canonical sidecar
// set in a space. It never interprets Markdown or patches provider bytes.
type PropertyMigrationPreview struct {
	ExpectedDefinitionRevision uint64 `json:"expected_definition_revision" jsonschema:"minimum=1,maximum=9007199254740990"`
	SourceKey                  string `json:"source_key" jsonschema:"pattern=^[a-z][a-z0-9_]*$,maxLength=64"`
	TargetKey                  string `json:"target_key" jsonschema:"pattern=^[a-z][a-z0-9_]*$,maxLength=64"`
	TargetType                 string `json:"target_type" jsonschema:"enum=text,enum=number,enum=date,enum=checkbox,enum=list,enum=reference"`
	ConversionPolicy           string `json:"conversion_policy" jsonschema:"enum=preserve,enum=parse_text,enum=scalar_text,enum=singleton_list"`
}

type PropertyMigrationDecision struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740990"`
	ImpactDigest     string `json:"impact_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
}

type PropertyMigrationLookup struct {
	Stage          string `json:"stage" jsonschema:"enum=preview,enum=confirm,enum=execute"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=200"`
}

type PropertyMigrationReceipt struct {
	SpaceID        string    `json:"space_id"`
	PlanID         string    `json:"plan_id"`
	Stage          string    `json:"stage" jsonschema:"enum=preview,enum=confirm,enum=execute"`
	RequestDigest  string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64    `json:"result_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CommittedAt    time.Time `json:"committed_at"`
}

type PropertyMigrationItem struct {
	FileID          string `json:"file_id"`
	VersionID       string `json:"version_id"`
	SidecarRevision uint64 `json:"sidecar_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	Before          string `json:"before"`
	After           string `json:"after"`
	Status          string `json:"status" jsonschema:"enum=pending,enum=done,enum=failed,enum=stale"`
	Error           string `json:"error"`
}

type PropertyMigrationView struct {
	PlanID         string                   `json:"plan_id"`
	SpaceID        string                   `json:"space_id"`
	Revision       uint64                   `json:"revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	ImpactDigest   string                   `json:"impact_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	Phase          string                   `json:"phase" jsonschema:"enum=previewed,enum=confirmed,enum=completed,enum=partial"`
	EditingEnabled bool                     `json:"editing_enabled"`
	Request        PropertyMigrationPreview `json:"request"`
	Items          []PropertyMigrationItem  `json:"items" jsonschema:"maxItems=200"`
}

type PropertyMigrationCapability struct {
	SpaceID            string   `json:"space_id"`
	EditingEnabled     bool     `json:"editing_enabled"`
	StorageKind        string   `json:"storage_kind" jsonschema:"enum=sidecar"`
	MaxSidecars        int      `json:"max_sidecars" jsonschema:"minimum=200,maximum=200"`
	MarkdownSupported  bool     `json:"markdown_supported"`
	ConversionPolicies []string `json:"conversion_policies"`
}
