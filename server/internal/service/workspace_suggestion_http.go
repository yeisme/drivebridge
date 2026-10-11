package service

import (
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSuggestionDecision(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent != nil || p.UserID == "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.SuggestionDecisionRequest](r.Body)
	if err != nil {
		return err
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" || len(keys[0]) > 200 {
		return code("invalid_request")
	}
	for _, ch := range keys[0] {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	var view contract.LabelSuggestionView
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		receipt, err := s.decideWorkspaceSuggestionRequestOn(tx, p, r.PathValue("id"), keys[0], input, time.Now().UTC())
		if err != nil {
			return err
		}
		var suggestion WorkspaceLabelSuggestion
		if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&suggestion).Error; err != nil {
			return code("not_found")
		}
		view = contract.LabelSuggestionView{SuggestionID: suggestion.ID, Revision: suggestion.Revision, FileID: suggestion.FileID, VersionID: suggestion.VersionID, AgentID: suggestion.AgentID, Labels: suggestion.Labels, CreationKey: suggestion.CreationKey, State: suggestion.State}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, view)
	return nil
}
