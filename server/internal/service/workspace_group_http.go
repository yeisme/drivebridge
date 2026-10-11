package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func workspaceGroupView(group WorkspaceAgentGroup) contract.AgentGroupView {
	return contract.AgentGroupView{GroupID: group.ID, Name: group.Name, Revision: group.Revision, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt}
}

func (s *Service) workspaceGroupGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	var group WorkspaceAgentGroup
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&group).Error; err != nil {
			return code("not_found")
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, workspaceGroupView(group))
	return nil
}
