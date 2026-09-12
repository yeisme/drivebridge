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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yeisme/drivebridge-server/sdk"
)

type Adapter struct {
	Client *sdk.Client
	Root   *os.Root
}

// UploadResult is the typed upload receipt: callers that need synchronous
// terminal semantics can poll the embedded operation by its ID.
type UploadResult struct {
	sdk.Operation
	FileRef string `json:"file_ref"`
}

func New(endpoint, credentialFile, localRoot string) (*Adapter, error) {
	i, e := os.Lstat(credentialFile)
	if e != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private_token_file_required")
	}
	b, e := os.ReadFile(credentialFile)
	if e != nil || len(b) > 8192 {
		return nil, errors.New("credential_unavailable")
	}
	client, e := sdk.New(endpoint, string(bytesTrim(b)))
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
	return &Adapter{client, root}, nil
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
	srv.AddResource(mcp.NewResource("drivebridge://capabilities", "Remote client capabilities", mcp.WithMIMEType("application/json")), func(ctx context.Context, q mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		out, e := a.Call(ctx, "capabilities", nil)
		if e != nil {
			return nil, e
		}
		b, _ := json.Marshal(map[string]any{"tools": tools, "service": out, "local_file_access": true, "unsupported_actions": []string{"mkdir", "changes", "purge"}, "after_semantics": "file_updated_at", "list_path_semantics": "recursive_prefix", "provenance": "caller_declared_labels"})
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: q.Params.URI, MIMEType: "application/json", Text: string(b)}}, nil
	})
	return srv
}
