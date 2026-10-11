package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// decideWorkspaceSuggestionRequestOn requires the caller's transaction. Replay
// rechecks current human access before returning a secret-free original receipt.
func (s *Service) decideWorkspaceSuggestionRequestOn(tx *gorm.DB, p Principal, id, key string, input contract.SuggestionDecisionRequest, now time.Time) (WorkspaceRequest, error) {
	var receipt WorkspaceRequest
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return receipt, code("forbidden")
	}
	if id == "" || key == "" || len(key) > 200 {
		return receipt, code("invalid_request")
	}
	if err := input.Validate(); err != nil {
		return receipt, err
	}
	var human User
	if err := tx.Where("id = ?", p.UserID).First(&human).Error; err != nil || human.Disabled || human.Archived {
		return receipt, code("forbidden")
	}
	var suggestion WorkspaceLabelSuggestion
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&suggestion).Error; err != nil {
		return receipt, code("not_found")
	}
	var file File
	if err := tx.Where("id = ? AND owner = ?", suggestion.FileID, p.UserID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return receipt, code("not_found")
	}
	var space Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
		return receipt, code("not_found")
	}
	if (p.ScopeSpace != "" && p.ScopeSpace != space.ID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
		return receipt, code("forbidden")
	}
	if err := s.governanceAllow(tx, p, space.ID, file.Path, "write"); err != nil {
		return receipt, err
	}
	payload, err := json.Marshal(struct {
		ID    string
		Input contract.SuggestionDecisionRequest
	}{id, input})
	if err != nil {
		return receipt, code("invalid_request")
	}
	digest := Hash(string(payload))
	err = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "suggestion.decision", key).First(&receipt).Error
	if err == nil {
		if receipt.RequestDigest != digest {
			return WorkspaceRequest{}, code("idempotency_conflict")
		}
		return receipt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return receipt, code("database_unavailable")
	}
	decided, err := s.decideWorkspaceSuggestionOn(tx, p, id, input, now)
	if err != nil {
		return receipt, err
	}
	receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "suggestion.decision", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: decided.Revision, CreatedAt: now}
	if err = tx.Create(&receipt).Error; err != nil {
		return WorkspaceRequest{}, code("database_unavailable")
	}
	audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.suggestion." + input.Decision, Resource: id, Outcome: "success", At: now}
	if err = tx.Create(&audit).Error; err != nil {
		return WorkspaceRequest{}, code("audit_unavailable")
	}
	return receipt, nil
}
