package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func workspaceOrganizationView(org WorkspaceAgentOrganization) contract.AgentOrganizationView {
	return contract.AgentOrganizationView{AgentID: org.AgentID, GroupID: org.GroupID, Pinned: org.Pinned, Revision: org.Revision, UpdatedAt: org.UpdatedAt}
}

func (s *Service) workspaceOrganizationPatch(w http.ResponseWriter, r *http.Request, p Principal) error {
	input, err := contract.DecodeRequest[contract.PatchOrganizationRequest](r.Body)
	if err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
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
	agentID := r.PathValue("agent_id")
	payload, err := json.Marshal(struct {
		AgentID string
		Input   contract.PatchOrganizationRequest
	}{agentID, input})
	if err != nil {
		return code("invalid_request")
	}
	digest := Hash(string(payload))
	var org WorkspaceAgentOrganization
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.authorizeWorkspaceOrganizationOn(tx, p, agentID); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "organization.patch", keys[0]).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ? AND agent_id = ?", receipt.ResourceID, p.UserID, p.TenantID, agentID).First(&org).Error; err != nil {
				return code("not_found")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		org, err = s.patchWorkspaceOrganizationOn(tx, p, agentID, input, now)
		if err != nil {
			return err
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "organization.patch", RequestKey: keys[0], RequestDigest: digest, ResourceID: org.ID, ResultRevision: org.Revision, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.organization.patch", Resource: agentID, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, workspaceOrganizationView(org))
	return nil
}
