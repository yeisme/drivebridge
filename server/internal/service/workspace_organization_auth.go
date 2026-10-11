package service

import "gorm.io/gorm"

func (s *Service) authorizeWorkspaceOrganizationOn(tx *gorm.DB, p Principal, agentID string) error {
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return err
	}
	var agent Agent
	if err := tx.Where("id = ? AND owner = ?", agentID, p.UserID).First(&agent).Error; err != nil || agent.Archived {
		return code("not_found")
	}
	var space Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", agent.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
		return code("not_found")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != space.ID {
		return code("not_found")
	}
	return s.governanceAllow(tx, p, space.ID, agent.Prefix, "write")
}
