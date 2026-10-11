package contract

import "github.com/invopop/jsonschema"

// Definitions select one of these shapes at runtime; schema validation alone
// cannot authorize references or determine a particular key's declared type.
func sidecarValuesSchema() *jsonschema.Schema {
	maxKeys, maxText, maxList, maxItem, maxKey := uint64(200), uint64(8192), uint64(100), uint64(1024), uint64(64)
	return &jsonschema.Schema{Type: "object", MaxProperties: &maxKeys, PropertyNames: &jsonschema.Schema{Type: "string", Pattern: `^[a-z][a-z0-9_]*$`, MaxLength: &maxKey}, AdditionalProperties: &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Type: "string", MaxLength: &maxText},
		{Type: "number"},
		{Type: "boolean"},
		{Type: "array", MaxItems: &maxList, Items: &jsonschema.Schema{Type: "string", MaxLength: &maxItem}},
	}}}
}
func sidecarPatchSchema(s *jsonschema.Schema) *jsonschema.Schema {
	s.Properties.Set("updates", sidecarValuesSchema())
	removed, _ := s.Properties.Get("remove_keys")
	removed.UniqueItems = true
	maxKey := uint64(64)
	removed.Items = &jsonschema.Schema{Type: "string", Pattern: `^[a-z][a-z0-9_]*$`, MaxLength: &maxKey}
	return s
}
func sidecarViewEnvelopeSchema(s *jsonschema.Schema) *jsonschema.Schema {
	data, _ := s.Properties.Get("data")
	data.Properties.Set("values", sidecarValuesSchema())
	return s
}
