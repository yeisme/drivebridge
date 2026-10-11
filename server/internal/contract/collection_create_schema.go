package contract

import (
	"encoding/json"
	"github.com/invopop/jsonschema"
)

func collectionFilterSchema() *jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	min, max := uint64(1), uint64(4096)
	text := func(nonempty bool) *jsonschema.Schema {
		s := &jsonschema.Schema{Type: "string", MaxLength: &max, Pattern: `^[^\u0000]*$`}
		if nonempty {
			s.MinLength = &min
		}
		return s
	}
	branch := func(fields, ops []any, value *jsonschema.Schema) *jsonschema.Schema {
		s := r.Reflect(CollectionRuleFilter{})
		s.Properties.Set("field", &jsonschema.Schema{Type: "string", Enum: fields})
		s.Properties.Set("op", &jsonschema.Schema{Type: "string", Enum: ops})
		s.Properties.Set("value", value)
		return s
	}
	date := text(false)
	date.Format = "date-time"
	compare := []any{"gt", "gte", "lt", "lte"}
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		branch([]any{"path_prefix"}, []any{"prefix", "contains"}, text(true)),
		branch([]any{"mime_type", "caller", "task_ref"}, []any{"eq"}, text(false)),
		branch([]any{"labels.manual", "labels.rule", "labels.suggested"}, []any{"eq", "ne", "contains"}, text(true)),
		branch([]any{"updated_at"}, compare, date),
		branch([]any{"size_bytes"}, compare, &jsonschema.Schema{Type: "integer", Minimum: json.Number("0"), Maximum: json.Number("9007199254740991")}),
		branch([]any{"trashed"}, []any{"eq"}, &jsonschema.Schema{Type: "boolean", Enum: []any{false}}),
	}}
}

func collectionRuleSchema() *jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	s := r.Reflect(CollectionRule{})
	filters, _ := s.Properties.Get("filters")
	filters.Items = collectionFilterSchema()
	sort, _ := s.Properties.Get("sort")
	sort.Items = r.Reflect(CollectionRuleSort{})
	s.Description = "Exact rule for one containing collection space. The application also checks identical space identity, original UTF-8 bytes <=64KiB, duplicate keys, scalar escapes and field byte budgets."
	return s
}

func collectionCreateSchema() *jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	manual := r.Reflect(CreateFileCollectionRequest{})
	// Manual requests historically ignore a supplied rule, including null.
	// Preserve transport compatibility; new consumers should omit it for manual.
	manual.Properties.Set("kind", &jsonschema.Schema{Type: "string", Enum: []any{"manual"}})
	manual.Properties.Set("rule", &jsonschema.Schema{})
	rule := r.Reflect(CreateFileCollectionRequest{})
	rule.Properties.Set("kind", &jsonschema.Schema{Type: "string", Enum: []any{"rule"}})
	rule.Properties.Set("rule", collectionRuleSchema())
	rule.Required = append(rule.Required, "rule")
	s := &jsonschema.Schema{OneOf: []*jsonschema.Schema{manual, rule}}
	s.Description = "Creation returns a current view; only the separate human lookup returns the original receipt. Preserve the original nested rule bytes for the legacy digest. Name is limited to 200 UTF-8 bytes by the application."
	return s
}
