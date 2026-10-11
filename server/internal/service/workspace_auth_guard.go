package service

import (
	"errors"

	"gorm.io/gorm"
)

// rejectWorkspaceLegacyAccess prevents a grant-bound identity from falling
// through legacy broad scopes. New action-specific authorization is separate.
// Database failures deny access; only a genuinely absent grant is legacy.
func rejectWorkspaceLegacyAccess(db *gorm.DB, agent Agent) error {
	if agent.ID == "" || agent.RestrictionProfile != "" {
		return code("forbidden")
	}
	var grant WorkspaceGrant
	err := db.Where("agent_id = ?", agent.ID).First(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return code("database_unavailable")
	}
	return code("forbidden")
}
