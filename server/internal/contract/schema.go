package contract

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// ResponseSchemas derives public response schemas from wire DTOs only.
// Request schemas and registered routes are added separately; an available
// schema does not imply that a corresponding endpoint is implemented.
func ResponseSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	grant := r.Reflect(GrantView{})
	grant.AllOf = []*jsonschema.Schema{{
		If:   propertyCondition("expiry_mode", "never"),
		Then: propertyCondition("expires_at", nil),
		Else: &jsonschema.Schema{Not: propertyCondition("expires_at", nil)},
	}}
	return map[string]*jsonschema.Schema{
		"GrantView":             grant,
		"AgentGroupView":        r.Reflect(AgentGroupView{}),
		"AgentOrganizationView": r.Reflect(AgentOrganizationView{}),
	}
}

// EncodeResponseSchemas is deterministic and contains no runtime state.
func EncodeResponseSchemas() ([]byte, error) {
	return json.MarshalIndent(ResponseSchemas(), "", "  ")
}
