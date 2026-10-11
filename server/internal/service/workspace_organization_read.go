package service

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

func (s *Service) workspaceOrganizationGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	id := r.PathValue("agent_id")
	org := WorkspaceAgentOrganization{AgentID: id}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.authorizeWorkspaceOrganizationOn(tx, p, id); err != nil {
			return err
		}
		var stored WorkspaceAgentOrganization
		err := tx.Where("owner = ? AND tenant_id = ? AND agent_id = ?", p.UserID, p.TenantID, id).First(&stored).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return code("database_unavailable")
		}
		org = stored
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, workspaceOrganizationView(org))
	return nil
}
