package service

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// Discovery is a projection only; CallTool independently enforces authority.
func (s *Service) workspaceToolFilter(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	p, ok := ctx.Value(principalKey{}).(Principal)
	if !ok {
		return []mcp.Tool{}
	}
	if p.Agent == nil {
		return tools
	}
	if p.Agent.RestrictionProfile == "" {
		if err := rejectWorkspaceLegacyAccess(s.DB, *p.Agent); err != nil {
			return []mcp.Tool{}
		}
		var current Agent
		if err := s.DB.Where("id = ? AND owner = ?", p.Agent.ID, p.UserID).First(&current).Error; err != nil || current.RestrictionProfile != "" {
			return []mcp.Tool{}
		}
		return tools
	}
	if p.Agent.RestrictionProfile != "personal_workspace_v1" {
		return []mcp.Tool{}
	}
	capabilities, err := s.workspaceCapabilities(p)
	if err != nil {
		return []mcp.Tool{}
	}
	names, _ := capabilities["tools"].([]string)
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	filtered := make([]mcp.Tool, 0, len(names))
	for _, tool := range tools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
