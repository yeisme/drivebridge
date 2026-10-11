package contract

import "testing"

func TestListRoutesHaveBoundedPagination(t *testing.T) {
	for _, route := range Routes() {
		if len(route.QueryFields) == 0 {
			continue
		}
		op := RegisteredPaths()[route.Path].(map[string]any)[route.Method].(map[string]any)
		if route.OperationID == "getFilePropertySidecar" {
			if len(route.QueryFields) != 1 || route.QueryFields[0] != "version_id" {
				t.Fatal("fixed sidecar query drift")
			}
			foundVersion := false
			for _, raw := range op["parameters"].([]any) {
				p := raw.(map[string]any)
				if p["name"] == "version_id" {
					foundVersion = p["required"] == true && p["schema"].(map[string]any)["pattern"] == `^[a-f0-9]{48}$`
				}
			}
			if !foundVersion {
				t.Fatal("fixed version must be required")
			}
			continue // One fixed-version object, not a list or cursor surface.
		}
		found := false
		for _, raw := range op["parameters"].([]any) {
			p := raw.(map[string]any)
			name, maximum, defaultValue := "limit", 200, 50
			if route.OperationID == "getWorkspaceHome" {
				name, maximum, defaultValue = "recent_limit", 20, 6
			}
			if p["name"] != name {
				continue
			}
			s := p["schema"].(map[string]any)
			if s["minimum"] != 1 || s["maximum"] != maximum || s["default"] != defaultValue {
				t.Fatalf("unbounded list %s", route.OperationID)
			}
			found = true
		}
		if !found {
			t.Fatalf("no limit: %s", route.OperationID)
		}
		if op["responses"].(map[string]any)["410"] == nil {
			t.Fatal("missing cursor expiry response")
		}
	}
}
