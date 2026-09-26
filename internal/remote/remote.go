// Package remote adapts local file access to the shared remote service API.
package remote

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yeisme/drivebridge-server/sdk"
	"github.com/yeisme/drivebridge/internal/bridge"
)

type Adapter struct {
	Client *sdk.Client
	Root   *os.Root
	// Surfaces consumes the server's public remote-surface contract
	// (batches, analytics, organization, connectors, change feeds). It is
	// attached only by an explicit opt-in and stays nil — invisible to
	// tools/list — otherwise.
	Surfaces *bridge.RemoteSurfaces
	endpoint string
	token    string
	// surfaceGap/surfaceDiscoveryErr record the last discovery outcome so
	// the capabilities resource can state the concrete gap.
	surfaceGap          []string
	surfaceDiscoveryErr string
	unifiedIdentity     bool
}

// UploadResult is the typed upload receipt: callers that need synchronous
// terminal semantics can poll the embedded operation by its ID.
type UploadResult struct {
	sdk.Operation
	FileRef string `json:"file_ref"`
}

func New(endpoint, credentialFile, localRoot string) (*Adapter, error) {
	return NewWithAigora(endpoint, credentialFile, localRoot, "")
}

// NewWithAigora retains local-only credential storage and exchanges the key
// centrally when aigoraOrigin is explicitly configured.
func NewWithAigora(endpoint, credentialFile, localRoot, aigoraOrigin string) (*Adapter, error) {
	i, e := os.Lstat(credentialFile)
	if e != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private_token_file_required")
	}
	b, e := os.ReadFile(credentialFile)
	if e != nil || len(b) > 8192 {
		return nil, errors.New("credential_unavailable")
	}
	var client *sdk.Client
	if aigoraOrigin != "" {
		client, e = sdk.NewAigora(endpoint, aigoraOrigin, string(bytesTrim(b)))
	} else {
		client, e = sdk.New(endpoint, string(bytesTrim(b)))
	}
	if e != nil {
		return nil, e
	}
	if !filepath.IsAbs(localRoot) {
		return nil, errors.New("absolute_local_root_required")
	}
	root, e := os.OpenRoot(localRoot)
	if e != nil {
		return nil, errors.New("local_root_unavailable")
	}
	return &Adapter{Client: client, Root: root, endpoint: endpoint, token: string(bytesTrim(b)), unifiedIdentity: aigoraOrigin != ""}, nil
}

// AttachRemoteSurfaces binds the remote-surface application contract to this
// adapter: the shared server SDK client stays the JSON transport, the byte
// path streams parts with the same credential, and the optional local root
// (when configured) becomes the bounded byte source. The thin adapter keeps
// no intent journal; the caller-held idempotency key governs recovery.
func (a *Adapter) AttachRemoteSurfaces() error {
	if a.Client == nil {
		return errors.New("service_unavailable")
	}
	a.Surfaces = bridge.NewRemoteSurfaces(a.endpoint, a.Client, &bridge.PartUploader{Base: a.endpoint, Token: a.token}, nil, a.Root)
	return nil
}
func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}
func (a *Adapter) Close() { a.Root.Close() }
func (a *Adapter) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	text := func(k string) string { v, _ := args[k].(string); return v }
	if args == nil {
		args = map[string]any{}
	}
	// Remote-surface actions run through the shared application contract
	// only after the explicit opt-in attached it; otherwise they stay
	// unsupported exactly like any unknown action.
	if a.Surfaces != nil && (bridge.IsRemoteSurfaceAction(name) || name == "upload_batch_file" || name == "remote_surfaces_status") {
		if name == "remote_surfaces_status" {
			return a.Surfaces.Status(ctx), nil
		}
		return a.Surfaces.Run(ctx, name, args)
	}
	if text("tenant_context") != "" || text("space_context") != "" {
		scoped := *a
		scoped.Client = a.Client.Scoped(text("tenant_context"), text("space_context"))
		a = &scoped
	}
	if text("ref") == "" && text("file_ref") != "" {
		args["ref"] = text("file_ref")
	}
	if text("ref") != "" {
		var resolved struct {
			File sdk.File `json:"file"`
			Ref  string   `json:"ref"`
		}
		if err := a.Client.JSON(ctx, "GET", "/api/v1/references?ref="+url.QueryEscape(text("ref")), "", nil, &resolved); err != nil {
			return nil, err
		}
		if text("file_id") != "" && text("file_id") != resolved.File.ID {
			return nil, errors.New("reference_conflict")
		}
		if text("version_id") != "" && text("version_id") != resolved.File.VersionID {
			return nil, errors.New("reference_conflict")
		}
		args["file_id"], args["version_id"] = resolved.File.ID, resolved.File.VersionID
		if name == "stat" {
			return resolved, nil
		}
	}

	var out any
	method, path := "GET", ""
	switch name {
	case "capabilities":
		path = "/api/v1/capabilities"
	case "spaces":
		path = "/api/v1/spaces"
	case "files", "list", "search":
		q := url.Values{"space_id": {text("space_id")}, "prefix": {text("path")}, "q": {text("query")}, "mime_type": {text("mime_type")}, "after": {text("after")}}
		path = "/api/v1/files?" + q.Encode()
	case "trash", "restore":
		method, path = "POST", "/api/v1/files/"+url.PathEscape(text("file_id"))+"/"+name
	case "mkdir", "changes", "purge":
		return nil, errors.New("remote_unsupported")
	case "stat":
		path = "/api/v1/files/" + url.PathEscape(text("file_id"))
	case "operations":
		path = "/api/v1/operations?space_id=" + url.QueryEscape(text("space_id"))
		if args["pending"] == true {
			path += "&pending=true"
		}
		if limit, ok := args["limit"].(int); ok {
			path += "&limit=" + strconv.Itoa(limit)
		}
	case "operation_get":
		path = "/api/v1/operations/" + url.PathEscape(text("operation_id"))
	case "resume", "cancel":
		method = "POST"
		path = "/api/v1/operations/" + url.PathEscape(text("operation_id")) + "/" + name
	case "upload":
		local := text("local_path")
		if !filepath.IsLocal(local) {
			return nil, errors.New("path_not_allowed")
		}
		f, e := a.Root.Open(local)
		if e != nil {
			return nil, errors.New("local_file_unavailable")
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() {
			return nil, errors.New("local_file_unavailable")
		}
		op, err := a.Client.Upload(ctx, sdk.UploadRequest{Caller: text("caller"), TaskRef: text("task_ref"), SpaceID: text("space_id"), Path: text("path"), FileID: text("file_id"), ExpectedVersion: text("expected_version")}, text("idempotency_key"), f, nil)
		if err != nil {
			return nil, err
		}
		var item struct {
			Ref string `json:"ref"`
		}
		if err = a.Client.JSON(ctx, "GET", "/api/v1/files/"+url.PathEscape(op.FileID), "", nil, &item); err != nil {
			// A write-only grant must still receive its successful operation receipt.
			return op, nil
		}
		if i := strings.LastIndex(item.Ref, "/version/"); i >= 0 {
			item.Ref = item.Ref[:i] + "/version/" + op.VersionID
		}
		return UploadResult{op, item.Ref}, nil
	case "download":
		local := text("local_path")
		if !filepath.IsLocal(local) {
			return nil, errors.New("path_not_allowed")
		}
		nonce := make([]byte, 16)
		if _, e := rand.Read(nonce); e != nil {
			return nil, errors.New("local_write_failed")
		}
		temp := filepath.Join(filepath.Dir(local), ".drivebridge-"+hex.EncodeToString(nonce))
		f, e := a.Root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if e != nil {
			return nil, errors.New("local_write_failed")
		}
		defer a.Root.Remove(temp)
		e = a.Client.Download(ctx, text("file_id"), text("version_id"), f)
		if e != nil {
			f.Close()
			return nil, e
		}
		if e = f.Sync(); e != nil {
			f.Close()
			return nil, errors.New("local_write_failed")
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			return nil, errors.New("local_write_failed")
		}
		f.Seek(0, 0)
		digest := sha256.New()
		io.Copy(digest, f)
		f.Close()
		if e = a.Root.Link(temp, local); e != nil {
			old, e := a.Root.Open(local)
			if e != nil {
				return nil, errors.New("target_exists_or_unavailable")
			}
			h := sha256.New()
			n, e := io.Copy(h, io.LimitReader(old, info.Size()+1))
			old.Close()
			if e != nil || n != info.Size() || hex.EncodeToString(h.Sum(nil)) != hex.EncodeToString(digest.Sum(nil)) {
				return nil, errors.New("target_conflict")
			}
		}
		return map[string]any{"status": "completed", "file_id": text("file_id"), "local_path": local}, nil

	default:
		return nil, errors.New("unknown_tool")
	}
	e := a.Client.JSON(ctx, method, path, "", nil, &out)
	return out, e
}
func (a *Adapter) MCP() *server.MCPServer {
	srv := server.NewMCPServer("drivebridge-remote-client", "0.1.0", server.WithToolCapabilities(false), server.WithResourceCapabilities(false, false))
	defs := []struct {
		name   string
		fields []string
	}{{"list", []string{"space_id"}}, {"search", []string{"space_id", "query"}}, {"trash", []string{"file_id"}}, {"restore", []string{"file_id"}}, {"capabilities", nil}, {"spaces", nil}, {"files", []string{"space_id"}}, {"stat", []string{"file_id"}}, {"operations", nil}, {"operation_get", []string{"operation_id"}}, {"resume", []string{"operation_id"}}, {"cancel", []string{"operation_id"}}, {"upload", []string{"space_id", "local_path", "path", "idempotency_key"}}, {"download", []string{"file_id", "local_path"}}}
	tools := []mcp.Tool{}
	for _, def := range defs {
		name := def.name
		opts := []mcp.ToolOption{mcp.WithDescription("Authenticated remote " + name + ". File paths are relative to this adapter's allowed local root; authorization is enforced by the file service.")}
		for _, field := range def.fields {
			opts = append(opts, mcp.WithString(field, mcp.Required()))
		}
		if name == "stat" || name == "download" || name == "trash" || name == "restore" {
			// file_id and ref are alternatives; runtime enforces their consistency.
			opts = []mcp.ToolOption{mcp.WithDescription("Use file_id or a pinned ref; local paths stay inside the adapter root."), mcp.WithString("file_id"), mcp.WithString("ref"), mcp.WithString("file_ref")}
			if name == "download" {
				opts = append(opts, mcp.WithString("local_path", mcp.Required()))
			}
		}
		if name == "files" || name == "list" || name == "search" {
			for _, field := range []string{"path", "mime_type", "after"} {
				opts = append(opts, mcp.WithString(field))
			}
		}
		if name == "upload" {
			opts = append(opts, mcp.WithString("caller"), mcp.WithString("task_ref"))
			opts = append(opts, mcp.WithString("file_id"), mcp.WithString("expected_version"))
		}
		if name == "download" {
			opts = append(opts, mcp.WithString("version_id"))
		}
		opts = append(opts, mcp.WithString("tenant_context", mcp.Description("Optional tenant ID; membership is verified by the service.")), mcp.WithString("space_context", mcp.Description("Optional selected space. Context cannot widen Agent permissions; approval_required requires human authorization in the service Web.")))
		tool := mcp.NewTool(name, opts...)
		tools = append(tools, tool)
		srv.AddTool(tool, func(ctx context.Context, q mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, e := a.Call(ctx, name, q.GetArguments())
			if e != nil {
				message := e.Error()
				if len(message) > 80 {
					message = "remote_operation_failed"
				}
				for _, r := range message {
					if !(r >= 'a' && r <= 'z') && r != '_' {
						message = "remote_operation_failed"
						break
					}
				}
				return mcp.NewToolResultError(message), nil
			}
			b, e := json.Marshal(out)
			if e != nil {
				return mcp.NewToolResultError("serialization_failed"), nil
			}
			return mcp.NewToolResultText(string(b)), nil
		})
	}
	if a.Surfaces != nil {
		// Discovery-driven registration: only actions the server currently
		// advertises become tools. A discovery failure registers only the
		// read-only status tool so the gap stays observable instead of
		// pretending availability.
		discoverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		surfaceTools, missing, discoverErr := a.Surfaces.DiscoverRemoteSurfaceTools(discoverCtx)
		cancel()
		for _, def := range surfaceTools {
			name := def.Name
			opts := []mcp.ToolOption{mcp.WithDescription(def.Description)}
			for _, field := range def.Required {
				opts = append(opts, mcp.WithString(field, mcp.Required(), mcp.Description("See the tool description and the service inputSchema")))
			}
			for _, field := range def.Optional {
				opts = append(opts, mcp.WithString(field, mcp.Description("Optional; see the tool description")))
			}
			tool := mcp.NewTool(name, opts...)
			if def.ReadOnly {
				tool.Annotations.ReadOnlyHint = &[]bool{true}[0]
			}
			tools = append(tools, tool)
			srv.AddTool(tool, func(ctx context.Context, q mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				out, e := a.Call(ctx, name, q.GetArguments())
				if e != nil {
					if detail, ok := e.(*bridge.RemoteError); ok && len(detail.MissingActions()) > 0 {
						hint, _ := json.Marshal(map[string]any{"code": detail.Error(), "next_action": detail.RecoveryHint(), "missing": detail.MissingActions()})
						return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{mcp.TextContent{Type: mcp.ContentTypeText, Text: detail.Error()}, mcp.TextContent{Type: mcp.ContentTypeText, Text: string(hint)}}}, nil
					}
					message := e.Error()
					if len(message) > 80 {
						message = "remote_operation_failed"
					}
					for _, r := range message {
						if !(r >= 'a' && r <= 'z') && r != '_' {
							message = "remote_operation_failed"
							break
						}
					}
					return mcp.NewToolResultError(message), nil
				}
				b, e := json.Marshal(out)
				if e != nil {
					return mcp.NewToolResultError("serialization_failed"), nil
				}
				return mcp.NewToolResultText(string(b)), nil
			})
		}
		statusTool := mcp.NewTool("remote_surfaces_status", mcp.WithDescription("Read-only discovery report for the remote surfaces: the attached instance, the actions the server advertises now, and the concrete catalog gap. Never guesses aliases."))
		statusTool.Annotations.ReadOnlyHint = &[]bool{true}[0]
		tools = append(tools, statusTool)
		srv.AddTool(statusTool, func(ctx context.Context, q mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, e := a.Call(ctx, "remote_surfaces_status", nil)
			if e != nil {
				return mcp.NewToolResultError(e.Error()), nil
			}
			b, e := json.Marshal(out)
			if e != nil {
				return mcp.NewToolResultError("serialization_failed"), nil
			}
			return mcp.NewToolResultText(string(b)), nil
		})
		a.surfaceGap = missing
		if discoverErr != nil {
			a.surfaceDiscoveryErr = discoverErr.Error()
		}
	}
	srv.AddResource(mcp.NewResource("drivebridge://capabilities", "Remote client capabilities", mcp.WithMIMEType("application/json")), func(ctx context.Context, q mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		out, e := a.Call(ctx, "capabilities", nil)
		if e != nil {
			return nil, e
		}
		b, _ := json.Marshal(map[string]any{"tools": tools, "service": out, "local_file_access": true, "unsupported_actions": []string{"mkdir", "changes", "purge"}, "after_semantics": "file_updated_at", "list_path_semantics": "recursive_prefix", "provenance": "caller_declared_labels", "remote_surfaces": map[string]any{"enabled": a.Surfaces != nil, "missing_actions": a.surfaceGap, "discovery_error": a.surfaceDiscoveryErr}})
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: q.Params.URI, MIMEType: "application/json", Text: string(b)}}, nil
	})

	srv.AddResource(mcp.NewResource("drivebridge://service-access", "Authentication and operation recovery", mcp.WithMIMEType("application/json")), func(ctx context.Context, q mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		body, err := json.Marshal(a.accessContract())
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: q.Params.URI, MIMEType: "application/json", Text: string(body)}}, nil
	})
	return srv
}

// accessContract is available even when the center is unavailable. It contains
// recovery instructions, never credentials or a claim that a write is enabled.
func (a *Adapter) accessContract() map[string]any {
	mode := "domain_credential"
	if a.unifiedIdentity {
		mode = "aigora_key_exchange"
	}
	return map[string]any{
		"schema_version": "1.0", "mode": mode, "client_cli_required": false,
		"parameters":             "Read tools/list inputSchema; capabilities reports current owner support.",
		"configuration_location": "MCP adapter host; the connected client does not need a product CLI.",
		"local_path_owner":       "MCP adapter host allowed root, not the connected client's filesystem.",
		"credential_policy":      "In aigora_key_exchange mode the configured key goes only to Aigora; the file service receives a short-lived audience-bound credential. Legacy mode uses the explicitly configured domain credential and is rejected after organization cutover.",
		"transfer_owner":         "DriveBridge HTTP API; MCP is the control surface.",
		"retry_policy":           "Query operation_get with the original operation_id after an uncertain result; retain idempotency_key. Never create a replacement operation automatically.",
		"billing":                "SaaS business quotas or an explicitly configured MCP product; never model tokens. Tool discovery does not grant entitlement or data permission.",
		"recovery": map[string]string{
			"identity_unavailable":       "No new writes or paid admission. Only permitted reads using a still-valid short-lived credential may continue.",
			"unified_login_required":     "The organization has cut over; the adapter host must configure the Aigora exchange. Do not retry legacy credentials.",
			"identity_mapping_required":  "A deployment operator must map the exact stable subject and organization. Never merge by email.",
			"service_admission_required": "This hosted write path is not enabled. Stop; do not switch credentials or direct-provider modes.",
			"approval_required":          "Request the owner's domain authorization; an administrator role alone grants no write authority.",
			"confirmation_required":      "Obtain confirmation for the exact payer, price and operation. Confirmation cannot override quota.",
		},
	}
}
