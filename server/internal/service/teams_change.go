package service

import (
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

func isTeamAction(a string) bool {
	switch a {
	case "teams", "team-member", "spaces", "space-quota", "permission", "team-update", "team-archive", "team-restore", "team-leave", "team-invite", "team-invite-cancel", "team-invite-accept", "team-invite-decline", "team-invite-resend", "team-storage-assign", "team-storage-unassign":
		return true
	}
	return false
}
func (s *Service) teamChange(w http.ResponseWriter, r *http.Request, p Principal, action string, q governanceInput) error {
	if action == "teams" {
		if !s.tenantAdmin(p.UserID, p.TenantID) || !validName(q.Name) {
			return code("forbidden")
		}
		if _, e := s.membership(q.UserID, p.TenantID); e != nil {
			return e
		}
		t := Team{ID: ID(), TenantID: p.TenantID, Name: strings.TrimSpace(q.Name), Description: q.Description, Revision: 1, UpdatedAt: time.Now().UTC()}
		if len(t.Description) > 2000 {
			return code("invalid_team")
		}
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			rules := &Service{DB: tx, Config: s.Config, Instance: s.Instance}
			if e := tx.Create(&t).Error; e != nil {
				return e
			}
			if e := tx.Create(&TeamMember{TeamID: t.ID, UserID: q.UserID, Role: "admin"}).Error; e != nil {
				return e
			}
			if rules.platformConnection() != "" {
				if e := tx.Create(&TeamStorage{TeamID: t.ID, TenantID: t.TenantID, ConnectionID: rules.platformConnection(), AssignedBy: p.UserID, Created: time.Now().UTC()}).Error; e != nil {
					return e
				}
			}
			return s.event(tx, p, Space{TenantID: t.TenantID}, "team.created", t.ID, "", "success", q.UserID)
		})
		if err != nil {
			return err
		}
		respond(w, t)
		return nil
	}
	// Existing space permission calls do not carry team_id.
	if q.TeamID == "" && q.SpaceID != "" {
		var sp Space
		if s.DB.First(&sp, "id = ? AND tenant_id = ?", q.SpaceID, p.TenantID).Error != nil {
			return code("not_found")
		}
		q.TeamID = sp.TeamID
	}
	var invitation TeamInvitation
	if action == "team-invite-accept" || action == "team-invite-cancel" || action == "team-invite-decline" || action == "team-invite-resend" {
		if s.DB.First(&invitation, "id = ? AND tenant_id = ?", q.ID, p.TenantID).Error != nil {
			return code("not_found")
		}
		if (action == "team-invite-accept" || action == "team-invite-decline") && invitation.UserID != p.UserID {
			return code("not_found")
		}
		if q.TeamID != "" && q.TeamID != invitation.TeamID {
			return code("not_found")
		}
		q.TeamID = invitation.TeamID
	}
	var team Team
	if s.DB.First(&team, "id = ? AND tenant_id = ?", q.TeamID, p.TenantID).Error != nil {
		return code("not_found")
	}
	manager := s.teamAdmin(p, team)
	tenantAdmin := s.tenantAdmin(p.UserID, p.TenantID)
	if action != "team-invite-accept" && action != "team-invite-decline" && action != "permission" && !manager && !(action == "team-leave" && s.teamRole(p.UserID, team.ID) != "") {
		return code("forbidden")
	}
	if strings.HasPrefix(action, "team-") && action != "team-member" && q.ExpectedRevision == nil {
		return code("revision_required")
	}
	if q.ExpectedRevision != nil && *q.ExpectedRevision != team.Revision {
		return code("version_conflict")
	}
	if team.Archived && action != "team-restore" {
		return code("team_archived")
	}
	var result any = map[string]bool{"ok": true}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		rules := &Service{DB: tx, Config: s.Config, Instance: s.Instance}
		var current Team
		if tx.First(&current, "id = ?", team.ID).Error != nil {
			return code("not_found")
		}
		if current.Revision != team.Revision {
			return code("version_conflict")
		}
		event := action
		target := q.UserID
		sp := Space{TenantID: team.TenantID}
		switch action {
		case "team-update":
			if !validName(q.Name) || len(q.Description) > 2000 {
				return code("invalid_team")
			}
			team.Name = strings.TrimSpace(q.Name)
			team.Description = q.Description
		case "team-archive", "team-restore":
			if !tenantAdmin {
				return code("forbidden")
			}
			if action == "team-restore" && !team.Archived {
				return code("version_conflict")
			}
			team.Archived = action == "team-archive"
			if team.Archived {
				var spaces []Space
				if e := tx.Where("team_id = ?", team.ID).Find(&spaces).Error; e != nil {
					return e
				}
				ids := []string{}
				for _, x := range spaces {
					ids = append(ids, x.ID)
				}
				if len(ids) > 0 {
					if e := tx.Model(&Share{}).Where("space_id IN ?", ids).Updates(map[string]any{"revoked": true, "revision": gorm.Expr("revision + 1")}).Error; e != nil {
						return e
					}
					if e := tx.Model(&Agent{}).Where("space_id IN ?", ids).Updates(map[string]any{"revoked": true, "archive_suspended": true}).Error; e != nil {
						return e
					}
					files := tx.Model(&File{}).Select("id").Where("space_id IN ?", ids)
					if e := tx.Where("file_id IN (?)", files).Delete(&AccessGrant{}).Error; e != nil {
						return e
					}
					if e := tx.Model(&HandoffGrant{}).Where("file_id IN (?)", files).Update("cancelled", true).Error; e != nil {
						return e
					}
					if e := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("space_id IN ? AND status IN ?", ids, []string{"pending", "approved", "executing"})); e != nil {
						return e
					}
				}
				if e := tx.Model(&TeamInvitation{}).Where("team_id = ? AND status = ?", team.ID, "pending").Update("status", "cancelled").Error; e != nil {
					return e
				}
			}
		case "team-member", "team-leave":
			if action == "team-leave" {
				q.UserID = p.UserID
				q.Role = "remove"
				target = q.UserID
			} else if q.UserID == p.UserID {
				return code("forbidden")
			}
			if q.Role != "admin" && q.Role != "member" && q.Role != "remove" {
				return code("invalid_role")
			}
			if _, e := rules.membership(q.UserID, team.TenantID); e != nil {
				return e
			}
			if action != "team-leave" && (q.Role == "admin" || rules.teamRole(q.UserID, team.ID) == "admin") && !tenantAdmin {
				return code("forbidden")
			}
			if e := protectTeamMember(tx, team, q.UserID, q.Role); e != nil {
				return e
			}
			if q.Role == "remove" {
				if e := tx.Where("team_id = ? AND user_id = ?", team.ID, q.UserID).Delete(&TeamMember{}).Error; e != nil {
					return e
				}
				var spaces []Space
				if e := tx.Where("team_id = ?", team.ID).Find(&spaces).Error; e != nil {
					return e
				}
				for _, v := range spaces {
					if e := tx.Where("space_id = ? AND user_id = ?", v.ID, q.UserID).Delete(&SpacePermission{}).Error; e != nil {
						return e
					}
					if e := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("space_id = ? AND (requester = ? OR approver = ?) AND status IN ?", v.ID, q.UserID, q.UserID, []string{"pending", "approved", "executing"})); e != nil {
						return e
					}
				}
			} else {
				if e := tx.Save(&TeamMember{TeamID: team.ID, UserID: q.UserID, Role: q.Role}).Error; e != nil {
					return e
				}
			}
			event = "team.member"
		case "spaces":
			if !tenantAdmin || !validName(q.Name) {
				return code("forbidden")
			}
			if _, e := rules.membership(q.UserID, team.TenantID); e != nil {
				return e
			}
			if rules.teamRole(q.UserID, team.ID) == "" {
				return code("forbidden")
			}
			connection := defaultString(q.ConnectionID, rules.platformConnection())
			var assignment TeamStorage
			if tx.First(&assignment, "team_id = ? AND connection_id = ?", team.ID, connection).Error != nil {
				return code("storage_not_assigned")
			}
			var c Connection
			if tx.First(&c, "id = ? AND kind = ? AND disabled = ?", connection, "s3", false).Error != nil {
				return code("storage_disconnected")
			}
			sp = Space{ID: ID(), TenantID: team.TenantID, TeamID: team.ID, Owner: ID(), Name: strings.TrimSpace(q.Name), ConnectionID: connection, MaxBytes: s.Config.MaxBytes}
			if q.MaxBytes != nil {
				if *q.MaxBytes <= 0 {
					return code("invalid_quota")
				}
				sp.MaxBytes = *q.MaxBytes
			}
			if e := tx.Create(&sp).Error; e != nil {
				return e
			}
			if e := tx.Create(&SpacePermission{SpaceID: sp.ID, UserID: q.UserID, Role: "custodian"}).Error; e != nil {
				return e
			}
			result = sp
			event = "space.created"
		case "space-quota":
			if !tenantAdmin {
				return code("forbidden")
			}
			if q.MaxBytes == nil || *q.MaxBytes <= 0 {
				return code("invalid_quota")
			}
			if e := tx.First(&sp, "id = ? AND team_id = ?", q.SpaceID, team.ID).Error; e != nil {
				return code("not_found")
			}
			resultDB := tx.Model(&Space{}).Where("id = ? AND used <= ? - reserved", sp.ID, *q.MaxBytes).Update("max_bytes", *q.MaxBytes)
			if resultDB.Error != nil {
				return resultDB.Error
			}
			if resultDB.RowsAffected != 1 {
				return code("quota_below_occupied")
			}
			sp.MaxBytes = *q.MaxBytes
			result = sp
			event = "space.quota_updated"
			target = sp.ID
		case "permission":
			if tx.First(&sp, "id = ? AND team_id = ?", q.SpaceID, team.ID).Error != nil {
				return code("not_found")
			}
			if q.UserID == p.UserID || !rules.canApprove(p, sp, q.UserID) {
				return code("forbidden")
			}
			if _, e := rules.membership(q.UserID, team.TenantID); e != nil {
				return e
			}
			if rules.teamRole(q.UserID, team.ID) == "" {
				return code("forbidden")
			}
			if q.Role != "viewer" && q.Role != "editor" && q.Role != "custodian" && q.Role != "remove" {
				return code("invalid_role")
			}
			if e := protectCustodian(tx, sp.ID, q.UserID, q.Role); e != nil {
				return e
			}
			if q.Role == "remove" {
				if e := tx.Where("space_id = ? AND user_id = ?", sp.ID, q.UserID).Delete(&SpacePermission{}).Error; e != nil {
					return e
				}
			} else {
				if e := tx.Save(&SpacePermission{SpaceID: sp.ID, UserID: q.UserID, Role: q.Role}).Error; e != nil {
					return e
				}
			}
			if e := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("space_id = ? AND (requester = ? OR approver = ?) AND status IN ?", sp.ID, q.UserID, q.UserID, []string{"pending", "approved", "executing"})); e != nil {
				return e
			}
			event = "space.permission"
		case "team-storage-assign":
			if !tenantAdmin {
				return code("forbidden")
			}
			var c Connection
			if tx.First(&c, "id = ?", q.ConnectionID).Error != nil {
				return code("not_found")
			}
			m, e := rules.membership(p.UserID, team.TenantID)
			if e != nil {
				return e
			}
			if c.ID != "platform" && c.Owner != m.DataOwner {
				return code("forbidden")
			}
			if c.Disabled || c.Kind != "s3" {
				return code("storage_disconnected")
			}
			a := TeamStorage{TeamID: team.ID, ConnectionID: c.ID, TenantID: team.TenantID, AssignedBy: p.UserID, Created: time.Now().UTC()}
			if e := tx.Where("team_id = ? AND connection_id = ?", a.TeamID, a.ConnectionID).FirstOrCreate(&a).Error; e != nil {
				return e
			}
		case "team-storage-unassign":
			if !tenantAdmin {
				return code("forbidden")
			}
			var n int64
			if e := tx.Model(&Space{}).Where("team_id = ? AND connection_id = ?", team.ID, q.ConnectionID).Count(&n).Error; e != nil {
				return e
			}
			if n > 0 {
				return code("storage_in_use")
			}
			if e := tx.Where("team_id = ? AND connection_id = ?", team.ID, q.ConnectionID).Delete(&TeamStorage{}).Error; e != nil {
				return e
			}
		case "team-invite":
			if q.UserID == "" || q.UserID == p.UserID {
				return code("forbidden")
			}
			var u User
			if tx.First(&u, "id = ? AND disabled = ?", q.UserID, false).Error != nil {
				return code("not_found")
			}
			if !tenantAdmin {
				if _, e := rules.membership(q.UserID, team.TenantID); e != nil {
					return e
				}
			}
			if rules.teamRole(q.UserID, team.ID) != "" {
				return code("membership_exists")
			}
			var old TeamInvitation
			if tx.First(&old, "team_id = ? AND user_id = ? AND status = ? AND expires > ?", team.ID, q.UserID, "pending", time.Now()).Error == nil {
				return code("invitation_exists")
			}
			invitation = TeamInvitation{ID: ID(), TeamID: team.ID, TenantID: team.TenantID, UserID: q.UserID, Inviter: p.UserID, Status: "pending", Created: time.Now().UTC(), Expires: time.Now().UTC().Add(7 * 24 * time.Hour)}
			if e := tx.Create(&invitation).Error; e != nil {
				return e
			}
			result = invitation
		case "team-invite-cancel":
			if invitation.Status != "pending" {
				return code("invitation_unavailable")
			}
			if e := tx.Model(&invitation).Update("status", "cancelled").Error; e != nil {
				return e
			}
			target = invitation.UserID
		case "team-invite-accept":
			if invitation.Status != "pending" || !time.Now().Before(invitation.Expires) {
				return code("invitation_unavailable")
			}
			if _, e := rules.membership(p.UserID, team.TenantID); e != nil {
				return e
			}
			var existing TeamMember
			if tx.First(&existing, "team_id = ? AND user_id = ?", team.ID, p.UserID).Error != nil {
				if e := tx.Create(&TeamMember{TeamID: team.ID, UserID: p.UserID, Role: "member"}).Error; e != nil {
					return e
				}
			}
			if e := tx.Model(&invitation).Update("status", "accepted").Error; e != nil {
				return e
			}
			target = p.UserID
		case "team-invite-decline":
			if invitation.Status != "pending" {
				return code("invitation_not_pending")
			}
			if !time.Now().Before(invitation.Expires) {
				return code("invitation_unavailable")
			}
			if e := tx.Model(&invitation).Update("status", "declined").Error; e != nil {
				return e
			}
			target = invitation.Inviter
			result = invitation
		case "team-invite-resend":
			if invitation.Status != "pending" {
				return code("invitation_not_pending")
			}
			invitation.Expires = time.Now().UTC().Add(7 * 24 * time.Hour)
			if e := tx.Model(&invitation).Update("expires", invitation.Expires).Error; e != nil {
				return e
			}
			target = invitation.UserID
			result = invitation
		default:
			return code("not_found")
		}
		team.Revision++
		team.UpdatedAt = time.Now().UTC()
		res := tx.Model(&Team{}).Where("id = ? AND revision = ?", team.ID, current.Revision).Updates(map[string]any{"name": team.Name, "description": team.Description, "archived": team.Archived, "revision": team.Revision, "updated_at": team.UpdatedAt})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return code("version_conflict")
		}
		return s.event(tx, p, sp, event, team.ID, "", "success", target)
	})
	if err != nil {
		return err
	}
	markAuditTarget(r, team.ID)
	if action == "spaces" {
		respond(w, result)
		return nil
	}
	respond(w, map[string]any{"team": team, "result": result})
	return nil
}
