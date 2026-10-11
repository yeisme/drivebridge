package service

import (
	"errors"

	"gorm.io/gorm"
)

// Policy revisions are checked at admission and again in each item's locked
// transaction. A prior preview is not continuing permission to dispatch.
func retentionPlanCurrentOn(tx *gorm.DB, plan RetentionPlan) error {
	switch plan.Status {
	case "preview", "executing", "completed", "failed":
	default:
		return code("retention_plan_stale")
	}
	if plan.Category != "trash" && plan.Category != "historical_versions" {
		return code("retention_plan_stale")
	}
	var policy RetentionPolicy
	if err := tx.First(&policy, "owner = ? AND category = ?", plan.Owner, plan.Category).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if plan.PolicyRevision != 0 {
			return code("retention_plan_stale")
		}
	} else if policy.Revision != plan.PolicyRevision {
		return code("retention_plan_stale")
	}
	return nil
}
