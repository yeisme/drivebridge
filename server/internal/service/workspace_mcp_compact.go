package service

import (
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func compactWorkspaceTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("workspace.search", mcp.WithDescription("Compact discovery across authorized files, operations and Agent submissions."), mcp.WithString("target_kind", mcp.Required()), mcp.WithString("query"), mcp.WithString("space_id"), mcp.WithString("cursor"), mcp.WithString("limit")),
		mcp.NewTool("workspace.inspect", mcp.WithDescription("Inspect one stable target without reading bytes or executing side effects."), mcp.WithString("target_kind", mcp.Required()), mcp.WithString("target_ref", mcp.Required())),
		mcp.NewTool("workspace.recover", mcp.WithDescription("Recover the original operation or submission result after an uncertain response."), mcp.WithString("target_kind", mcp.Required()), mcp.WithString("target_ref", mcp.Required())),
	}
}

// workspaceCompactSearch is a bounded read projection for the compact MCP
// surface. It deliberately returns identities and summaries, never bytes,
// credentials, provider URLs, or an executable action.
func (s *Service) workspaceCompactSearch(p Principal, kind, query, spaceID string, limit int) (map[string]any, error) {
	if kind == "" {
		return nil, code("invalid_request")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}
	if spaceID == "" && p.Agent != nil {
		spaceID = p.Agent.SpaceID
	}
	if spaceID == "" {
		return nil, code("invalid_space")
	}
	query = strings.TrimSpace(query)
	observed := time.Now().UTC()
	items := []map[string]any{}
	switch kind {
	case "file", "files":
		var files []File
		var err error
		if p.Agent != nil {
			files, err = s.searchWorkspaceFiles(p, spaceID, query, "", "", "", false)
		} else {
			files, err = s.SearchFiles(p, spaceID, query, "", "", "", false)
		}
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if len(items) >= limit {
				break
			}
			items = append(items, map[string]any{"kind": "file", "id": file.ID, "ref": s.FileRef(file), "space_id": file.SpaceID, "path": file.Path, "version_id": file.VersionID, "mime_type": file.MIME, "size_bytes": file.Size, "trashed": file.Trashed})
		}
	case "operation", "operations":
		q := s.DB.Where("owner = ?", p.dataOwner()).Order("updated DESC").Limit(limit)
		if spaceID != "" {
			q = q.Where("space_id = ?", spaceID)
		}
		if query != "" {
			q = q.Where("id LIKE ? OR path LIKE ?", "%"+query+"%", "%"+query+"%")
		}
		var operations []Operation
		if err := q.Find(&operations).Error; err != nil {
			return nil, code("database_unavailable")
		}
		for _, op := range operations {
			if s.allow(p, op.SpaceID, op.Path, "read") != nil {
				continue
			}
			items = append(items, map[string]any{"kind": "operation", "id": op.ID, "space_id": op.SpaceID, "status": op.Status, "purpose": op.Purpose, "bytes_transferred": op.Transferred, "size_bytes": op.Size, "updated_at": op.Updated})
		}
	case "submission", "submissions":
		if p.UserID == "" {
			return nil, code("forbidden")
		}
		q := s.DB.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID).Order("updated_at DESC").Limit(limit)
		if spaceID != "" {
			q = q.Where("space_id = ?", spaceID)
		}
		if query != "" {
			q = q.Where("id LIKE ? OR agent_id LIKE ?", "%"+query+"%", "%"+query+"%")
		}
		var submissions []WorkspaceSubmission
		if err := q.Find(&submissions).Error; err != nil {
			return nil, code("database_unavailable")
		}
		for _, submission := range submissions {
			view, err := s.workspaceSubmissionViewOn(s.DB, p, submission.ID)
			if err != nil {
				if safeCode(err) == "not_found" || safeCode(err) == "forbidden" {
					continue
				}
				return nil, err
			}
			items = append(items, map[string]any{"kind": "submission", "id": view.SubmissionID, "space_id": view.SpaceID, "state": view.State, "file_name": view.FileName, "agent_name": view.AgentName, "size_bytes": view.SizeBytes, "operation_id": view.UploadOperationRef, "created_at": view.CreatedAt, "updated_at": view.UpdatedAt})
		}
	default:
		return nil, code("unsupported_action")
	}
	return map[string]any{"status": "success", "items": items, "next_cursor": nil, "coverage": map[string]any{"kind": kind, "partial": len(items) >= limit}, "observed_at": observed}, nil
}

func (s *Service) workspaceCompactInspect(p Principal, kind, ref string) (map[string]any, error) {
	if kind == "" || ref == "" {
		return nil, code("invalid_request")
	}
	switch kind {
	case "file":
		if strings.HasPrefix(ref, "drivebridge://") {
			return s.PinnedFile(p, ref)
		}
		file, err := s.file(p, ref, "read")
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "file", "file": map[string]any{"id": file.ID, "space_id": file.SpaceID, "path": file.Path, "version_id": file.VersionID, "mime_type": file.MIME, "size_bytes": file.Size, "trashed": file.Trashed}, "ref": s.FileRef(file)}, nil
	case "operation":
		if p.Agent != nil {
			return s.workspaceOperationRecovery(p, ref)
		}
		op, err := s.operation(p, ref, "read")
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "operation", "operation": workspaceOperationProgress(op)}, nil
	case "submission":
		var view contract.SubmissionView
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			var err error
			view, err = s.workspaceSubmissionViewOn(tx, p, ref)
			return err
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "submission", "submission": view}, nil
	default:
		return nil, code("unsupported_action")
	}
}

func (s *Service) workspaceCompactRecover(p Principal, kind, ref string) (map[string]any, error) {
	if kind == "operation" {
		if p.Agent != nil {
			return s.workspaceOperationRecovery(p, ref)
		}
		op, err := s.operation(p, ref, "read")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "success", "certainty": "observed", "operation": workspaceOperationProgress(op), "next_action": "operation.get"}, nil
	}
	if kind == "submission" {
		return s.workspaceCompactInspect(p, "submission", ref)
	}
	return nil, code("unsupported_action")
}
