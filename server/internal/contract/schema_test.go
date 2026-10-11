package contract

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResponseSchemasDeterministic(t *testing.T) {
	first, err := EncodeResponseSchemas()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		next, err := EncodeResponseSchemas()
		if err != nil || !bytes.Equal(first, next) {
			t.Fatalf("schema drift: %v", err)
		}
	}
}

func TestGrantNullableFieldsAreRequired(t *testing.T) {
	schema := ResponseSchemas()["GrantView"]
	for _, name := range []string{"expiry_mode", "expires_at", "expiry_changed_at"} {
		found := false
		for _, key := range schema.Required {
			if key == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s not required", name)
		}
	}
	for _, name := range []string{"expires_at", "expiry_changed_at"} {
		field, ok := schema.Properties.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		nullable := false
		for _, variant := range field.OneOf {
			if variant.Type == "null" {
				nullable = true
			}
		}
		if !nullable {
			t.Fatalf("%s not nullable: %+v", name, field)
		}
	}
	body, err := json.Marshal(GrantView{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"expires_at":null`)) || !bytes.Contains(body, []byte(`"expiry_changed_at":null`)) {
		t.Fatalf("nullable fields omitted: %s", body)
	}
}
