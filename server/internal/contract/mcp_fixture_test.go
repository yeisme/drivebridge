package contract

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// This is a transport contract fixture, not a production workspace tool.
// The fake tool is registered only in this isolated test server.
func TestGeneratedFixtureThroughMCPTransport(t *testing.T) {
	srv := server.NewMCPServer("contract-fixture", "1", server.WithToolCapabilities(false))
	srv.AddTool(mcp.NewTool("fixture_read"), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		payload, err := json.Marshal(ResponseFixtures()["GrantReplayEnvelope"])
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(payload)), nil
	})
	httpServer := httptest.NewServer(server.NewStreamableHTTPServer(srv, server.WithStateLess(true)))
	defer httpServer.Close()
	client, err := mcpclient.NewStreamableHttpClient(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = "2024-11-05"
	init.Params.ClientInfo = mcp.Implementation{Name: "contract-test", Version: "1"}
	if _, err = client.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	request := mcp.CallToolRequest{}
	request.Params.Name = "fixture_read"
	result, err := client.CallTool(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatal("unexpected MCP result")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected text result")
	}
	var replay DataEnvelope[GrantReplayView]
	if err = json.Unmarshal([]byte(text.Text), &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Data.Grant.GrantID != "grant-fixture" || replay.Data.Credential.State != "not_replayable" {
		t.Fatal("fixture changed in transport")
	}
	var wire map[string]any
	if err = json.Unmarshal([]byte(text.Text), &wire); err != nil {
		t.Fatal(err)
	}
	credential := wire["data"].(map[string]any)["credential"].(map[string]any)
	if _, present := credential["secret"]; present {
		t.Fatal("replay exposed secret")
	}
}
