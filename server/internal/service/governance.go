package service

import (
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

func (s *Service) event(tx *gorm.DB, p Principal, sp Space, action, resource, request, outcome string, extra ...string) error {
	now := time.Now().UTC()
	e := GovernanceEvent{ID: ID(), TenantID: defaultString(sp.TenantID, p.TenantID), Actor: p.UserID, SpaceID: sp.ID, Action: action, ResourceID: resource, RequestID: request, Outcome: outcome, At: now}
	if action == "data.view" {
		e.Dedupe = p.UserID + ":" + sp.ID + ":" + now.Truncate(30*time.Minute).Format(time.RFC3339)
		var n int64
		if err := tx.Model(&GovernanceEvent{}).Where("dedupe = ?", e.Dedupe).Count(&n).Error; err != nil {
			return code("audit_unavailable")
		}
		if n > 0 {
			return nil
		}
	}
	if tx.Create(&e).Error != nil {
		return code("audit_unavailable")
	}
	recipients := map[string]bool{}
	for _, u := range extra {
		if u != "" {
			recipients[u] = true
		}
	}
	if sp.ID != "" {
		if sp.TeamID == "" {
			var m Membership
			if tx.First(&m, "tenant_id = ? AND data_owner = ?", sp.TenantID, sp.Owner).Error == nil {
				recipients[m.UserID] = true
			}
		} else {
			var ps []SpacePermission
			if tx.Where("space_id = ? AND role = ?", sp.ID, "custodian").Find(&ps).Error != nil {
				return code("audit_unavailable")
			}
			for _, v := range ps {
				recipients[v.UserID] = true
			}
		}
	}
	if strings.HasPrefix(action, "emergency") {
		var ms []Membership
		if tx.Where("tenant_id = ? AND role = ? AND status = ?", e.TenantID, "admin", "active").Find(&ms).Error != nil {
			return code("audit_unavailable")
		}
		for _, m := range ms {
			recipients[m.UserID] = true
		}
	}
	for u := range recipients {
		n := Notification{ID: ID(), TenantID: e.TenantID, UserID: u, EventID: e.ID, Action: action, ResourceID: resource, Actor: p.UserID, Created: now}
		if tx.Create(&n).Error != nil {
			return code("audit_unavailable")
		}
	}
	return nil
}
func (s *Service) recordAccess(p Principal, sp Space, kind, resource string) error {
	if len(resource) != 48 {
		resource = ""
	}
	return s.DB.Transaction(func(tx *gorm.DB) error { return s.event(tx, p, sp, "data."+kind, resource, "", "intent") })
}
func validName(v string) bool {
	return strings.TrimSpace(v) != "" && len(v) <= 120 && !strings.ContainsAny(v, "\r\n\x00")
}

type governanceInput struct {
	MaxBytes         *int64            `json:"max_bytes"`
	Description      string            `json:"description"`
	ConnectionID     string            `json:"connection_id"`
	ExpectedRevision *uint64           `json:"expected_revision"`
	ID               string            `json:"id"`
	TenantID         string            `json:"tenant_id"`
	TeamID           string            `json:"team_id"`
	SpaceID          string            `json:"space_id"`
	UserID           string            `json:"user_id"`
	Name             string            `json:"name"`
	Role             string            `json:"role"`
	Status           string            `json:"status"`
	Reason           string            `json:"reason"`
	Kind             string            `json:"kind"`
	Action           string            `json:"action"`
	Prefix           string            `json:"prefix"`
	Hours            int               `json:"hours"`
	Items            []RequestedAction `json:"items"`
	Disabled         bool              `json:"disabled"`
	Proof            string            `json:"proof"`
	ClaimID          string            `json:"claim_id"`
}

func (s *Service) governanceRoutes(mux *http.ServeMux) {
	s.accountDirectoryRoutes(mux)
	mux.HandleFunc("GET /api/v1/files-export", s.wrap("files.export", s.exportFiles))
	mux.HandleFunc("GET /api/v1/governance", s.wrap("governance.overview", s.governanceOverview))
	mux.HandleFunc("POST /api/v1/governance/{action}", s.wrap("governance.change", s.governanceChange))
	s.teamRoutes(mux)
	s.recoveryRoutes(mux)
	mux.HandleFunc("POST /auth/enroll", s.enroll)
}
func (s *Service) governanceOverview(w http.ResponseWriter, r *http.Request, p Principal) error {
	if !s.governanceActive() {
		respond(w, map[string]any{"enabled": false})
		return nil
	}
	if p.Agent != nil {
		return code("forbidden")
	}
	if err := s.expireRequests(); err != nil {
		return err
	}
	tenants := []Tenant{}
	memberships := []Membership{}
	if s.DB.Where("user_id = ?", p.UserID).Find(&memberships).Error != nil {
		return code("database_unavailable")
	}
	ids := []string{}
	for _, m := range memberships {
		ids = append(ids, m.TenantID)
	}
	query := s.DB
	if !s.platformAdmin(p.UserID) {
		query = query.Where("id IN ?", ids)
	}
	if query.Find(&tenants).Error != nil {
		return code("database_unavailable")
	}
	spaces := []Space{}
	all := []Space{}
	s.DB.Where("tenant_id = ?", p.TenantID).Find(&all)
	roles := map[string]string{}
	for _, sp := range all {
		if role := s.scopeRole(p, sp); role != "" {
			spaces = append(spaces, sp)
			roles[sp.ID] = role
		}
	}
	teams := []Team{}
	allTeams := []Team{}
	s.DB.Where("tenant_id = ?", p.TenantID).Find(&allTeams)
	for _, t := range allTeams {
		if s.tenantAdmin(p.UserID, p.TenantID) || s.teamRole(p.UserID, t.ID) != "" {
			teams = append(teams, t)
		}
	}
	members := []Membership{}
	users := []User{}
	teamMembers := []TeamMember{}
	permissions := []SpacePermission{}
	memberIDs := []string{p.UserID}
	if s.tenantAdmin(p.UserID, p.TenantID) {
		s.DB.Where("tenant_id = ?", p.TenantID).Find(&members)
		for _, m := range members {
			memberIDs = append(memberIDs, m.UserID)
		}
	}
	for _, t := range teams {
		var ms []TeamMember
		s.DB.Where("team_id = ?", t.ID).Find(&ms)
		teamMembers = append(teamMembers, ms...)
		for _, m := range ms {
			memberIDs = append(memberIDs, m.UserID)
		}
	}
	userQuery := s.DB
	if !s.platformAdmin(p.UserID) {
		userQuery = userQuery.Where("id IN ?", memberIDs)
	}
	userQuery.Find(&users)
	for _, sp := range spaces {
		var ps []SpacePermission
		s.DB.Where("space_id = ?", sp.ID).Find(&ps)
		permissions = append(permissions, ps...)
	}
	requests := []AccessRequest{}
	allRequests := []AccessRequest{}
	s.DB.Where("tenant_id = ?", p.TenantID).Order("created DESC").Limit(500).Find(&allRequests)
	approvable := map[string]bool{}
	for _, a := range allRequests {
		var sp Space
		if s.DB.First(&sp, "id = ?", a.SpaceID).Error != nil {
			continue
		}
		can := s.canApprove(p, sp, a.Requester)
		if can || a.Requester == p.UserID {
			requests = append(requests, a)
			approvable[a.ID] = can
		}
	}
	notifications := []Notification{}
	s.DB.Where("tenant_id = ? AND user_id = ?", p.TenantID, p.UserID).Order("created DESC").Limit(500).Find(&notifications)
	events := []GovernanceEvent{}
	eventQuery := s.DB.Where("tenant_id = ?", p.TenantID)
	if !s.tenantAdmin(p.UserID, p.TenantID) {
		eventIDs := []string{}
		for _, n := range notifications {
			eventIDs = append(eventIDs, n.EventID)
		}
		eventQuery = eventQuery.Where("actor = ? OR id IN ?", p.UserID, eventIDs)
	}
	eventQuery.Order("at DESC").Limit(500).Find(&events)
	recoveries := []RecoveryClaim{}
	platformRoles := []PlatformRole{}
	if s.platformAdmin(p.UserID) {
		s.DB.Order("created DESC").Limit(100).Find(&recoveries)
		s.DB.Find(&platformRoles)
	}
	respond(w, map[string]any{"enabled": true, "actor_id": p.UserID, "tenant_id": p.TenantID, "platform_admin": s.platformAdmin(p.UserID), "tenant_admin": s.tenantAdmin(p.UserID, p.TenantID), "tenants": tenants, "memberships": memberships, "members": members, "users": users, "teams": teams, "team_members": teamMembers, "spaces": spaces, "space_roles": roles, "permissions": permissions, "requests": requests, "approvable": approvable, "notifications": notifications, "events": events, "recoveries": recoveries, "platform_roles": platformRoles})
	return nil
}
func (s *Service) governanceChange(w http.ResponseWriter, r *http.Request, p Principal) error {
	if !s.governanceActive() || p.Agent != nil {
		return code("forbidden")
	}
	var q governanceInput
	if err := decode(r, &q); err != nil {
		return err
	}
	if q.TenantID != "" && q.TenantID != p.TenantID {
		return code("forbidden")
	}
	// One owner serializes governance decisions, including approval/revocation.
	s.governanceMu.Lock()
	defer s.governanceMu.Unlock()
	action := r.PathValue("action")
	if isTeamAction(action) {
		return s.teamChange(w, r, p, action, q)
	}
	if action == "remove-member" {
		if e := s.protectTenantRemoval(p, q.UserID); e != nil {
			return e
		}
	}
	if action == "requests" || action == "approve" || action == "reject" || action == "revoke" || action == "execute" || action == "emergency" {
		return s.requestChange(w, r, p, action, q)
	}
	var result any = map[string]bool{"ok": true}
	empty := Space{TenantID: p.TenantID}
	switch action {
	case "provision":
		out, e := s.provisionAccount(p, q.Name)
		if e != nil {
			return e
		}
		result = out
	case "platform-role":
		if !s.platformAdmin(p.UserID) || q.UserID == p.UserID || s.verifyProof(p, q.Proof) != nil {
			return code("forbidden")
		}
		var u User
		if s.DB.First(&u, "id = ? AND disabled = ?", q.UserID, false).Error != nil {
			return code("not_found")
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			var e error
			if q.Role == "admin" {
				e = tx.Save(&PlatformRole{UserID: q.UserID}).Error
			} else if q.Role == "remove" {
				e = tx.Delete(&PlatformRole{}, "user_id = ?", q.UserID).Error
			} else {
				return code("invalid_role")
			}
			if e != nil {
				return e
			}
			if e = s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("(requester = ? OR approver = ?) AND status IN ?", q.UserID, q.UserID, []string{"pending", "approved", "executing"})); e != nil {
				return e
			}
			return s.event(tx, p, empty, "account.platform_role", q.UserID, "", "success", q.UserID)
		}); err != nil {
			return err
		}
	case "member-role":
		if !s.platformAdmin(p.UserID) || q.UserID == p.UserID || (q.Role != "admin" && q.Role != "member") {
			return code("forbidden")
		}
		m, e := s.membership(q.UserID, p.TenantID)
		if e != nil {
			return e
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&m).Updates(map[string]any{"role": q.Role, "suspended_role": ""}).Error; e != nil {
				return e
			}
			if e := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("tenant_id = ? AND (requester = ? OR approver = ?) AND status IN ?", p.TenantID, q.UserID, q.UserID, []string{"pending", "approved", "executing"})); e != nil {
				return e
			}
			return s.event(tx, p, empty, "member.role", m.ID, "", "success", q.UserID)
		}); err != nil {
			return err
		}
	case "tenants":
		if !s.platformAdmin(p.UserID) || !validName(q.Name) || q.UserID == "" {
			return code("forbidden")
		}
		var u User
		if s.DB.First(&u, "id = ? AND disabled = ?", q.UserID, false).Error != nil {
			return code("not_found")
		}
		t := Tenant{ID: ID(), Name: q.Name}
		m := Membership{ID: ID(), TenantID: t.ID, UserID: q.UserID, DataOwner: ID(), Role: "admin", Status: "invited"}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&t).Error; err != nil {
				return err
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			return s.event(tx, p, Space{TenantID: t.ID}, "tenant.invited", m.ID, "", "success", q.UserID)
		}); err != nil {
			return err
		}
		result = t
	case "tenant-status":
		if !s.platformAdmin(p.UserID) {
			return code("forbidden")
		}
		if q.ID != p.TenantID {
			return code("forbidden")
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&Tenant{}).Where("id = ?", q.ID).Update("disabled", q.Disabled)
			if res.Error != nil || res.RowsAffected != 1 {
				return code("not_found")
			}
			if q.Disabled {
				if err := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("tenant_id = ? AND status IN ?", q.ID, []string{"pending", "approved", "executing"})); err != nil {
					return err
				}
			}
			return s.event(tx, p, empty, "tenant.status", q.ID, "", "success")
		}); err != nil {
			return err
		}
	case "invite":
		if !s.tenantAdmin(p.UserID, p.TenantID) || q.UserID == p.UserID || (q.Role != "member" && q.Role != "admin") {
			return code("forbidden")
		}
		if q.Role == "admin" && !s.platformAdmin(p.UserID) {
			return code("forbidden")
		}
		var u User
		if s.DB.First(&u, "id = ? AND disabled = ?", q.UserID, false).Error != nil {
			return code("not_found")
		}
		m := Membership{}
		s.DB.Where("tenant_id = ? AND user_id = ?", p.TenantID, q.UserID).First(&m)
		if m.Status == "active" {
			return code("membership_exists")
		}
		if m.ID == "" {
			m = Membership{ID: ID(), TenantID: p.TenantID, UserID: q.UserID, DataOwner: ID()}
		}
		m.Status, m.Role = "invited", q.Role
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Save(&m).Error; e != nil {
				return e
			}
			return s.event(tx, p, empty, "member.invited", m.ID, "", "success", m.UserID)
		}); err != nil {
			return err
		}
		result = m
	case "accept":
		var m Membership
		if s.DB.First(&m, "id = ? AND user_id = ? AND status = ?", q.ID, p.UserID, "invited").Error != nil {
			return code("forbidden")
		}
		var t Tenant
		if s.DB.First(&t, "id = ? AND disabled = ?", m.TenantID, false).Error != nil {
			return code("forbidden")
		}
		m.Status = "active"
		connection := s.platformConnection()
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Save(&m).Error; e != nil {
				return e
			}
			var n int64
			tx.Model(&Space{}).Where("owner = ?", m.DataOwner).Count(&n)
			if n == 0 {
				sp := Space{ID: ID(), TenantID: m.TenantID, Owner: m.DataOwner, Name: "个人空间", ConnectionID: connection, MaxBytes: s.Config.MaxBytes}
				if e := tx.Create(&sp).Error; e != nil {
					return e
				}
			}
			return s.event(tx, p, Space{TenantID: m.TenantID}, "member.accepted", m.ID, "", "success", p.UserID)
		}); err != nil {
			return err
		}
	case "remove-member":
		if !s.tenantAdmin(p.UserID, p.TenantID) || q.UserID == p.UserID {
			return code("forbidden")
		}
		m, e := s.membership(q.UserID, p.TenantID)
		if e != nil {
			return e
		}
		if m.Role == "admin" && !s.platformAdmin(p.UserID) {
			return code("forbidden")
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&m).Update("status", "removed").Error; e != nil {
				return e
			}
			var teamIDs []string
			if e := tx.Model(&Team{}).Where("tenant_id = ?", p.TenantID).Pluck("id", &teamIDs).Error; e != nil {
				return e
			}
			if e := tx.Where("team_id IN ? AND user_id = ?", teamIDs, q.UserID).Delete(&TeamMember{}).Error; e != nil {
				return e
			}
			var spaceIDs []string
			if e := tx.Model(&Space{}).Where("tenant_id = ?", p.TenantID).Pluck("id", &spaceIDs).Error; e != nil {
				return e
			}
			if e := tx.Where("space_id IN ? AND user_id = ?", spaceIDs, q.UserID).Delete(&SpacePermission{}).Error; e != nil {
				return e
			}
			if e := s.invalidateRequests(tx, p, tx.Model(&AccessRequest{}).Where("tenant_id = ? AND (requester = ? OR approver = ?) AND status IN ?", p.TenantID, q.UserID, q.UserID, []string{"pending", "approved", "executing"})); e != nil {
				return e
			}
			return s.event(tx, p, empty, "member.removed", m.ID, "", "success", q.UserID)
		}); err != nil {
			return err
		}
	case "read-notification":
		res := s.DB.Model(&Notification{}).Where("id = ? AND user_id = ? AND tenant_id = ?", q.ID, p.UserID, p.TenantID).Update("read", true)
		if res.Error != nil || res.RowsAffected != 1 {
			return code("not_found")
		}
	case "recovery-approve", "restore-privileges", "account-status":
		return s.recoveryAdmin(w, r, p, action, q)
	default:
		return code("not_found")
	}
	respond(w, result)
	return nil
}
func (s *Service) platformConnection() string {
	var c Connection
	if s.DB.First(&c, "id = ? AND disabled = ?", "platform", false).Error == nil {
		return c.ID
	}
	return ""
}

// Membership and role changes notify every affected grant holder atomically.
func (s *Service) invalidateRequests(tx *gorm.DB, p Principal, query *gorm.DB) error {
	var requests []AccessRequest
	if e := query.Find(&requests).Error; e != nil {
		return e
	}
	for _, a := range requests {
		result := tx.Model(&AccessRequest{}).Where("id = ? AND status IN ?", a.ID, []string{"pending", "approved", "executing"}).Update("status", "revoked")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		var sp Space
		if e := tx.First(&sp, "id = ?", a.SpaceID).Error; e != nil {
			return e
		}
		if e := s.event(tx, p, sp, "request.revoked", a.ID, a.ID, "revoked", a.Requester, a.Approver); e != nil {
			return e
		}
	}
	return nil
}
