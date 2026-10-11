package service

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// toolProperties decodes a tool's declared input schema into property names
// with their descriptions.
func toolProperties(t *testing.T, tool mcp.Tool) map[string]string {
	t.Helper()
	raw, err := json.Marshal(tool)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		InputSchema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for name, prop := range decoded.InputSchema.Properties {
		out[name] = prop.Description
	}
	return out
}

func findTool(t *testing.T, name string) mcp.Tool {
	t.Helper()
	for _, tool := range ToolDefinitions() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %s not defined", name)
	return mcp.Tool{}
}

// TestTrashRestoreDeclareRef kills the schema drift: CallTool accepts a ref
// argument on trash/restore, so the declared schema must carry it too.
func TestTrashRestoreDeclareRef(t *testing.T) {
	for _, name := range []string{"trash", "restore"} {
		props := toolProperties(t, findTool(t, name))
		if _, ok := props["ref"]; !ok {
			t.Fatalf("%s: ref accepted by CallTool but missing from the declared schema", name)
		}
		if _, ok := props["file_id"]; !ok {
			t.Fatalf("%s: file_id missing from the declared schema", name)
		}
	}
}

// TestInsightToolsDeclareAcceptedFields pins the filter surface: the declared
// fields are exactly the keys CallTool reads, each with a description.
func TestInsightToolsDeclareAcceptedFields(t *testing.T) {
	accepted := []string{"space_id", "connection_id", "from", "to", "actor", "caller", "direction", "purpose", "group", "action", "outcome", "tenant_context", "space_context"}
	for _, name := range []string{"usage_summary", "usage_series", "usage_breakdown", "usage_scans", "audit_events"} {
		props := toolProperties(t, findTool(t, name))
		if len(props) != len(accepted) {
			t.Fatalf("%s: declared %d fields, want exactly the %d CallTool reads: %v", name, len(props), len(accepted), props)
		}
		for _, field := range accepted {
			if props[field] == "" {
				t.Fatalf("%s: field %s lacks a description", name, field)
			}
		}
	}
}
