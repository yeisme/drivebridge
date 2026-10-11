package contract

import (
	"reflect"
	"strings"
	"testing"
)

func TestInvocationOrderContract(t *testing.T) {
	for _, route := range Routes() {
		for _, field := range route.QueryFields {
			if field != "order" {
				continue
			}
			op := RegisteredPaths()[route.Path].(map[string]any)[route.Method].(map[string]any)
			found := false
			for _, raw := range op["parameters"].([]any) {
				p := raw.(map[string]any)
				if p["name"] != "order" {
					continue
				}
				found = true
				schema := p["schema"].(map[string]any)
				want := []string{"pinned"}
				if route.OperationID == "listAgentInvocations" {
					want = []string{"started_desc"}
					if !strings.Contains(schema["description"].(string), "invocation_id ascending") {
						t.Fatal("missing tie-break semantics")
					}
				}
				if p["required"] != false || !reflect.DeepEqual(schema["enum"], want) {
					t.Fatalf("wrong order enum for %s", route.OperationID)
				}
			}
			if !found {
				t.Fatal("missing order parameter")
			}
		}
	}
}

func TestInvocationTimeBoundsContract(t *testing.T) {
	op := RegisteredPaths()["/api/v1/agent-invocations"].(map[string]any)["get"].(map[string]any)
	found := map[string]string{}
	for _, raw := range op["parameters"].([]any) {
		p := raw.(map[string]any)
		name := p["name"].(string)
		if name != "started_after" && name != "started_at_or_after" && name != "started_before" {
			continue
		}
		schema := p["schema"].(map[string]any)
		if p["in"] != "query" || p["required"] != false || schema["format"] != "date-time" {
			t.Fatalf("invalid time parameter: %s", name)
		}
		found[name], _ = schema["description"].(string)
	}
	for key, phrase := range map[string]string{"started_after": "Exclusive lower", "started_at_or_after": "[start, end)", "started_before": "Exclusive upper"} {
		if !strings.Contains(found[key], phrase) {
			t.Fatalf("missing time semantics: %s", key)
		}
	}
	for _, key := range []string{"started_after", "started_at_or_after"} {
		if !strings.Contains(found[key], "Mutually exclusive") {
			t.Fatal("ambiguous lower bounds")
		}
	}
}
