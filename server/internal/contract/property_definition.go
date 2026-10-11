package contract

import "time"

// PropertyDefinition describes a stable key; changing its type requires migration.
type PropertyDefinition struct {
	Key         string `json:"key" jsonschema:"pattern=^[a-z][a-z0-9_]*$,maxLength=64"`
	DisplayName string `json:"display_name" jsonschema:"minLength=1,maxLength=120"`
	Group       string `json:"group" jsonschema:"maxLength=80"`
	Type        string `json:"type" jsonschema:"enum=text,enum=number,enum=date,enum=checkbox,enum=list,enum=reference"`
}

type PropertyDefinitionUpdate struct {
	ExpectedRevision uint64               `json:"expected_revision" jsonschema:"minimum=0,maximum=9007199254740990"`
	Definitions      []PropertyDefinition `json:"definitions" jsonschema:"maxItems=200"`
}

type PropertyDefinitionView struct {
	SpaceID     string               `json:"space_id"`
	Revision    uint64               `json:"revision" jsonschema:"minimum=0,maximum=9007199254740991"`
	Definitions []PropertyDefinition `json:"definitions" jsonschema:"maxItems=200"`
}

type PropertyDefinitionReceipt struct {
	SpaceID        string    `json:"space_id"`
	RequestDigest  string    `json:"request_digest" jsonschema:"pattern=^[a-f0-9]{64}$"`
	ResultRevision uint64    `json:"result_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CommittedAt    time.Time `json:"committed_at"`
}
