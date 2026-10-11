package service

import (
	"path"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// prepareWorkspaceArchiveOn validates intent without reserving quota or moving
// bytes. The eventual allocation must revalidate in its publication workflow.
func (s *Service) prepareWorkspaceArchiveOn(tx *gorm.DB, p Principal, id string, input contract.ArchiveSubmissionRequest) (ArchiveRequest, error) {
	var q ArchiveRequest
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return q, code("forbidden")
	}
	if input.VersionID == "" || input.TargetSpaceID == "" {
		return q, code("invalid_request")
	}
	if input.TargetPrefix != "" {
		cleaned, err := cleanPath(input.TargetPrefix)
		if err != nil || cleaned != input.TargetPrefix {
			return q, code("invalid_request")
		}
	}
	if workspaceStagingPath(input.TargetPrefix) {
		return q, code("forbidden")
	}
	var user User
	if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil || user.Disabled || user.Archived {
		return q, code("forbidden")
	}
	var sub WorkspaceSubmission
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&sub).Error; err != nil {
		return q, code("not_found")
	}
	if sub.State != "accepted" || sub.Revision != input.ExpectedRevision {
		return q, code("revision_conflict")
	}
	if sub.FileID == nil || sub.VersionID == nil || *sub.VersionID != input.VersionID {
		return q, code("version_conflict")
	}
	if (p.DataOwner != "" && p.DataOwner != p.UserID) || (p.ScopeSpace != "" && (p.ScopeSpace != sub.SpaceID || p.ScopeSpace != input.TargetSpaceID)) {
		return q, code("forbidden")
	}
	var spaces []Space
	if err := tx.Where("id IN ? AND owner = ? AND tenant_id = ?", []string{sub.SpaceID, input.TargetSpaceID}, p.UserID, p.TenantID).Find(&spaces).Error; err != nil {
		return q, code("database_unavailable")
	}
	expected := 2
	if sub.SpaceID == input.TargetSpaceID {
		expected = 1
	}
	if len(spaces) != expected {
		return q, code("not_found")
	}
	var file File
	if err := tx.Where("id = ? AND owner = ? AND space_id = ?", *sub.FileID, p.UserID, sub.SpaceID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return q, code("not_found")
	}
	if file.VersionID != input.VersionID {
		return q, code("version_conflict")
	}
	var version Version
	if err := tx.Where("id = ? AND owner = ? AND file_id = ?", input.VersionID, p.UserID, file.ID).First(&version).Error; err != nil {
		return q, code("version_conflict")
	}
	target := path.Join(input.TargetPrefix, path.Base(file.Path))
	if workspaceStagingPath(target) {
		return q, code("forbidden")
	}
	if err := s.governanceAllow(tx, p, sub.SpaceID, file.Path, "read"); err != nil {
		return q, err
	}
	if err := s.governanceAllow(tx, p, input.TargetSpaceID, target, "write"); err != nil {
		return q, err
	}
	return ArchiveRequest{SourceFileID: file.ID, SourceVersionID: input.VersionID, ExpectedSourceVersion: input.VersionID, TargetSpaceID: input.TargetSpaceID, TargetPath: target}, nil
}
