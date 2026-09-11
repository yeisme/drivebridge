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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yeisme/drivebridge-server/sdk"
)

type Adapter struct {
	Client *sdk.Client
	Root   *os.Root
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
	var out any
	method, path := "GET", ""
	switch name {
	case "capabilities":
		path = "/api/v1/capabilities"
	case "spaces":
		path = "/api/v1/spaces"
	case "files":
		path = "/api/v1/files?space_id=" + url.QueryEscape(text("space_id"))
	case "stat":
		path = "/api/v1/files/" + url.PathEscape(text("file_id"))
	case "operations":
		path = "/api/v1/operations"
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
		return a.Client.Upload(ctx, sdk.UploadRequest{SpaceID: text("space_id"), Path: text("path"), FileID: text("file_id"), ExpectedVersion: text("expected_version")}, text("idempotency_key"), f, nil)
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
	}{{"capabilities", nil}, {"spaces", nil}, {"files", []string{"space_id"}}, {"stat", []string{"file_id"}}, {"operations", nil}, {"operation_get", []string{"operation_id"}}, {"resume", []string{"operation_id"}}, {"cancel", []string{"operation_id"}}, {"upload", []string{"space_id", "local_path", "path", "idempotency_key"}}, {"download", []string{"file_id", "local_path"}}}
	tools := []mcp.Tool{}
	for _, def := range defs {
		name := def.name
		opts := []mcp.ToolOption{mcp.WithDescription("Authenticated remote " + name + ". File paths are relative to this adapter's allowed local root; authorization is enforced by the file service.")}
		for _, field := range def.fields {
			opts = append(opts, mcp.WithString(field, mcp.Required()))
		}
		if name == "upload" {
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
		b, _ := json.Marshal(map[string]any{"tools": tools, "service": out, "local_file_access": true})
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: q.Params.URI, MIMEType: "application/json", Text: string(b)}}, nil
	})
	return srv
}
