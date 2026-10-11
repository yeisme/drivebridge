package service

import (
	"math"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// decideWorkspaceSubmissionOn records a human decision without archiving bytes.
// The caller supplies the transaction, request receipt and audit.
func (s *Service) decideWorkspaceSubmissionOn(tx *gorm.DB, p Principal, id string, input contract.SubmissionDecisionRequest, now time.Time) (WorkspaceSubmission, error) {
	var sub WorkspaceSubmission
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return sub, code("forbidden")
	}
	if err := input.Validate(); err != nil {
		return sub, err
	}
	if input.ExpectedRevision == math.MaxUint64 {
		return sub, code("invalid_request")
	}
	var owner User
	if err := tx.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
		return sub, code("forbidden")
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&sub).Error; err != nil {
		return sub, code("not_found")
	}
	if sub.State != "submitted" || sub.Revision != input.ExpectedRevision {
		return sub, code("revision_conflict")
	}
	if sub.FileID == nil || sub.VersionID == nil || *sub.VersionID != input.VersionID {
		return sub, code("version_conflict")
	}
	if (p.ScopeSpace != "" && p.ScopeSpace != sub.SpaceID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
		return sub, code("forbidden")
	}
	var sp Space
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", sub.SpaceID, p.UserID, p.TenantID).First(&sp).Error; err != nil {
		return sub, code("not_found")
	}
	var file File
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ? AND space_id = ?", *sub.FileID, p.UserID, sp.ID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return sub, code("not_found")
	}
	if file.VersionID != input.VersionID {
		return sub, code("version_conflict")
	}
	var version Version
	if err := tx.Where("id = ? AND file_id = ? AND owner = ?", input.VersionID, file.ID, p.UserID).First(&version).Error; err != nil {
		return sub, code("version_conflict")
	}
	if err := s.governanceAllow(tx, p, sp.ID, file.Path, "write"); err != nil {
		return sub, err
	}
	state := "rejected"
	if input.Decision == "accept" {
		state = "accepted"
	}
	result := tx.Model(&WorkspaceSubmission{}).Where("id = ? AND revision = ? AND state = ?", sub.ID, input.ExpectedRevision, "submitted").Updates(map[string]any{"state": state, "revision": input.ExpectedRevision + 1, "updated_at": now})
	if result.Error != nil {
		return sub, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return sub, code("revision_conflict")
	}
	decision := WorkspaceSubmissionDecision{ID: ID(), SubmissionID: sub.ID, Revision: input.ExpectedRevision + 1, HumanID: p.UserID, VersionID: input.VersionID, Decision: input.Decision, Reason: input.Reason, CreatedAt: now}
	if err := tx.Create(&decision).Error; err != nil {
		return sub, code("database_unavailable")
	}
	sub.State = state
	sub.Revision++
	sub.UpdatedAt = now
	return sub, nil
}
