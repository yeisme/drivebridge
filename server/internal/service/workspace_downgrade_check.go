package service

// CheckWorkspaceDowngrade is a conservative preflight for an operator-managed
// downgrade. Disabled and expired grants still block: neither state is explicit
// retirement. Call only after stopping issuance and workers. A successful check
// does not certify compatibility of other schemas or stop an external binary.
func (s *Service) CheckWorkspaceDowngrade() error {
	var count int64
	if err := s.DB.Model(&WorkspaceGrant{}).Where("revoked = ?", false).Count(&count).Error; err != nil {
		return code("database_unavailable")
	}
	if count != 0 {
		return code("workspace_downgrade_blocked")
	}
	return nil
}
