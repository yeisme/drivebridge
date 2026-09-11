package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func tool(name, description string, required ...string) mcp.Tool {
	opts := []mcp.ToolOption{mcp.WithDescription(description)}
	for _, key := range required {
		opts = append(opts, mcp.WithString(key, mcp.Required(), mcp.Description(key)))
	}
	return mcp.NewTool(name, opts...)
}
func (s *Service) Tools() []mcp.Tool {
	return []mcp.Tool{
		tool("capabilities", "Discover allowed spaces, exact tool schemas, transfer limits and recovery."),
		tool("list", "List files directly in the configured OneDrive root.", "space_id"),
		tool("stat", "Verify a stable reference and detect replacement or moves outside the space.", "file_ref"),
		tool("upload", "Upload a local file. The destination is an operation-prefixed name; existing files are not replaced. Poll operation_get.", "space_id", "local_path", "name", "idempotency_key"),
		tool("download", "Download into an allowed local directory without replacing a different file. Poll operation_get.", "file_ref", "local_path", "idempotency_key"),
		tool("operation_get", "Recover the original transfer result before starting another operation.", "operation_id"),
		tool("resume", "Resume an interrupted or failed transfer; may retransmit bytes. Never creates a generation task.", "operation_id"),
		tool("cancel", "Cancel a transfer; completed files are retained.", "operation_id"),
		tool("save_output", "Save an existing succeeded/imported Scaena task artifact to OneDrive. Never submits generation. Poll operation_get.", "space_id", "submission_ref", "idempotency_key"),
		tool("prepare_reference", "Verify a file and report handoff readiness. No URL is returned. The authorized local owner acquires access just before submission.", "file_ref"),
	}
}
func (s *Service) Capabilities() any {
	return map[string]any{"spec_version": "0.1", "enabled": s.Enabled, "spaces": s.Spaces(), "tools": s.Tools(), "secure_handoff": s.Enabled && s.Handoff, "output_adapter_enabled": s.Enabled && s.Output != nil, "authorization_status": "not_probed", "storage_status": "not_probed", "provider_consumption_status": "not_verified", "handoff_transport": "owner_unix_socket", "download_url_ttl": "unknown", "cli_required_on_mcp_client": false, "recovery": "operation_get_then_resume", "scope": "local_single_user", "generation_owner": "scaena"}
}
func (s *Service) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	text := func(k string) string { v, _ := args[k].(string); return v }
	switch name {
	case "capabilities":
		return s.Capabilities(), nil
	case "list":
		return s.List(ctx, text("space_id"))
	case "stat":
		return s.Stat(ctx, text("file_ref"))
	case "upload":
		return s.Upload(ctx, text("space_id"), text("local_path"), text("name"), text("idempotency_key"))
	case "download":
		return s.Download(ctx, text("file_ref"), text("local_path"), text("idempotency_key"))
	case "operation_get":
		return s.Get(text("operation_id"))
	case "resume":
		return s.Resume(text("operation_id"))
	case "cancel":
		if !s.Enabled {
			return nil, errors.New("disabled")
		}
		return s.Cancel(text("operation_id"))
	case "save_output":
		return s.SaveOutput(text("space_id"), text("submission_ref"), text("idempotency_key"))
	case "prepare_reference":
		f, e := s.Stat(ctx, text("file_ref"))
		if e != nil {
			return nil, e
		}
		p, e := s.Space(f.SpaceID)
		if e != nil {
			return nil, e
		}
		if !s.Handoff || p.Project == "" || p.Actor == "" {
			return nil, errors.New("secure_handoff_unavailable")
		}
		return map[string]any{"file": f, "status": "ready_for_owner_acquisition", "checked_at": time.Now().UTC(), "expires_at": nil}, nil
	}
	return nil, errors.New("unknown_tool")
}
func (s *Service) MCP() *server.MCPServer {
	srv := server.NewMCPServer("drivebridge", "0.1.0", server.WithToolCapabilities(false), server.WithResourceCapabilities(false, false))
	for _, t := range s.Tools() {
		name := t.Name
		srv.AddTool(t, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := s.Call(ctx, name, request.GetArguments())
			if err != nil {
				return mcp.NewToolResultError(safeError(err)), nil
			}
			b, err := json.Marshal(out)
			if err != nil {
				return mcp.NewToolResultError("serialization_failed"), nil
			}
			return mcp.NewToolResultText(string(b)), nil
		})
	}
	srv.AddResource(mcp.NewResource("drivebridge://capabilities", "DriveBridge capabilities", mcp.WithMIMEType("application/json")), func(ctx context.Context, r mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		b, e := json.Marshal(s.Capabilities())
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: "drivebridge://capabilities", MIMEType: "application/json", Text: string(b)}}, e
	})
	return srv
}
