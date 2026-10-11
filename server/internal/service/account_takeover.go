package service

import (
	"encoding/json"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type accountHandoff struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
}

func accountHandoffDigest(plan []accountHandoff) string {
	data, _ := json.Marshal(plan)
	return Hash(string(data))
}

// Plan only responsibilities with no other active successor. Merely being a
// platform administrator never grants these data-writing roles implicitly.
func accountHandoffs(tx *gorm.DB, user string) ([]accountHandoff, error) {
	plan := []accountHandoff{}
	active := tx.Model(&User{}).Select("id").Where("disabled = ?", false)
	var memberships []Membership
	if err := tx.Where("user_id = ? AND role = ? AND status = ?", user, "admin", "active").Find(&memberships).Error; err != nil {
		return nil, err
	}
	for _, m := range memberships {
		var n int64
		if err := tx.Model(&Membership{}).Where("tenant_id = ? AND user_id <> ? AND role = ? AND status = ?", m.TenantID, user, "admin", "active").Where("user_id IN (?)", active).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			var t Tenant
			if err := tx.First(&t, "id = ?", m.TenantID).Error; err != nil {
				return nil, err
			}
			plan = append(plan, accountHandoff{"tenant_admin", t.ID, t.ID, t.Name})
		}
	}
	var teams []Team
	if err := tx.Where("id IN (?)", tx.Model(&TeamMember{}).Select("team_id").Where("user_id = ? AND role = ?", user, "admin")).Find(&teams).Error; err != nil {
		return nil, err
	}
	for _, t := range teams {
		var n int64
		if err := tx.Model(&TeamMember{}).Where("team_id = ? AND user_id <> ? AND role = ?", t.ID, user, "admin").Where("user_id IN (?)", active).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			plan = append(plan, accountHandoff{"team_admin", t.ID, t.TenantID, t.Name})
		}
	}
	var spaces []Space
	if err := tx.Where("id IN (?)", tx.Model(&SpacePermission{}).Select("space_id").Where("user_id = ? AND role = ?", user, "custodian")).Find(&spaces).Error; err != nil {
		return nil, err
	}
	for _, sp := range spaces {
		var n int64
		if err := tx.Model(&SpacePermission{}).Where("space_id = ? AND user_id <> ? AND role = ?", sp.ID, user, "custodian").Where("user_id IN (?)", active).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			plan = append(plan, accountHandoff{"custodian", sp.ID, sp.TenantID, sp.Name})
		}
	}
	sort.Slice(plan, func(i, j int) bool { return plan[i].Kind+plan[i].ID < plan[j].Kind+plan[j].ID })
	return plan, nil
}

func (s *Service) applyAccountHandoffs(tx *gorm.DB, p Principal, plan []accountHandoff) error {
	for _, item := range plan {
		var membership Membership
		err := tx.First(&membership, "tenant_id = ? AND user_id = ?", item.TenantID, p.UserID).Error
		if err == gorm.ErrRecordNotFound {
			membership = Membership{ID: ID(), TenantID: item.TenantID, UserID: p.UserID, DataOwner: ID(), Role: "member"}
		} else if err != nil {
			return err
		}
		membership.Status = "active"
		if item.Kind == "tenant_admin" {
			membership.Role = "admin"
			membership.SuspendedRole = ""
		}
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		switch item.Kind {
		case "team_admin":
			v := TeamMember{TeamID: item.ID, UserID: p.UserID, Role: "admin"}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "team_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"role", "suspended_role"})}).Create(&v).Error; err != nil {
				return err
			}
			var t Team
			if err := tx.First(&t, "id = ?", item.ID).Error; err != nil {
				return err
			}
			t.Revision++
			t.UpdatedAt = time.Now().UTC()
			if err := tx.Save(&t).Error; err != nil {
				return err
			}
		case "custodian":
			var sp Space
			if err := tx.First(&sp, "id = ?", item.ID).Error; err != nil {
				return err
			}
			if sp.TeamID != "" {
				v := TeamMember{TeamID: sp.TeamID, UserID: p.UserID, Role: "member"}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&v).Error; err != nil {
					return err
				}
			}
			v := SpacePermission{SpaceID: item.ID, UserID: p.UserID, Role: "custodian"}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "space_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"role", "suspended_role"})}).Create(&v).Error; err != nil {
				return err
			}
		}
		if err := s.event(tx, p, Space{TenantID: item.TenantID}, "account.responsibility_taken", item.ID, "", item.Kind, p.UserID); err != nil {
			return err
		}
	}
	return nil
}
