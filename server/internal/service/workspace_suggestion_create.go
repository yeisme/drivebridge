package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// createWorkspaceSuggestionOn authorizes the fixed version and writes only a
// proposal. The caller transaction also owns audit and the HTTP receipt.
func (s *Service) createWorkspaceSuggestionOn(tx *gorm.DB, p Principal, fileID, key string, input contract.LabelSuggestionRequest, now time.Time) (WorkspaceLabelSuggestion, bool, error) {
	var result WorkspaceLabelSuggestion
	if fileID == "" || key == "" || len(key) > 200 {
		return result, false, code("invalid_request")
	}
	if err := input.Validate(); err != nil {
		return result, false, err
	}
	var file File
	if err := tx.Where("id = ? AND owner = ?", fileID, p.UserID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return result, false, code("not_found")
	}
	var version Version
	if err := tx.Where("id = ? AND file_id = ? AND owner = ?", input.VersionID, file.ID, p.UserID).First(&version).Error; err != nil {
		return result, false, code("not_found")
	}
	grant, err := s.authorizeWorkspaceActionOn(tx, p, file.SpaceID, version.ConnectionID, file.Path, "labels.suggest", now)
	if err != nil {
		return result, false, err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return result, false, code("invalid_request")
	}
	digest := Hash(string(data))
	err = tx.Where("owner = ? AND tenant_id = ? AND agent_id = ? AND file_id = ? AND version_id = ? AND creation_key = ?", p.UserID, p.TenantID, grant.AgentID, file.ID, version.ID, key).First(&result).Error
	if err == nil {
		if result.RequestDigest != digest {
			return WorkspaceLabelSuggestion{}, false, code("idempotency_conflict")
		}
		return result, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, false, code("database_unavailable")
	}
	result = WorkspaceLabelSuggestion{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, AgentID: grant.AgentID, GrantID: grant.ID, FileID: file.ID, VersionID: version.ID, CreationKey: key, RequestDigest: digest, Labels: append([]string(nil), input.Labels...), State: "pending", Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err = tx.Create(&result).Error; err != nil {
		return WorkspaceLabelSuggestion{}, false, code("database_unavailable")
	}
	return result, true, nil
}
