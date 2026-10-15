package service

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type accountRow struct {
	User
	Role           string `json:"role"`
	Teams          []Team `json:"teams"`
	KeyCount       *int64 `json:"key_count"`
	ActiveKeyCount *int64 `json:"active_key_count"`
	CanManage      bool   `json:"can_manage"`
}

func (s *Service) visibleAccounts(p Principal) *gorm.DB {
	q := s.DB.Model(&User{})
	if s.platformAdmin(p.UserID) {
		return q
	}
	if s.tenantAdmin(p.UserID, p.TenantID) {
		return q.Where("id IN (?)", s.DB.Model(&Membership{}).Select("user_id").Where("tenant_id = ?", p.TenantID))
	}
	teams := s.DB.Model(&Team{}).Select("id").Where("tenant_id = ?", p.TenantID).Where("id IN (?)", s.DB.Model(&TeamMember{}).Select("team_id").Where("user_id = ?", p.UserID))
	return q.Where("id = ? OR id IN (?)", p.UserID, s.DB.Model(&TeamMember{}).Select("user_id").Where("team_id IN (?)", teams))
}

func (s *Service) accountProjection(p Principal, u User) (accountRow, error) {
	row := accountRow{User: u, Role: "member", Teams: []Team{}, CanManage: s.platformAdmin(p.UserID) && p.UserID != u.ID}
	var platformRole PlatformRole
	if s.DB.First(&platformRole, "user_id = ? AND suspended = ?", u.ID, false).Error == nil {
		row.Role = "platform_admin"
	} else if m, err := s.membership(u.ID, p.TenantID); err == nil && m.Role == "admin" {
		row.Role = "tenant_admin"
	}
	q := s.DB.Where("tenant_id = ?", p.TenantID).Where("id IN (?)", s.DB.Model(&TeamMember{}).Select("team_id").Where("user_id = ?", u.ID))
	if !s.tenantAdmin(p.UserID, p.TenantID) {
		q = q.Where("id IN (?)", s.DB.Model(&TeamMember{}).Select("team_id").Where("user_id = ?", p.UserID))
	}
	if err := q.Find(&row.Teams).Error; err != nil {
		return row, err
	}
	if s.platformAdmin(p.UserID) || p.UserID == u.ID {
		var count, active int64
		if err := s.DB.Model(&UserKey{}).Where("owner = ?", u.ID).Count(&count).Error; err != nil {
			return row, err
		}
		if err := s.DB.Model(&UserKey{}).Where("owner = ? AND revoked = ? AND archived = ? AND disabled = ?", u.ID, false, false, false).Where("expires_at = ? OR expires_at > ?", time.Time{}, time.Now()).Count(&active).Error; err != nil {
			return row, err
		}
		row.KeyCount, row.ActiveKeyCount = &count, &active
	}
	return row, nil
}

func (s *Service) accountDirectoryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/governance/users", s.wrap("governance.users.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !s.governanceActive() || p.Agent != nil || p.ServiceIdentity != nil {
			return code("forbidden")
		}
		q := s.visibleAccounts(p)
		switch r.URL.Query().Get("status") {
		case "archived":
			q = q.Where("archived = ?", true)
		case "frozen":
			q = q.Where("archived = ? AND disabled = ?", false, true)
		case "active":
			q = q.Where("archived = ? AND disabled = ?", false, false)
		default:
			q = q.Where("archived = ?", false)
		}
		if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
			keyQuery := s.DB.Model(&UserKey{}).Select("owner").Where("name LIKE ? OR id LIKE ?", "%"+search+"%", "%"+search+"%")
			if !s.platformAdmin(p.UserID) {
				keyQuery = keyQuery.Where("owner = ?", p.UserID)
			}
			q = q.Where("name LIKE ? OR id LIKE ? OR id IN (?)", "%"+search+"%", "%"+search+"%", keyQuery)
		}
		if team := r.URL.Query().Get("team"); team != "" {
			members := s.DB.Model(&TeamMember{}).Select("user_id").Where("team_id IN (?)", s.DB.Model(&Team{}).Select("id").Where("tenant_id = ?", p.TenantID))
			if team == "none" {
				q = q.Where("id NOT IN (?)", members)
			} else {
				q = q.Where("id IN (?)", members.Where("team_id = ?", team))
			}
		}
		platform := s.DB.Model(&PlatformRole{}).Select("user_id").Where("suspended = ?", false)
		admins := s.DB.Model(&Membership{}).Select("user_id").Where("tenant_id = ? AND role = ? AND status = ?", p.TenantID, "admin", "active")
		switch r.URL.Query().Get("role") {
		case "platform_admin":
			q = q.Where("id IN (?)", platform)
		case "tenant_admin":
			q = q.Where("id IN (?)", admins).Where("id NOT IN (?)", platform)
		case "member":
			q = q.Where("id NOT IN (?)", platform).Where("id NOT IN (?)", admins)
		}
		var total int64
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			return code("invalid_page")
		}
		users := []User{}
		if err := q.Order("name, id").Limit(50).Offset((page - 1) * 50).Find(&users).Error; err != nil {
			return err
		}
		rows := []accountRow{}
		for _, u := range users {
			row, err := s.accountProjection(p, u)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}
		respond(w, map[string]any{"items": rows, "total": total, "page": page, "page_size": 50})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/governance/users/{id}", s.wrap("governance.users.detail", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !s.governanceActive() || p.Agent != nil || p.ServiceIdentity != nil {
			return code("forbidden")
		}
		var u User
		if s.visibleAccounts(p).Where("id = ?", r.PathValue("id")).First(&u).Error != nil {
			return code("not_found")
		}
		row, err := s.accountProjection(p, u)
		if err != nil {
			return err
		}
		keys := []UserKey{}
		visible := s.platformAdmin(p.UserID) || p.UserID == u.ID
		if visible && s.DB.Where("owner = ?", u.ID).Order("created_at DESC").Find(&keys).Error != nil {
			return code("database_unavailable")
		}
		// UserKey serializes lifecycle metadata only; all secret fields are hidden.
		respond(w, map[string]any{"user": row, "keys": keys, "keys_visible": visible})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/governance/users/{id}/impact", s.wrap("governance.users.impact", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !s.governanceActive() || p.Agent != nil || p.ServiceIdentity != nil || !s.platformAdmin(p.UserID) {
			return code("forbidden")
		}
		s.governanceMu.Lock()
		defer s.governanceMu.Unlock()
		var u User
		if s.DB.First(&u, "id = ?", r.PathValue("id")).Error != nil {
			return code("not_found")
		}
		action := r.URL.Query().Get("action")
		blocker := s.accountActionCheck(s.DB, p, u, action)
		counts := map[string]int64{}
		for name, model := range map[string]any{"keys": &UserKey{}, "sessions": &Session{}, "agents": &Agent{}, "spaces": &Space{}, "files": &File{}} {
			var count int64
			query := s.DB.Model(model).Where("owner = ?", u.ID)
			if name == "spaces" || name == "files" {
				query = s.DB.Model(model).Where("owner = ? OR owner IN (?)", u.ID, s.DB.Model(&Membership{}).Select("data_owner").Where("user_id = ?", u.ID))
			}
			if err := query.Count(&count).Error; err != nil {
				return err
			}
			counts[name] = count
		}
		reason := ""
		if blocker != nil {
			reason = safeCode(blocker)
		}
		handoffs := []accountHandoff{}
		takeoverAllowed := reason == "last_team_admin" || reason == "last_space_custodian" || reason == "last_tenant_admin"
		if takeoverAllowed {
			var err error
			handoffs, err = accountHandoffs(s.DB, u.ID)
			if err != nil {
				return err
			}
		}
		respond(w, map[string]any{"user": u, "action": action, "allowed": blocker == nil, "reason": reason, "counts": counts, "takeover_allowed": takeoverAllowed, "handoffs": handoffs, "takeover_digest": accountHandoffDigest(handoffs)})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/governance/users/{id}/action", s.wrap("governance.users.action", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !s.governanceActive() || p.Agent != nil || p.ServiceIdentity != nil || !s.platformAdmin(p.UserID) {
			return code("forbidden")
		}
		var q struct {
			Action           string  `json:"action"`
			Proof            string  `json:"proof"`
			Revision         *uint64 `json:"expected_revision"`
			Confirm          bool    `json:"confirm"`
			ConfirmationName string  `json:"confirmation_name"`
			Takeover         bool    `json:"takeover"`
			TakeoverDigest   string  `json:"takeover_digest"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if !q.Confirm {
			return code("confirmation_required")
		}
		if q.Revision == nil {
			return code("revision_required")
		}
		s.governanceMu.Lock()
		defer s.governanceMu.Unlock()
		if q.ConfirmationName == "" {
			if err := s.verifyProof(p, q.Proof); err != nil {
				return err
			}
		}
		if err := s.changeAccount(p, r.PathValue("id"), q.Action, q.Revision, accountChangeOptions{Name: q.ConfirmationName, Takeover: q.Takeover, Digest: q.TakeoverDigest}); err != nil {
			return err
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"updated": true})
		return nil
	}))
}

func (s *Service) accountActionCheck(tx *gorm.DB, p Principal, u User, action string) error {
	if !s.platformAdmin(p.UserID) || u.ID == p.UserID {
		return code("forbidden")
	}
	switch action {
	case "freeze":
		if u.Archived || u.Disabled {
			return code("account_state_conflict")
		}
	case "unfreeze":
		if u.Archived || !u.Disabled {
			return code("account_state_conflict")
		}
	case "archive":
		if u.Archived {
			return code("account_state_conflict")
		}
	case "restore":
		if !u.Archived {
			return code("account_state_conflict")
		}
	default:
		return code("unsupported")
	}
	if action != "freeze" && action != "archive" {
		return nil
	}
	active := tx.Model(&User{}).Select("id").Where("disabled = ?", false)
	var role PlatformRole
	if tx.First(&role, "user_id = ? AND suspended = ?", u.ID, false).Error == nil {
		var n int64
		if err := tx.Model(&PlatformRole{}).Where("user_id <> ? AND suspended = ?", u.ID, false).Where("user_id IN (?)", active).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return code("last_platform_admin")
		}
	}
	var memberships []Membership
	if err := tx.Where("user_id = ? AND role = ? AND status = ?", u.ID, "admin", "active").Find(&memberships).Error; err != nil {
		return err
	}
	for _, m := range memberships {
		var n int64
		if err := tx.Model(&Membership{}).Where("tenant_id = ? AND user_id <> ? AND role = ? AND status = ?", m.TenantID, u.ID, "admin", "active").Where("user_id IN (?)", active).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return code("last_tenant_admin")
		}
	}
	var teams []Team
	if err := tx.Where("id IN (?)", tx.Model(&TeamMember{}).Select("team_id").Where("user_id = ?", u.ID)).Find(&teams).Error; err != nil {
		return err
	}
	for _, t := range teams {
		if err := protectTeamMember(tx, t, u.ID, "remove"); err != nil {
			return err
		}
	}
	var permissions []SpacePermission
	if err := tx.Where("user_id = ? AND role = ?", u.ID, "custodian").Find(&permissions).Error; err != nil {
		return err
	}
	for _, v := range permissions {
		if err := protectCustodian(tx, v.SpaceID, u.ID, "remove"); err != nil {
			return err
		}
	}
	return nil
}

type accountChangeOptions struct {
	Name     string
	Takeover bool
	Digest   string
}

func (s *Service) changeAccount(p Principal, id, action string, revision *uint64, options ...accountChangeOptions) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var u User
		if tx.First(&u, "id = ?", id).Error != nil {
			return code("not_found")
		}
		if revision != nil && u.Revision != *revision {
			return code("version_conflict")
		}
		// Use the same transaction for authority checks to avoid pool deadlock.
		rules := &Service{DB: tx, Instance: s.Instance, Config: s.Config}
		if len(options) > 0 {
			option := options[0]
			if option.Name != "" && strings.TrimSpace(option.Name) != strings.TrimSpace(u.Name) {
				return code("confirmation_mismatch")
			}
			if option.Takeover {
				if !rules.platformAdmin(p.UserID) || u.ID == p.UserID || (action != "freeze" && action != "archive") {
					return code("forbidden")
				}
				plan, err := accountHandoffs(tx, u.ID)
				if err != nil {
					return err
				}
				if option.Digest != accountHandoffDigest(plan) {
					return code("version_conflict")
				}
				if err := s.applyAccountHandoffs(tx, p, plan); err != nil {
					return err
				}
			}
		}
		if err := rules.accountActionCheck(tx, p, u, action); err != nil {
			return err
		}
		u.Revision++
		switch action {
		case "freeze":
			u.Disabled = true
		case "unfreeze":
			u.Disabled = false
		case "archive":
			u.Archived = true
			u.Disabled = true
		case "restore":
			u.Archived = false
			u.Disabled = true
		}
		if err := tx.Save(&u).Error; err != nil {
			return err
		}
		if action == "freeze" || action == "archive" {
			if err := tx.Where("owner = ?", id).Delete(&Session{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&UserKey{}).Where("owner = ?", id).Updates(map[string]any{"revoked": true, "sealed_key": nil}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Agent{}).Where("owner = ?", id).Update("revoked", true).Error; err != nil {
				return err
			}
			if err := tx.Where("owner = ? OR actor_id = ?", id, id).Delete(&AccessGrant{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&HandoffGrant{}).Where("owner = ? OR initiator = ?", id, id).Update("cancelled", true).Error; err != nil {
				return err
			}
			if err := tx.Model(&ShareSession{}).Where("user_id = ?", id).Update("revoked", true).Error; err != nil {
				return err
			}
			if err := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("(requester = ? OR approver = ?) AND status IN ?", id, id, []string{"pending", "approved", "executing"})); err != nil {
				return err
			}
		}
		if action == "archive" {
			if err := tx.Model(&Share{}).Where("owner = ? OR actor_id = ? OR owner IN (?)", id, id, tx.Model(&Membership{}).Select("data_owner").Where("user_id = ?", id)).Updates(map[string]any{"revoked": true, "revision": gorm.Expr("revision + 1")}).Error; err != nil {
				return err
			}
		}
		return s.event(tx, p, Space{TenantID: p.TenantID}, "account."+action, id, "", "success", id)
	})
}
