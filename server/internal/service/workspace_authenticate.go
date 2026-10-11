package service

import (
	"net/http"
	"strings"
	"time"
)

// Only explicitly implemented dedicated endpoints admit workspace credentials.
// Authentication alone never enables a legacy broad-scope route.
func (s *Service) authenticateWorkspaceAgent(r *http.Request, a Agent) (Principal, error) {
	denied := func() (Principal, error) { return Principal{}, code("unauthorized") }
	if a.RestrictionProfile != "personal_workspace_v1" || a.Revoked || a.Archived || a.ArchiveSuspended {
		return denied()
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	required := ""
	if r.Method == "GET" && r.URL.Path == "/api/v1/files" {
		required = "files.read"
	}
	if r.Method == "GET" && len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "files" && parts[3] != "" {
		required = "files.read"
	}
	if r.Method == "GET" && len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "files" && parts[3] != "" && parts[4] == "content" {
		required = "files.read"
	}
	if (r.Method == "POST" || r.Method == "GET") && len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "files" && parts[3] != "" && parts[4] == "label-suggestions" {
		required = "labels.suggest"
	}
	if r.Method == "POST" && (r.URL.Path == "/api/v1/inbox-submissions" || r.URL.Path == "/api/v1/inbox-submissions/lookup") {
		required = "inbox.submit"
	}
	if r.Method == "GET" && r.URL.Path == "/api/v1/inbox-submissions" {
		required = "inbox.submit"
	}
	if r.Method == "PUT" && len(parts) == 6 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "operations" && parts[3] != "" && parts[4] == "parts" && parts[5] != "" {
		required = "inbox.submit"
	}
	if r.Method == "GET" && len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && (parts[2] == "operations" || parts[2] == "inbox-submissions") && parts[3] != "" {
		required = "inbox.submit"
	}
	if r.Method == "POST" && len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "operations" && parts[3] != "" && parts[4] == "complete" {
		required = "inbox.submit"
	}
	mcpSession := r.URL.Path == "/mcp" && (r.Method == "POST" || r.Method == "GET" || r.Method == "DELETE")
	if required == "" && !mcpSession {
		return denied()
	}
	var grant WorkspaceGrant
	if err := s.DB.Where("agent_id = ? AND owner = ?", a.ID, a.Owner).First(&grant).Error; err != nil || workspaceGrantState(grant, time.Now().UTC()) != "active" {
		return denied()
	}
	if grant.SpaceID != a.SpaceID || grant.ConnectionID != a.ConnectionID || grant.Prefix != a.Prefix {
		return denied()
	}
	var owner User
	if err := s.DB.Where("id = ?", a.Owner).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
		return denied()
	}
	allowed := false
	for _, action := range strings.Split(grant.Actions, ",") {
		switch action {
		case "labels.suggest", "files.read", "inbox.submit":
			if action == required || mcpSession {
				allowed = true
			}
		default:
			return denied()
		}
	}
	if !allowed {
		return denied()
	}
	return Principal{UserID: a.Owner, TenantID: grant.TenantID, ScopeSpace: grant.SpaceID, Agent: &a}, nil
}
