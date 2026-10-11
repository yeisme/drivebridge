package service

import "time"

// WorkspaceInvocation is an observed transport call, never a business receipt.
// No request headers, arguments, response payloads, paths or credentials belong
// here. Missing records are possible when telemetry persistence is unavailable.
type WorkspaceInvocation struct {
	ID         string `gorm:"primaryKey"`
	Owner      string `gorm:"not null;index:workspace_invocation_owner"`
	TenantID   string `gorm:"not null;index:workspace_invocation_owner"`
	SpaceID    string `gorm:"not null"`
	AgentID    string `gorm:"not null;index"`
	GrantID    string `gorm:"not null"`
	Protocol   string `gorm:"not null"`
	Action     string `gorm:"not null"`
	RequestID  string `gorm:"not null;uniqueIndex"`
	Result     string `gorm:"not null"`
	ErrorCode  string
	StartedAt  time.Time `gorm:"not null;index:workspace_invocation_owner"`
	FinishedAt *time.Time
}

func (s *Service) startWorkspaceInvocation(p Principal, protocol, action string) *WorkspaceInvocation {
	if p.Agent == nil || p.Agent.RestrictionProfile != "personal_workspace_v1" {
		return nil
	}
	if protocol != "http" && protocol != "mcp" {
		return nil
	}
	// Action is supplied by the fixed route/tool registry, not client input.
	if action == "" || len(action) > 128 {
		return nil
	}
	for _, r := range action {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '.' && r != '_' {
			return nil
		}
	}
	var grant WorkspaceGrant
	if err := s.DB.Where("agent_id = ? AND owner = ? AND tenant_id = ?", p.Agent.ID, p.UserID, p.TenantID).First(&grant).Error; err != nil {
		return nil
	}
	if grant.SpaceID != p.Agent.SpaceID || grant.ConnectionID != p.Agent.ConnectionID {
		return nil
	}
	row := WorkspaceInvocation{ID: ID(), RequestID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: grant.SpaceID, AgentID: p.Agent.ID, GrantID: grant.ID, Protocol: protocol, Action: action, Result: "unknown", StartedAt: time.Now().UTC()}
	if err := s.DB.Create(&row).Error; err != nil {
		return nil
	}
	return &row
}

func (s *Service) finishWorkspaceInvocation(row *WorkspaceInvocation, err error) {
	if row == nil {
		return
	}
	// A handler returning nil says nothing about async work, partial output or
	// later publication. A transport error can likewise follow a committed write.
	result, errorCode := "unknown", ""
	if err != nil {
		switch safeCode(err) {
		case "unauthorized", "forbidden", "approval_required":
			result = "denied"
			errorCode = safeCode(err)
		case "not_found", "invalid_request", "invalid_query", "unsupported", "version_conflict", "revision_conflict", "idempotency_conflict", "database_unavailable", "storage_unavailable", "audit_unavailable":
			errorCode = safeCode(err)
		default:
			errorCode = "invocation_error"
		}
	}
	now := time.Now().UTC()
	// Best effort: changing a completed business response on observation failure
	// would invite duplicate writes. Failed finish leaves the persisted start
	// unknown with no completion timestamp, never an invented success.
	_ = s.DB.Model(&WorkspaceInvocation{}).Where("id = ? AND finished_at IS NULL", row.ID).Updates(map[string]any{"finished_at": now, "result": result, "error_code": errorCode}).Error
}
