package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSubmissionDecision(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.SubmissionDecisionRequest](r.Body)
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
	id := r.PathValue("id")
	payload, err := json.Marshal(struct {
		ID    string
		Input contract.SubmissionDecisionRequest
	}{id, input})
	if err != nil {
		return code("invalid_request")
	}
	digest := Hash(string(payload))
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := tx.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
			return code("forbidden")
		}
		var sub WorkspaceSubmission
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&sub).Error; err != nil {
			return code("not_found")
		}
		if sub.FileID == nil {
			return code("invalid_operation")
		}
		if (p.ScopeSpace != "" && p.ScopeSpace != sub.SpaceID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
			return code("forbidden")
		}
		var sp Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", sub.SpaceID, p.UserID, p.TenantID).First(&sp).Error; err != nil {
			return code("not_found")
		}
		var file File
		if err := tx.Where("id = ? AND owner = ? AND space_id = ?", *sub.FileID, p.UserID, sp.ID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
			return code("not_found")
		}
		if err := s.governanceAllow(tx, p, sp.ID, file.Path, "write"); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "submission.decision", keys[0]).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		decided, err := s.decideWorkspaceSubmissionOn(tx, p, id, input, now)
		if err != nil {
			return err
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "submission.decision", RequestKey: keys[0], RequestDigest: digest, ResourceID: id, ResultRevision: decided.Revision, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.submission." + input.Decision, Resource: id, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.workspaceSubmissionGet(w, r, p)
}
