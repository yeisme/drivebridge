package service

import (
	"errors"

	"gorm.io/gorm"
)

// workspaceArchiveReplayOn recovers only the recorded operation, after current
// human, tenant, source and destination authorization. No bytes are moved.
func (s *Service) workspaceArchiveReplayOn(tx *gorm.DB, p Principal, id, key, digest string) (Operation, bool, error) {
	fail := func(err error) (Operation, bool, error) { return Operation{}, false, err }
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return fail(code("forbidden"))
	}
	var user User
	if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil || user.Disabled || user.Archived {
		return fail(code("forbidden"))
	}
	var receipt WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "submission.archive", key).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Operation{}, false, nil
	}
	if err != nil {
		return fail(code("database_unavailable"))
	}
	if receipt.RequestDigest != digest {
		return fail(code("idempotency_conflict"))
	}
	var sub WorkspaceSubmission
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&sub).Error; err != nil {
		return fail(code("not_found"))
	}
	if sub.ArchiveOperationID == nil || *sub.ArchiveOperationID != receipt.ResourceID || sub.FileID == nil || sub.VersionID == nil {
		return fail(code("invalid_archive"))
	}
	var op Operation
	if err := tx.Where("id = ? AND owner = ?", receipt.ResourceID, p.UserID).First(&op).Error; err != nil {
		return fail(code("not_found"))
	}
	if op.Kind != "archive" || op.ArchiveFileID != *sub.FileID || op.ArchiveVersionID != *sub.VersionID {
		return fail(code("invalid_archive"))
	}
	if (p.DataOwner != "" && p.DataOwner != p.UserID) || (p.ScopeSpace != "" && (p.ScopeSpace != sub.SpaceID || p.ScopeSpace != op.SpaceID)) {
		return fail(code("forbidden"))
	}
	var spaces []Space
	if err := tx.Where("id IN ? AND owner = ? AND tenant_id = ?", []string{sub.SpaceID, op.SpaceID}, p.UserID, p.TenantID).Find(&spaces).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	expected := 2
	if sub.SpaceID == op.SpaceID {
		expected = 1
	}
	if len(spaces) != expected {
		return fail(code("not_found"))
	}
	var source File
	if err := tx.Where("id = ? AND owner = ? AND space_id = ?", *sub.FileID, p.UserID, sub.SpaceID).First(&source).Error; err != nil || source.Trashed || source.PurgeStartedAt != nil {
		return fail(code("not_found"))
	}
	if err := s.governanceAllow(tx, p, sub.SpaceID, source.Path, "read"); err != nil {
		return fail(err)
	}
	if err := s.governanceAllow(tx, p, op.SpaceID, op.Path, "write"); err != nil {
		return fail(err)
	}
	return op, true, nil
}
