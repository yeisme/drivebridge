package contract

import "github.com/invopop/jsonschema"

func resourceTargetSchema(s *jsonschema.Schema) *jsonschema.Schema {
	s.OneOf = []*jsonschema.Schema{
		{Required: []string{"file_ref"}, Not: &jsonschema.Schema{Required: []string{"collection_snapshot_id"}}},
		{Required: []string{"collection_snapshot_id"}, Not: &jsonschema.Schema{Required: []string{"file_ref"}}},
	}
	s.AllOf = append(s.AllOf, &jsonschema.Schema{If: propertyCondition("kind", "reference_pack"), Then: &jsonschema.Schema{Required: []string{"collection_snapshot_id"}}})
	return s
}
