package service

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
)

func workspaceGroupName(value string) (string, string, error) {
	name := strings.TrimSpace(value)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
		return "", "", code("invalid_request")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", "", code("invalid_request")
		}
	}
	return name, strings.ToLower(name), nil
}

func workspaceGroupHumanOn(tx *gorm.DB, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" || (p.DataOwner != "" && p.DataOwner != p.UserID) {
		return code("forbidden")
	}
	var user User
	if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil || user.Disabled || user.Archived {
		return code("forbidden")
	}
	return nil
}

// The caller owns the transaction and request receipt for group mutations.
func (s *Service) createWorkspaceGroupOn(tx *gorm.DB, p Principal, value string, now time.Time) (WorkspaceAgentGroup, error) {
	var group WorkspaceAgentGroup
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return group, err
	}
	name, normalized, err := workspaceGroupName(value)
	if err != nil {
		return group, err
	}
	var count int64
	if err := tx.Model(&WorkspaceAgentGroup{}).Where("owner = ? AND tenant_id = ? AND normalized_name = ?", p.UserID, p.TenantID, normalized).Count(&count).Error; err != nil {
		return group, code("database_unavailable")
	}
	if count != 0 {
		return group, code("name_conflict")
	}
	group = WorkspaceAgentGroup{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, Name: name, NormalizedName: normalized, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&group).Error; err != nil {
		return WorkspaceAgentGroup{}, code("database_unavailable")
	}
	return group, nil
}

func (s *Service) renameWorkspaceGroupOn(tx *gorm.DB, p Principal, id, value string, revision uint64, now time.Time) (WorkspaceAgentGroup, error) {
	var group WorkspaceAgentGroup
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return group, err
	}
	name, normalized, err := workspaceGroupName(value)
	if err != nil {
		return group, err
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&group).Error; err != nil {
		return group, code("not_found")
	}
	if revision == 0 || revision == math.MaxUint64 || group.Revision != revision {
		return group, code("revision_conflict")
	}
	var count int64
	if err := tx.Model(&WorkspaceAgentGroup{}).Where("owner = ? AND tenant_id = ? AND normalized_name = ? AND id <> ?", p.UserID, p.TenantID, normalized, id).Count(&count).Error; err != nil {
		return group, code("database_unavailable")
	}
	if count != 0 {
		return group, code("name_conflict")
	}
	result := tx.Model(&WorkspaceAgentGroup{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, p.UserID, p.TenantID, revision).Updates(map[string]any{"name": name, "normalized_name": normalized, "revision": revision + 1, "updated_at": now})
	if result.Error != nil {
		return group, code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return group, code("revision_conflict")
	}
	group.Name = name
	group.NormalizedName = normalized
	group.Revision++
	group.UpdatedAt = now
	return group, nil
}
