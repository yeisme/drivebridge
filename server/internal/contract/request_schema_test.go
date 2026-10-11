package contract

import "testing"

func TestRequestSchemasUnwrapPresence(t *testing.T) {
	schemas := RequestSchemas()
	patch := schemas["PatchGrantRequest"]
	if len(patch.Required) != 1 || patch.Required[0] != "expected_revision" {
		t.Fatalf("unexpected required fields: %v", patch.Required)
	}
	for name, kind := range map[string]string{"enabled": "boolean", "expiry_mode": "string", "expires_in_seconds": "integer", "actions": "array"} {
		field, ok := patch.Properties.Get(name)
		if !ok || field.Type != kind || field.Properties != nil {
			t.Fatalf("%s: wrapper leaked: %+v", name, field)
		}
	}
	group, _ := schemas["PatchOrganizationRequest"].Properties.Get("group_id")
	if len(group.OneOf) != 2 || group.OneOf[1].Type != "null" {
		t.Fatal("group removal must allow null")
	}
	pinned, _ := schemas["PatchOrganizationRequest"].Properties.Get("pinned")
	if pinned.Type != "boolean" || len(pinned.OneOf) != 0 {
		t.Fatal("pinned must reject null")
	}
}

func TestExpirySchemaRequiresExplicitDuration(t *testing.T) {
	for name, schema := range RequestSchemas() {
		if name != "CreateGrantRequest" && name != "PatchGrantRequest" {
			continue
		}
		if len(schema.AllOf) != 1 {
			t.Fatalf("%s missing expiry condition", name)
		}
		rule := schema.AllOf[0]
		if rule.If.Required[0] != "expiry_mode" || rule.Then.Required[0] != "expires_in_seconds" || rule.Else.Not.Required[0] != "expires_in_seconds" {
			t.Fatalf("%s invalid expiry condition", name)
		}
		seconds, _ := schema.Properties.Get("expires_in_seconds")
		if len(seconds.Enum) != 6 {
			t.Fatalf("%s duration count", name)
		}
	}
}
