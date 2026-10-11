package contract

import "time"

// Label definitions name stable manual-namespace keys; display edits never
// rename existing assignments or reinterpret legacy free-form labels.
type LabelDefinitionInput struct {
	Key string `json:"key" jsonschema:"pattern=^[a-z][a-z0-9_]*$,maxLength=64"`
	DisplayName string `json:"display_name" jsonschema:"minLength=1,maxLength=120"`
	Group string `json:"group" jsonschema:"maxLength=80"`
}

type LabelDefinition struct {
	Key string `json:"key"`
	DisplayName string `json:"display_name"`
	Group string `json:"group"`
	Revision uint64 `json:"revision" jsonschema:"minimum=1,maximum=9007199254740991"`
}

type LabelDefinitionUpdate struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=0,maximum=9007199254740990"`
	Definitions []LabelDefinitionInput `json:"definitions" jsonschema:"maxItems=200"`
}

type LabelDefinitionPage struct {
	SpaceID string `json:"space_id"`
	Namespace string `json:"namespace" jsonschema:"enum=manual"`
	Revision uint64 `json:"revision"`
	ManagementEnabled bool `json:"management_enabled"`
	Definitions []LabelDefinition `json:"definitions" jsonschema:"maxItems=200"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type LabelDefinitionReceipt struct {
	SpaceID string `json:"space_id"`
	Namespace string `json:"namespace" jsonschema:"enum=manual"`
	RequestDigest string `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64 `json:"result_revision"`
	CommittedAt time.Time `json:"committed_at"`
}

// LabelMergePreview selects at most 200 current fixed versions. It moves only
// their manual source assignment, never deletes either dictionary identity.
type LabelMergePreview struct {
	Namespace string `json:"namespace" jsonschema:"enum=manual"`
	SourceKey string `json:"source_key"`
	TargetKey string `json:"target_key"`
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	SourceRevision uint64 `json:"source_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	TargetRevision uint64 `json:"target_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	Refs []string `json:"refs" jsonschema:"minItems=1,maxItems=200"`
}

// LabelMergePlanView is the canonical organization-plan wire projection used
// for selected-file label merges. It reports durable owner outcomes only.
type LabelMergePlanView struct {
	PlanID string `json:"plan_id"`
	SpaceID string `json:"space_id"`
	Revision uint64 `json:"revision"`
	Phase string `json:"phase"`
	Risk string `json:"risk"`
	ApprovalRequired bool `json:"approval_required"`
	PolicyID string `json:"policy_id,omitempty"`
	PolicyRevision uint64 `json:"policy_revision,omitempty"`
	ImpactDigest string `json:"impact_digest"`
	ItemsTotal int64 `json:"items_total"`
	ItemsDone int64 `json:"items_done"`
	ItemsStale int64 `json:"items_stale"`
	ItemsFailed int64 `json:"items_failed"`
	CompensationOf string `json:"compensation_of,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	Items []LabelMergePlanItemView `json:"items,omitempty"`
}

type LabelMergePlanItemView struct {
	ItemKey string `json:"item_key"`
	FileID string `json:"file_id"`
	VersionID string `json:"version_id"`
	Path string `json:"path"`
	Action string `json:"action"`
	ActionDetail string `json:"action_detail,omitempty"`
	Before string `json:"before,omitempty"`
	After string `json:"after,omitempty"`
	Status string `json:"status"`
	Error string `json:"error,omitempty"`
	ResultVersion string `json:"result_version,omitempty"`
}

// Confirmation pins the persisted preview; execution resumes that same plan.
// Recovery reads plan identity and revision, not a fabricated operation receipt.
type LabelMergeConfirm struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	ImpactDigest string `json:"impact_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
}

type LabelMergeExecute struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
}
