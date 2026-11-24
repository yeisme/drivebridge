package contract

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// ResponseSchemas derives public response schemas from wire DTOs only.
// Request schemas and registered routes are added separately; an available
// schema does not imply that a corresponding endpoint is implemented.
type ActionCatalog struct {
	CatalogVersion string                `json:"catalog_version"`
	Actions        []ActionCatalogAction `json:"actions"`
	Limits         map[string]*int64     `json:"limits,omitempty"`
	Context        ActionCatalogContext  `json:"context"`
}

type ActionCatalogContext struct {
	InstanceID      string   `json:"instance_id"`
	LocalFileAccess bool     `json:"local_file_access"`
	PublicBaseURL   string   `json:"public_base_url,omitempty"`
	Auth            string   `json:"auth"`
	ContextHeaders  []string `json:"context_headers,omitempty"`
}

type ActionCatalogAction struct {
	ID                string   `json:"id"`
	Intent            string   `json:"intent"`
	Surface           []string `json:"surface"`
	Tool              string   `json:"tool,omitempty"`
	Implemented       bool     `json:"implemented"`
	Configured        bool     `json:"configured"`
	Authorized        bool     `json:"authorized"`
	DependencyStatus  string   `json:"dependency_status"`
	Availability      string   `json:"availability"`
	BlockReason       string   `json:"block_reason,omitempty"`
	SideEffect        string   `json:"side_effect"`
	Actor             string   `json:"actor"`
	ExecutionLocation string   `json:"execution_location"`
	InputSchemaRef    string   `json:"input_schema_ref,omitempty"`
	ResultKind        string   `json:"result_kind,omitempty"`
	ByteTransport     string   `json:"byte_transport,omitempty"`
	Recovery          []string `json:"recovery,omitempty"`
	ObservedAt        string   `json:"observed_at,omitempty"`
}

func ResponseSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	grant := r.Reflect(GrantView{})
	grant.AllOf = []*jsonschema.Schema{{
		If:   propertyCondition("expiry_mode", "never"),
		Then: propertyCondition("expires_at", nil),
		Else: &jsonschema.Schema{Not: propertyCondition("expires_at", nil)},
	}}
	return map[string]*jsonschema.Schema{
		"ActionCatalog":         r.Reflect(ActionCatalog{}),
		"GrantView":             grant,
		"AgentGroupView":        r.Reflect(AgentGroupView{}),
		"AgentOrganizationView": r.Reflect(AgentOrganizationView{}),
	}
}

// EncodeResponseSchemas is deterministic and contains no runtime state.
func EncodeResponseSchemas() ([]byte, error) {
	return json.MarshalIndent(ResponseSchemas(), "", "  ")
}
