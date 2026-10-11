package service_test

import (
	"bytes"
	"encoding/json"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func workspaceMCP(t *testing.T, base, token string, expectedTools ...string) *mcpclient.Client {
	t.Helper()
	c, err := mcpclient.NewStreamableHttpClient(base+"/mcp", transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = "2024-11-05"
	init.Params.ClientInfo = mcp.Implementation{Name: "workspace-fixture", Version: "1"}
	if _, err := c.Initialize(t.Context(), init); err != nil {
		t.Fatal(err)
	}
	listed, err := c.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(expectedTools) == 0 {
		expectedTools = []string{"capabilities", "operation_get", "complete_upload"}
	}
	want := make(map[string]bool, len(expectedTools))
	for _, name := range expectedTools {
		want[name] = true
	}
	if len(listed.Tools) != len(want) {
		t.Fatalf("restricted tool count %d", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if !want[tool.Name] {
			t.Fatalf("restricted discovery exposed %s", tool.Name)
		}
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatal("restricted recovery tool missing")
	}
	return c
}

func workspaceMCPRevoked(t *testing.T, c *mcpclient.Client) {
	t.Helper()
	q := mcp.CallToolRequest{}
	q.Params.Name = "capabilities"
	q.Params.Arguments = map[string]any{}
	if result, err := c.CallTool(t.Context(), q); err == nil && !result.IsError {
		t.Fatal("revoked MCP session retained authority")
	}
}

func workspaceMCPCall(t *testing.T, c *mcpclient.Client, name string, args map[string]any, wantError bool) []byte {
	t.Helper()
	q := mcp.CallToolRequest{}
	q.Params.Name = name
	q.Params.Arguments = args
	result, err := c.CallTool(t.Context(), q)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError != wantError {
		t.Fatalf("MCP %s error=%t expected=%t", name, result.IsError, wantError)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("Bearer ")) {
		t.Fatal("MCP leaked credential header")
	}
	return encoded
}
