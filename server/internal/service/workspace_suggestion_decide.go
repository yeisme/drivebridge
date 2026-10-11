package service

import (
	"math"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// decideWorkspaceSuggestionOn merges a human-selected subset into manual
// labels. The caller owns the transaction, idempotency receipt and audit.
func (s *Service) decideWorkspaceSuggestionOn(tx *gorm.DB, p Principal, id string, input contract.SuggestionDecisionRequest, now time.Time) (WorkspaceLabelSuggestion, error) {
	var suggestion WorkspaceLabelSuggestion
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return suggestion, code("forbidden")
	}
	if input.ExpectedRevision == math.MaxUint64 {
		return suggestion, code("invalid_request")
	}
	if err := input.Validate(); err != nil {
		return suggestion, err
	}
	var human User
	if err := tx.Where("id = ?", p.UserID).First(&human).Error; err != nil || human.Disabled || human.Archived {
		return suggestion, code("forbidden")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&suggestion).Error; err != nil {
		return suggestion, code("not_found")
	}
	if suggestion.Revision != input.ExpectedRevision || suggestion.State != "pending" {
		return WorkspaceLabelSuggestion{}, code("revision_conflict")
	}
	var file File
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", suggestion.FileID, p.UserID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return WorkspaceLabelSuggestion{}, code("not_found")
	}
	if file.VersionID != input.BaseVersionID || suggestion.VersionID != input.BaseVersionID {
		return WorkspaceLabelSuggestion{}, code("version_conflict")
	}
	var space Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
		return WorkspaceLabelSuggestion{}, code("not_found")
	}
	if (p.ScopeSpace != "" && p.ScopeSpace != space.ID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
		return WorkspaceLabelSuggestion{}, code("forbidden")
	}
	if err := s.governanceAllow(tx, p, space.ID, file.Path, "write"); err != nil {
		return WorkspaceLabelSuggestion{}, err
	}
	allowed := map[string]bool{}
	for _, label := range suggestion.Labels {
		allowed[label] = true
	}
	for _, label := range input.Labels {
		if !allowed[label] {
			return WorkspaceLabelSuggestion{}, code("invalid_request")
		}
	}
	state := "rejected"
	if input.Decision == "accept" {
		state = "accepted"
	}
	result := tx.Model(&WorkspaceLabelSuggestion{}).Where("id = ? AND revision = ? AND state = ?", id, input.ExpectedRevision, "pending").Updates(map[string]any{"state": state, "revision": input.ExpectedRevision + 1, "updated_at": now})
	if result.Error != nil {
		return WorkspaceLabelSuggestion{}, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return WorkspaceLabelSuggestion{}, code("revision_conflict")
	}
	rows := make([]FileLabelAssignment, 0, len(input.Labels))
	for _, label := range input.Labels {
		rows = append(rows, FileLabelAssignment{ID: ID(), Owner: p.UserID, FileID: file.ID, Namespace: "manual", Label: label, AuthorKind: "human", ActorID: p.UserID, AcceptedFrom: suggestion.ID, CreatedAt: now, UpdatedAt: now})
	}
	if len(rows) > 0 {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner"}, {Name: "file_id"}, {Name: "namespace"}, {Name: "label"}}, DoNothing: true}).Create(&rows).Error; err != nil {
			return WorkspaceLabelSuggestion{}, code("database_unavailable")
		}
	}
	decision := WorkspaceSuggestionDecision{ID: ID(), SuggestionID: id, Revision: input.ExpectedRevision + 1, HumanID: p.UserID, VersionID: file.VersionID, Decision: input.Decision, AcceptedLabels: append([]string(nil), input.Labels...), CreatedAt: now}
	if err := tx.Create(&decision).Error; err != nil {
		return WorkspaceLabelSuggestion{}, code("database_unavailable")
	}
	suggestion.State = state
	suggestion.Revision++
	suggestion.UpdatedAt = now
	return suggestion, nil
}
