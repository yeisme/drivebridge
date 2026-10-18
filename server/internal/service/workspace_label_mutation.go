package service

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Every organization label writer takes the same file lock as selected-file
// merges and human suggestion acceptance before changing an assignment.
func (s *Service) organizationLabelMutation(p Principal, item OrganizationPlanItem, namespace, label string, remove bool, now time.Time) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var file File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", item.FileID, p.dataOwner()).First(&file).Error; err != nil {
			return resourceReadError(err)
		}
		if file.VersionID != item.VersionID || file.Trashed || file.PurgeStartedAt != nil {
			return code("version_conflict")
		}
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
			return err
		}
		if p.Agent != nil && namespace != "rule" && namespace != "suggested" {
			return code("label_namespace_forbidden")
		}
		if remove {
			if err := tx.Where("owner = ? AND file_id = ? AND namespace = ? AND label = ?", p.dataOwner(), file.ID, namespace, label).Delete(&FileLabelAssignment{}).Error; err != nil {
				return code("database_unavailable")
			}
			return nil
		}
		row := FileLabelAssignment{ID: ID(), Owner: p.dataOwner(), FileID: file.ID, Namespace: namespace, Label: label, AuthorKind: "rule", ActorID: p.UserID, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
}
