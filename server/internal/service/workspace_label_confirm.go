package service

import (
	"time"

	"gorm.io/gorm"
)

func (s *Service) confirmLabelMergePlan(p Principal, plan OrganizationPlan, q ConfirmOrganizationPlanRequest) (OrganizationPlanView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", plan.ID).First(&plan).Error; err != nil {
			return resourceReadError(err)
		}
		if err := s.labelMergePlanAuthorizeOn(tx, p, plan, true); err != nil {
			return err
		}
		if plan.Phase != "previewed" || q.ExpectedRevision != plan.Revision || q.ImpactDigest != plan.ImpactDigest {
			return code("plan_stale")
		}
		var items []OrganizationPlanItem
		if err := tx.Where("plan_id = ?", plan.ID).Order("file_id ASC").Limit(201).Find(&items).Error; err != nil {
			return code("database_unavailable")
		}
		if len(items) > 200 {
			return code("query_budget_exceeded")
		}
		for _, item := range items {
			file, err := s.labelMergeFileOn(tx, p, plan.SpaceID, item.FileID, item.VersionID, true)
			if err != nil {
				return err
			}
			if file.Path != item.Path {
				return code("plan_stale")
			}
			_, digest, err := labelMergeAssignmentsOn(tx, p.UserID, item.FileID)
			if err != nil {
				return err
			}
			if digest != item.BeforeValue {
				return code("plan_stale")
			}
		}
		result := tx.Model(&OrganizationPlan{}).Where("id = ? AND revision = ? AND phase = ?", plan.ID, q.ExpectedRevision, "previewed").Updates(map[string]any{"phase": "confirmed", "confirmed_by": p.UserID, "revision": plan.Revision + 1, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return code("plan_stale")
		}
		plan.Phase, plan.Revision = "confirmed", plan.Revision+1
		return nil
	})
	if err != nil {
		return OrganizationPlanView{}, err
	}
	return s.OrganizationPlanViewFor(p, plan, "", 50)
}
