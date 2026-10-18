package contract

import (
	"testing"
)

func TestRegisteredRoutesReferenceKnownSchemas(t *testing.T) {
	schemas := ResponseSchemas()
	for n, s := range RequestSchemas() {
		schemas[n] = s
	}
	for n, s := range EnvelopeSchemas() {
		schemas[n] = s
	}
	for n, s := range PageSchemas() {
		schemas[n] = s
	}
	seen := map[string]bool{}
	for _, route := range Routes() {
		if seen[route.OperationID] {
			t.Fatalf("duplicate operation %s", route.OperationID)
		}
		seen[route.OperationID] = true
		if !route.HumanOnly && route.Action == "" && route.ReadPolicy == "" {
			t.Fatalf("missing permission policy: %s", route.OperationID)
		}
		if schemas[route.ResponseSchema] == nil || (route.RequestSchema != "" && schemas[route.RequestSchema] == nil) {
			t.Fatalf("unknown schema: %+v", route)
		}
		op := RegisteredPaths()[route.Path].(map[string]any)[route.Method].(map[string]any)
		if op["x-implementation-status"] != "planned" || op["x-human-only"] != route.HumanOnly {
			t.Fatal("route readiness or identity was weakened")
		}
	}
}

func TestSubmissionRoutesUseActionAndNoInventedPathParameter(t *testing.T) {
	paths := RegisteredPaths()
	for _, path := range []string{"/api/v1/inbox-submissions", "/api/v1/inbox-submissions/lookup"} {
		op := paths[path].(map[string]any)["post"].(map[string]any)
		if op["x-required-action"] != "inbox.submit" || op["x-human-only"] != false {
			t.Fatal("wrong submission identity")
		}
		for _, p := range op["parameters"].([]any) {
			if p.(map[string]any)["in"] == "path" {
				t.Fatal("invented path parameter")
			}
		}
		responses := op["responses"].(map[string]any)
		status := "200"
		if path == "/api/v1/inbox-submissions" {
			status = "201"
		}
		if responses[status] == nil {
			t.Fatalf("missing success status %s", status)
		}
	}
}
