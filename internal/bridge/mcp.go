package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/yeisme/drivebridge/internal/recovery"
)

func tool(name, description string, required ...string) mcp.Tool {
	opts := []mcp.ToolOption{mcp.WithDescription(description)}
	for _, key := range required {
		opts = append(opts, mcp.WithString(key, mcp.Required(), mcp.Description(key)))
	}
	return mcp.NewTool(name, opts...)
}
func (s *Service) Tools() []mcp.Tool {
	tools := []mcp.Tool{
		tool("relay_targets", "List owner-configured relay target IDs, projects and disabled state; never probes endpoints or grants authority."),
		tool("relay_start", "Relay a pinned image, video or audio file to a preconfigured owner input (eikona, scaena or sonora) without generation; poll operation_get.", "file_ref", "target_id", "idempotency_key"),
		tool("relay_cancel", "Cancel the original local relay and target input; retain completed assets.", "operation_id"),
		tool("capabilities", "Discover allowed spaces, exact tool schemas, transfer limits and recovery."),
		tool("bind_discover", "List spaces eligible for consumer bind; default omits escape kinds. Caller labels are not authentication."),
		tool("bind_list", "List local consumer bind records (caller-declared labels are not authentication)."),
		tool("list", "List files in the space root or a workspace subdirectory.", "space_id"),
		tool("mkdir", "Create a workspace directory (idempotent); existing files with the name are a conflict.", "space_id", "path"),
		tool("search", "Search the space index by name (case-insensitive substring); verify hits with stat before consuming.", "space_id", "query"),
		tool("changes", "Poll incremental events since a cursor; a zero cursor returns the current position without history.", "space_id", "since"),
		tool("trash", "Soft-delete a reference inside the space; it stays queryable and restorable.", "file_ref"),
		tool("restore", "Move a trashed reference back into its directory; same-name conflicts get a suffix, never an overwrite.", "file_ref"),
		tool("stat", "Verify a stable reference and detect replacement or moves outside the space.", "file_ref"),
		tool("upload", "Upload a local file. The destination is an operation-prefixed name inside an optional directory; existing files are not replaced; same-content objects are reused without retransfer. Poll operation_get.", "space_id", "local_path", "name", "idempotency_key"),
		tool("link", "Register an explicit reference to an existing object in another space on the same drive; no bytes are copied and the source is never modified.", "file_ref", "space_id"),
		tool("download", "Download into an allowed local directory without replacing a different file. Poll operation_get.", "file_ref", "local_path", "idempotency_key"),
		tool("operation_get", "Recover the original transfer result before starting another operation.", "operation_id"),
		tool("inbox", "List recent completed or failed operations for the configured spaces."),
		tool("resume", "Resume an interrupted or failed transfer; may retransmit bytes. Never creates a generation task.", "operation_id"),
		tool("cancel", "Cancel a transfer; completed files are retained.", "operation_id"),
		tool("save_output", "Save an existing succeeded/imported Scaena task artifact to OneDrive. Never submits generation. Poll operation_get.", "space_id", "submission_ref", "idempotency_key"),
		tool("prepare_reference", "Verify a file and report handoff readiness. No URL is returned. The authorized local owner acquires access just before submission.", "file_ref"),
		tool("mount_status", "Read the CLI-owned mount projection (listen, mode, space_ids, alive); agents cannot start or stop mounts."),
	}
	for i := range tools {
		fields := []string{}
		switch tools[i].Name {
		case "upload":
			fields = []string{"dir", "caller", "task_ref", "consumer", "project", "bind_id"}
		case "link":
			fields = []string{"dir", "name"}
		case "search":
			fields = []string{"path", "mime_type", "after"}
		case "list":
			fields = []string{"path", "consumer", "project", "bind_id"}
		case "mkdir", "stat", "trash":
			fields = []string{"consumer", "project", "bind_id"}
		case "bind_discover":
			fields = []string{"allow_generic"}
		}
		if len(fields) > 0 && tools[i].InputSchema.Properties == nil {
			tools[i].InputSchema.Properties = map[string]any{}
		}
		for _, field := range fields {
			tools[i].InputSchema.Properties[field] = map[string]any{"type": "string"}
		}
	}
	for _, name := range []string{"usage_summary", "usage_series", "usage_breakdown", "usage_scans", "audit_events"} {
		t := tool(name, "Read local usage/audit projections; caller labels are not authenticated Agent identities.")
		if t.InputSchema.Properties == nil {
			t.InputSchema.Properties = map[string]any{}
		}
		for _, field := range []string{"space_id", "connection_id", "from", "to", "actor", "caller", "direction", "purpose", "group", "action", "outcome"} {
			t.InputSchema.Properties[field] = map[string]any{"type": "string"}
		}
		tools = append(tools, t)
	}
	t := tool("usage_scan", "Run an already-enabled local inventory with its existing budget; cannot enable scanning.", "space_id")
	t.InputSchema.Properties["resume_id"] = map[string]any{"type": "string"}
	tools = append(tools, t)
	planGet := tool("sync_plan_get", "Read a frozen sync plan, one item page and its recovery hints (read-only; apply stays a user-terminal command).", "plan_id")
	planGet.InputSchema.Properties["after"] = map[string]any{"type": "string", "description": "keyset cursor from next_after"}
	planGet.InputSchema.Properties["limit"] = map[string]any{"type": "integer", "description": "page size, default 50, max 200"}
	tools = append(tools, planGet)
	tools = append(tools, tool("retention_preview", "Read-only cleanup classification for a space: staging, local trash, protected unflushed VFS bytes, remote references; never deletes or purges.", "space_id"))
	applyToolHints(tools)
	return tools
}
func (s *Service) Capabilities() any {
	kinds := map[string]bool{}
	for _, p := range s.Spaces() {
		kinds[p.Kind()] = true
	}
	return map[string]any{"relay": s.RelayDiscovery(), "storage_kinds": StorageKinds(), "bind_default_kinds": BindDefaultKinds(), "storage_add": map[string]any{"add": "drivebridge storage add --kind <kind> …", "probe": "drivebridge storage probe --remote <remote> [--remote-path <path>]", "guide": "drivebridge storage kinds", "note": "generic kinds are transfer-only; OneDrive onboards via auth + setup"}, "storage_adopt": map[string]any{"adopt": "drivebridge storage adopt --kind local|s3 --consumer <id> --space <slug> …", "status": "drivebridge storage adopt status", "note": "zero-copy attach of owner local roots or S3 prefixes; mutating owner-protected paths return owner_protected"}, "bind": map[string]any{"discover": "drivebridge bind discover [--allow-generic]", "add": "drivebridge bind add --consumer <id> --project <ref> --space <space> --prefix <prefix> --purpose <purpose>", "list": "drivebridge bind list", "bind_discover": true, "bind_list": true, "note": "Caller-declared consumer and project labels are stored facts, not authentication. Default discovery lists bind_default kinds and adopted spaces."}, "provenance": "caller_declared_labels", "search_scope": "observed_index_not_full_remote", "after_semantics": "first_observed_at", "history": "pinned_identity_not_snapshot", "concurrency": "single_process_per_state", "spec_version": "0.1", "enabled": s.Enabled, "spaces": s.Spaces(), "tools": s.Tools(), "backend_capabilities": map[string]any{"onedrive": map[string]any{"transfer": true, "direct_url": true, "reference_handoff": true}, "generic": map[string]any{"transfer": true, "direct_url": false, "reference_handoff": false}}, "configured_backend_kinds": kinds, "secure_handoff": s.Enabled && s.Handoff, "output_adapter_enabled": s.Enabled && s.Output != nil, "authorization_status": "not_probed", "storage_status": "not_probed", "provider_consumption_status": "not_verified", "handoff_transport": "owner_unix_socket", "download_url_ttl": "unknown", "cli_required_on_mcp_client": false, "recovery": "operation_get_then_resume", "scope": "local_single_user", "generation_owner": "scaena", "cas_reuse": true, "mount": map[string]any{"start": "cli_only", "adapters": []string{"webdav", "pinax-vault", "pinax-vault-rclone"}, "fuse": "unsupported_default_build", "writable": "explicit_flag_only", "path_preset": "pinax-vault", "path_adapters": []string{"alias", "s3"}, "overlay": "pinax"}}
}
func (s *Service) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	text := func(k string) string { v, _ := args[k].(string); return v }
	ctx = applyBindScope(ctx, args)
	if name == "usage_scan" {
		return s.Scan(ctx, text("space_id"), text("resume_id"))
	}
	if name == "sync_plan_get" {
		after, _ := strconv.ParseUint(text("after"), 10, 64)
		limit := 0
		if v, ok := args["limit"].(float64); ok {
			limit = int(v)
		}
		return s.SyncPlanGet(text("plan_id"), after, limit)
	}
	if name == "retention_preview" {
		return s.RetentionPreview(ctx, text("space_id"))
	}
	if strings.HasPrefix(name, "usage_") || name == "audit_events" {
		q := url.Values{}
		for _, field := range []string{"space_id", "connection_id", "from", "to", "actor", "caller", "direction", "purpose", "group", "action", "outcome"} {
			if text(field) != "" {
				q.Set(field, text(field))
			}
		}
		return s.Usage(strings.TrimPrefix(name, "usage_"), q)
	}
	switch name {
	case "relay_targets":
		return s.RelayDiscovery(), nil
	case "relay_start":
		return s.StartRelay(ctx, text("file_ref"), text("target_id"), text("idempotency_key"))
	case "relay_cancel":
		return s.CancelRelay(ctx, text("operation_id"))
	case "inbox":
		var ops []Operation
		if e := s.DB.Where("updated >= ?", time.Now().UTC().Add(-30*24*time.Hour)).Order("updated DESC").Limit(200).Find(&ops).Error; e != nil {
			return nil, errors.New("state_unavailable")
		}
		return ops, nil
	case "capabilities":
		return s.Capabilities(), nil
	case "bind_discover":
		allow := false
		switch v := args["allow_generic"].(type) {
		case bool:
			allow = v
		case string:
			allow = v == "true" || v == "1"
		}
		spaces := s.DiscoverBindSpaces(allow)
		return map[string]any{"count": len(spaces), "spaces": spaces}, nil
	case "bind_list":
		rows := s.Binds()
		return map[string]any{"count": len(rows), "binds": rows}, nil
	case "list":
		return s.List(ctx, text("space_id"), text("path"))
	case "mkdir":
		if e := s.Mkdir(ctx, text("space_id"), text("path")); e != nil {
			return nil, e
		}
		return map[string]any{"status": "created", "path": text("path")}, nil
	case "stat":
		return s.Stat(ctx, text("file_ref"))
	case "search":
		after := time.Time{}
		if text("after") != "" {
			var err error
			after, err = time.Parse(time.RFC3339, text("after"))
			if err != nil {
				return nil, fmt.Errorf("invalid_time")
			}
		}
		return s.Search(text("space_id"), text("query"), text("path"), SearchOptions{MIME: text("mime_type"), After: after})
	case "changes":
		since, _ := strconv.ParseUint(text("since"), 10, 64)
		events, cursor, e := s.Changes(text("space_id"), since)
		if e != nil {
			return nil, e
		}
		return map[string]any{"events": events, "cursor": cursor}, nil
	case "trash":
		return s.Trash(ctx, text("file_ref"))
	case "restore":
		return s.Restore(ctx, text("file_ref"))
	case "upload":
		if text("caller") == "" && text("task_ref") == "" {
			// Without declared provenance the upload may reuse content that
			// is already stored in the space (content-addressed).
			return s.ReuseUpload(ctx, text("space_id"), text("dir"), text("local_path"), text("name"), text("idempotency_key"))
		}
		return s.Upload(ctx, text("space_id"), text("dir"), text("local_path"), text("name"), text("idempotency_key"), Provenance{Caller: text("caller"), TaskRef: text("task_ref")})
	case "link":
		return s.LinkRef(ctx, text("file_ref"), text("space_id"), text("dir"), text("name"))
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
		f, _, e = s.reference(ctx, HandoffRequest{Ref: f.Ref, Project: p.Project, Actor: p.Actor})
		if e != nil {
			return nil, e
		}
		return map[string]any{"file": f, "status": "ready_for_owner_acquisition", "checked_at": time.Now().UTC(), "expires_at": nil}, nil
	case "mount_status":
		return s.MountStatus(), nil
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
				return toolErrorResult(err), nil
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

// toolErrorResult keeps the historical first text block — the bare machine
// code — and appends a structured JSON block with the recovery hint so MCP
// hosts get an executable next_action without the product CLI.
func toolErrorResult(err error) *mcp.CallToolResult {
	code := safeError(err)
	hintPayload, marshalErr := json.Marshal(map[string]string{"code": code, "next_action": recovery.Hint(code)})
	if marshalErr != nil {
		return mcp.NewToolResultError(code)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: mcp.ContentTypeText, Text: code},
			mcp.TextContent{Type: mcp.ContentTypeText, Text: string(hintPayload)},
		},
	}
}

// applyToolHints adds the read/destructive annotations hosts use for safety
// heuristics. Reads carry ReadOnlyHint; trash/cancel carry DestructiveHint.
func applyToolHints(tools []mcp.Tool) {
	readOnly := map[string]bool{
		"relay_targets": true, "capabilities": true, "bind_discover": true, "bind_list": true,
		"list": true, "search": true, "changes": true,
		"stat": true, "operation_get": true, "inbox": true, "prepare_reference": true,
		"mount_status":  true,
		"sync_plan_get": true, "retention_preview": true,
		"usage_summary": true, "usage_series": true, "usage_breakdown": true, "usage_scans": true,
		"audit_events": true,
	}
	destructive := map[string]bool{"trash": true, "cancel": true, "relay_cancel": true}
	idempotent := map[string]bool{"mkdir": true, "relay_cancel": true, "cancel": true}
	for i := range tools {
		name := tools[i].Name
		if readOnly[name] {
			tools[i].Annotations.ReadOnlyHint = &[]bool{true}[0]
		}
		if destructive[name] {
			tools[i].Annotations.DestructiveHint = &[]bool{true}[0]
		}
		if idempotent[name] {
			tools[i].Annotations.IdempotentHint = &[]bool{true}[0]
		}
	}
}

func applyBindScope(ctx context.Context, args map[string]any) context.Context {
	text := func(k string) string { v, _ := args[k].(string); return v }
	project := text("project")
	if project == "" {
		project = text("project_ref")
	}
	return WithBindScope(ctx, BindScope{ID: text("bind_id"), Consumer: text("consumer"), ProjectRef: project})
}
