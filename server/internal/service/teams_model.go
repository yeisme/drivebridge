package service

import (
	"gorm.io/gorm"
	"time"
)

type TeamInvitation struct {
	ID       string    `json:"id" gorm:"primaryKey"`
	TeamID   string    `json:"team_id" gorm:"index"`
	TenantID string    `json:"tenant_id" gorm:"index"`
	UserID   string    `json:"user_id" gorm:"index"`
	Inviter  string    `json:"inviter"`
	Status   string    `json:"status"`
	Expires  time.Time `json:"expires_at"`
	Created  time.Time `json:"created_at"`
}
type TeamStorage struct {
	TeamID       string    `json:"team_id" gorm:"primaryKey"`
	ConnectionID string    `json:"connection_id" gorm:"primaryKey"`
	TenantID     string    `json:"tenant_id" gorm:"index"`
	AssignedBy   string    `json:"assigned_by"`
	Created      time.Time `json:"created_at"`
}

func teamWritable(db *gorm.DB, space string) error {
	var sp Space
	if db.First(&sp, "id = ?", space).Error != nil {
		return code("not_found")
	}
	if sp.TeamID != "" {
		var t Team
		if db.First(&t, "id = ?", sp.TeamID).Error != nil {
			return code("not_found")
		}
		if t.Archived {
			return code("team_archived")
		}
	}
	return nil
}
func (s *Service) teamAdmin(p Principal, t Team) bool {
	return s.tenantAdmin(p.UserID, t.TenantID) || s.teamRole(p.UserID, t.ID) == "admin"
}
func (s *Service) visibleTeam(p Principal, id string) (Team, error) {
	var t Team
	if !s.governanceActive() || p.Agent != nil || p.ServiceIdentity != nil {
		return t, code("forbidden")
	}
	if s.DB.First(&t, "id = ? AND tenant_id = ?", id, p.TenantID).Error != nil {
		return t, code("not_found")
	}
	if !s.tenantAdmin(p.UserID, t.TenantID) && s.teamRole(p.UserID, t.ID) == "" {
		return t, code("not_found")
	}
	return t, nil
}

// Removing a member must not orphan either management or data responsibility.
func protectTeamMember(tx *gorm.DB, t Team, user, role string) error {
	var old TeamMember
	tx.First(&old, "team_id = ? AND user_id = ?", t.ID, user)
	if old.Role == "admin" && role != "admin" {
		var n int64
		if err := tx.Model(&TeamMember{}).Where("team_id = ? AND user_id <> ? AND role = ?", t.ID, user, "admin").Where("user_id IN (?)", tx.Model(&User{}).Select("id").Where("disabled = ?", false)).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return code("last_team_admin")
		}
	}
	if role == "remove" {
		var spaces []Space
		if err := tx.Where("team_id = ?", t.ID).Find(&spaces).Error; err != nil {
			return err
		}
		for _, sp := range spaces {
			if err := protectCustodian(tx, sp.ID, user, "remove"); err != nil {
				return err
			}
		}
	}
	return nil
}
func protectCustodian(tx *gorm.DB, space, user, role string) error {
	var old SpacePermission
	tx.First(&old, "space_id = ? AND user_id = ?", space, user)
	if old.Role == "custodian" && role != "custodian" {
		var n int64
		if err := tx.Model(&SpacePermission{}).Where("space_id = ? AND user_id <> ? AND role = ?", space, user, "custodian").Where("user_id IN (?)", tx.Model(&User{}).Select("id").Where("disabled = ?", false)).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return code("last_space_custodian")
		}
	}
	return nil
}
func (s *Service) protectTenantRemoval(p Principal, user string) error {
	var teams []Team
	if err := s.DB.Where("tenant_id = ?", p.TenantID).Find(&teams).Error; err != nil {
		return err
	}
	for _, t := range teams {
		if err := protectTeamMember(s.DB, t, user, "remove"); err != nil {
			return err
		}
	}
	return nil
}

// Resolve an exact space assignment; never grant all users access to a connection.
func (s *Service) storageForSpace(owner, space, connection string) (Storage, error) {
	var sp Space
	if s.DB.First(&sp, "id = ? AND owner = ? AND connection_id = ?", space, owner, connection).Error != nil {
		return nil, code("storage_disconnected")
	}
	if sp.TeamID == "" {
		return s.storageFor(owner, connection)
	}
	var a TeamStorage
	if s.DB.First(&a, "team_id = ? AND tenant_id = ? AND connection_id = ?", sp.TeamID, sp.TenantID, connection).Error != nil {
		return nil, code("storage_disconnected")
	}
	var c Connection
	if s.DB.First(&c, "id = ? AND disabled = ? AND kind = ?", connection, false, "s3").Error != nil {
		return nil, code("storage_disconnected")
	}
	var config S3Config
	if err := s.unseal(c.Owner, c.Secret, &config); err != nil {
		return nil, err
	}
	return s.storage(config, s.Config.Development)
}

// Cancellation releases staging and quota; it does not modify archived files.
func (s *Service) cancellableOperation(p Principal, id string) (Operation, error) {
	op, err := s.operation(p, id, "transfer")
	if err == nil || safeCode(err) != "team_archived" || p.Agent != nil {
		return op, err
	}
	op, err = s.operation(p, id, "read")
	if err != nil {
		return op, err
	}
	if op.ActorID != p.UserID {
		return Operation{}, code("forbidden")
	}
	return op, nil
}
