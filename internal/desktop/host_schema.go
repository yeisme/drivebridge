package desktop

import "github.com/invopop/jsonschema"

// JSONSchema keeps the host event union in the emitting Go owner. A ready
// event may never carry a ticket or an error, and a selection has no address.
func (HostEvent) JSONSchema() *jsonschema.Schema {
	type plain HostEvent
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	event := r.Reflect(plain{})
	pointer := func(value uint64) *uint64 { return &value }
	fields := []string{"schema_version", "type", "session_nonce", "address", "request_id", "selection_id", "code"}
	version, _ := event.Properties.Get("schema_version")
	version.Enum = []any{HostVersion}
	for _, name := range []string{"request_id", "selection_id", "session_nonce"} {
		value, _ := event.Properties.Get(name)
		value.MinLength = pointer(1)
		value.MaxLength = pointer(128)
		value.Pattern = `^[A-Za-z0-9][A-Za-z0-9._:-]*$`
	}
	nonce, _ := event.Properties.Get("session_nonce")
	nonce.MinLength = pointer(16)
	address, _ := event.Properties.Get("address")
	address.MaxLength = pointer(64)
	address.Pattern = `^http://127\.0\.0\.1:[1-9][0-9]*$`
	code, _ := event.Properties.Get("code")
	code.Enum = []any{"desktop_host_invalid", "native_selection_invalid"}
	for _, branch := range []struct {
		kind     string
		required []string
	}{
		{"ready", []string{"session_nonce", "address"}},
		{"selection", []string{"request_id", "selection_id"}},
		{"cancelled", []string{"request_id"}},
		{"shutdown", []string{"session_nonce"}},
		{"error", []string{"code"}},
	} {
		schema := r.Reflect(plain{})
		schema.Required = append([]string{"schema_version", "type"}, branch.required...)
		kind := &jsonschema.Schema{Const: branch.kind}
		// Branch-local property maps prevent one branch from mutating another.
		schema.Properties = r.Reflect(plain{}).Properties
		schema.Properties.Set("type", kind)
		for _, name := range fields {
			if name != "type" {
				value, _ := event.Properties.Get(name)
				schema.Properties.Set(name, value)
			}
		}
		allowed := map[string]bool{"schema_version": true, "type": true}
		for _, name := range branch.required {
			allowed[name] = true
		}
		if branch.kind == "error" {
			allowed["request_id"] = true
		}
		for _, name := range fields {
			if !allowed[name] {
				schema.Properties.Delete(name)
			}
		}
		event.OneOf = append(event.OneOf, schema)
	}
	// Each branch is a complete closed shape, not a permissive union overlay.
	return &jsonschema.Schema{OneOf: event.OneOf}
}
