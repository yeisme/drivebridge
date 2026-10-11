package service

import (
	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"net/http"
	"time"
)

func (s *Service) workspaceSuggestionSubmit(w http.ResponseWriter, r *http.Request, p Principal) error {
	input, err := contract.DecodeRequest[contract.LabelSuggestionRequest](r.Body)
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
		now := time.Now().UTC()
		suggestion, created, err := s.createWorkspaceSuggestionOn(tx, p, r.PathValue("id"), keys[0], input, now)
		if err != nil {
			return err
		}
		if created {
			audit := Audit{Owner: p.UserID, ActorID: p.UserID, AgentID: suggestion.AgentID, TenantID: p.TenantID, Action: "workspace.suggestion.submit", Resource: suggestion.ID, Outcome: "success", At: now}
			if err = tx.Create(&audit).Error; err != nil {
				return code("audit_unavailable")
			}
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
