package service

import (
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func workspaceSubmissionUploadView(sub WorkspaceSubmission) contract.SubmissionUploadView {
	return contract.SubmissionUploadView{SubmissionID: sub.ID, Revision: sub.Revision, OperationID: sub.UploadOperationID, Purpose: "inbox_staging", PartSize: PartSize, PartPath: "/api/v1/operations/" + sub.UploadOperationID + "/parts/{index}"}
}

func (s *Service) workspaceSubmissionCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent == nil {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.CreateSubmissionRequest](r.Body)
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
	var sub WorkspaceSubmission
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result, _, created, err := s.createWorkspaceSubmissionOn(tx, p, keys[0], input, now)
		if err != nil {
			return err
		}
		if created {
			audit := Audit{Owner: p.UserID, ActorID: p.UserID, AgentID: p.Agent.ID, TenantID: p.TenantID, Action: "workspace.submission.create", Resource: result.ID, Outcome: "success", At: now}
			if err := tx.Create(&audit).Error; err != nil {
				return code("audit_unavailable")
			}
		}
		sub = result
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	respond(w, workspaceSubmissionUploadView(sub))
	return nil
}

func (s *Service) workspaceSubmissionLookup(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent == nil {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](r.Body)
	if err != nil {
		return err
	}
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return code("invalid_request")
	}
	var sub WorkspaceSubmission
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		grant, err := s.authorizeWorkspaceActionOn(tx, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, "inbox.submit", now)
		if err != nil {
			return err
		}
		if err := tx.Where("owner = ? AND tenant_id = ? AND grant_id = ? AND creation_key = ?", p.UserID, p.TenantID, grant.ID, input.IdempotencyKey).First(&sub).Error; err != nil {
			return code("not_found")
		}
		_, _, err = s.workspaceOperationOn(tx, p, sub.UploadOperationID, now)
		return err
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, workspaceSubmissionUploadView(sub))
	return nil
}
