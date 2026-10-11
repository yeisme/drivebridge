package service

import "time"

// Workspace discovery advertises only implemented actions after current
// application authorization. HTTP byte transport is not an MCP upload tool.
func (s *Service) workspaceCapabilities(p Principal) (map[string]any, error) {
	if p.Agent == nil {
		return nil, code("forbidden")
	}
	actions := []map[string]any{}
	tools := []string{"capabilities"}
	for _, action := range []string{"inbox.submit", "labels.suggest"} {
		if _, err := s.authorizeWorkspaceActionOn(s.DB, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, action, time.Now().UTC()); err != nil {
			continue
		}
		if action == "inbox.submit" {
			tools = append(tools, "operation_get", "complete_upload")
			actions = append(actions, map[string]any{"action": action, "method": "POST", "path": "/api/v1/inbox-submissions", "required_fields": []string{"name", "mime_type", "size_bytes", "sha256"}, "required_headers": []string{"Authorization", "Idempotency-Key"}, "lookup": map[string]any{"method": "POST", "path": "/api/v1/inbox-submissions/lookup", "required_fields": []string{"idempotency_key"}}, "parts": "PUT returned part_path with the same credential and context; replace {index} with the zero-based part index", "recovery": "operation_get received_parts, then complete_upload using the original operation_id", "status_path": "/api/v1/inbox-submissions/{id}", "list": map[string]any{"method": "GET", "path": "/api/v1/inbox-submissions", "query_fields": []string{"space_id", "state", "agent_id", "cursor", "limit"}, "visibility": "own_grant", "cursor_expired_status": 410}})
		} else {
			actions = append(actions, map[string]any{"action": action, "method": "POST", "path": "/api/v1/files/{id}/label-suggestions", "required_fields": []string{"version_id", "labels"}, "required_headers": []string{"Authorization", "Idempotency-Key"}, "recovery": "Replay the same request and Idempotency-Key, or list own suggestions for the fixed version and match creation_key; do not create a new key after a lost response.", "list": map[string]any{"method": "GET", "path": "/api/v1/files/{id}/label-suggestions", "required_query_fields": []string{"version_id"}, "query_fields": []string{"version_id", "cursor", "limit"}, "visibility": "own_agent", "cursor_expired_status": 410}})
		}
	}
	readReady := false
	if _, err := s.authorizeWorkspaceActionOn(s.DB, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, "files.read", time.Now().UTC()); err == nil {
		readReady = true
		tools = append(tools, "stat", "files", "search")
		actions = append(actions, map[string]any{"action": "files.read", "method": "GET", "path": "/api/v1/files", "required_query_fields": []string{"space_id"}, "query_fields": []string{"space_id", "q", "prefix", "mime_type", "after"}, "staging_readable": false})
		actions = append(actions, map[string]any{"action": "files.read", "method": "GET", "path": "/api/v1/files/{id}", "returns": "current file metadata and fixed version reference", "staging_readable": false})
		actions = append(actions, map[string]any{"action": "files.read", "method": "GET", "path": "/api/v1/files/{id}/content", "required_query_fields": []string{"version"}, "required_headers": []string{"Authorization"}, "staging_readable": false})
	}
	if len(actions) == 0 {
		return nil, code("forbidden")
	}
	return map[string]any{"restriction_profile": "personal_workspace_v1", "tools": tools, "http_actions": actions, "public_base_url": s.Config.Origin, "byte_transport": "authenticated_http", "local_file_access": false, "files_read_ready": readReady, "fixed_version_content_ready": readReady, "legacy_create_upload": "forbidden", "human_decisions": "not_delegated", "context_headers": []string{"X-DriveBridge-Tenant", "X-DriveBridge-Space"}}, nil
}
