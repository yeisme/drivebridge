package service

import (
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type teamAction struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func capability(ok bool, reason string) teamAction {
	if ok {
		return teamAction{Allowed: true}
	}
	return teamAction{Reason: reason}
}
func (s *Service) teamActions(p Principal, t Team) map[string]teamAction {
	manager := s.teamAdmin(p, t)
	tenant := s.tenantAdmin(p.UserID, t.TenantID)
	out := map[string]teamAction{}
	for _, a := range []string{"update", "manage_members", "invite"} {
		out[a] = capability(manager && !t.Archived, "team_management_required")
	}
	for _, a := range []string{"create_space", "set_quota", "manage_storage", "archive", "promote_admin"} {
		out[a] = capability(tenant && !t.Archived, "tenant_admin_required")
	}
	out["restore"] = capability(tenant && t.Archived, "tenant_admin_required")
	out["provision"] = capability(s.platformAdmin(p.UserID) && !t.Archived, "platform_admin_required")
	out["leave"] = capability(s.teamRole(p.UserID, t.ID) != "" && !t.Archived, "team_membership_required")
	if t.Archived {
		for k, v := range out {
			if k != "restore" && !v.Allowed {
				v.Reason = "team_archived"
				out[k] = v
			}
		}
	}
	return out
}
func teamPage(r *http.Request) (int, int, error) {
	page := 1
	limit := 50
	var err error
	if v := r.URL.Query().Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page < 1 || page > 100000 {
			return 0, 0, code("invalid_page")
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, code("invalid_page")
		}
	}
	return page, limit, nil
}
func pageRows[T any](w http.ResponseWriter, r *http.Request, q *gorm.DB) error {
	p, l, e := teamPage(r)
	if e != nil {
		return e
	}
	items := []T{}
	if e = q.Offset((p - 1) * l).Limit(l + 1).Find(&items).Error; e != nil {
		return e
	}
	next := 0
	if len(items) > l {
		items = items[:l]
		next = p + 1
	}
	respond(w, map[string]any{"items": items, "next_page": next})
	return nil
}
func (s *Service) teamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/governance/teams", s.wrap("teams.list", s.teamList))
	mux.HandleFunc("GET /api/v1/governance/team-invitations", s.wrap("teams.invitations", s.myTeamInvitations))
	mux.HandleFunc("GET /api/v1/governance/teams/{team}", s.wrap("teams.detail", s.teamDetail))
	for _, section := range []string{"members", "spaces", "activity", "impact", "invitations"} {
		mux.HandleFunc("GET /api/v1/governance/teams/{team}/"+section, s.wrap("teams."+section, s.teamSection))
	}
}
func (s *Service) teamList(w http.ResponseWriter, r *http.Request, p Principal) error {
	if !s.governanceActive() {
		respond(w, map[string]any{"enabled": false, "items": []any{}, "next_page": 0})
		return nil
	}
	if p.Agent != nil || p.ServiceIdentity != nil {
		return code("forbidden")
	}
	query := s.DB.Model(&Team{}).Where("tenant_id = ?", p.TenantID)
	filter := r.URL.Query().Get("filter")
	if filter == "archived" {
		query = query.Where("archived = ?", true)
	} else {
		query = query.Where("archived = ?", false)
	}
	if !s.tenantAdmin(p.UserID, p.TenantID) || filter == "joined" {
		query = query.Where("id IN (?)", s.DB.Model(&TeamMember{}).Select("team_id").Where("user_id = ?", p.UserID))
	}
	if filter == "managed" && !s.tenantAdmin(p.UserID, p.TenantID) {
		query = query.Where("id IN (?)", s.DB.Model(&TeamMember{}).Select("team_id").Where("user_id = ? AND role = ?", p.UserID, "admin"))
	}
	if text := strings.TrimSpace(r.URL.Query().Get("q")); text != "" {
		query = query.Where("name LIKE ?", "%"+text+"%")
	}
	page, limit, e := teamPage(r)
	if e != nil {
		return e
	}
	teams := []Team{}
	if e = query.Order("name ASC, id ASC").Offset((page - 1) * limit).Limit(limit + 1).Find(&teams).Error; e != nil {
		return e
	}
	next := 0
	if len(teams) > limit {
		teams = teams[:limit]
		next = page + 1
	}
	rows := []map[string]any{}
	for _, t := range teams {
		rows = append(rows, s.teamSummary(p, t))
	}
	respond(w, map[string]any{"enabled": true, "items": rows, "next_page": next, "can_create": s.tenantAdmin(p.UserID, p.TenantID)})
	return nil
}
func (s *Service) teamSummary(p Principal, t Team) map[string]any {
	var count int64
	s.DB.Model(&TeamMember{}).Where("team_id = ?", t.ID).Count(&count)
	spaces := s.visibleTeamSpaces(p, t)
	var used, quota int64
	for _, sp := range spaces {
		used += sp.Used
		quota += sp.MaxBytes
	}
	var pending int64
	q := s.DB.Model(&TeamInvitation{}).Where("team_id = ? AND status = ? AND expires > ?", t.ID, "pending", time.Now())
	if !s.teamAdmin(p, t) {
		q = q.Where("user_id = ?", p.UserID)
	}
	q.Count(&pending)
	role := s.teamRole(p.UserID, t.ID)
	if role == "" && s.tenantAdmin(p.UserID, t.TenantID) {
		role = "tenant_admin"
	}
	return map[string]any{"team": t, "role": role, "member_count": count, "space_count": len(spaces), "used_bytes": used, "quota_bytes": quota, "pending_invitations": pending, "actions": s.teamActions(p, t)}
}
func (s *Service) visibleTeamSpaces(p Principal, t Team) []Space {
	rows := []Space{}
	s.DB.Where("team_id = ? AND tenant_id = ?", t.ID, t.TenantID).Order("name ASC, id ASC").Find(&rows)
	out := []Space{}
	for _, sp := range rows {
		if s.scopeRole(p, sp) != "" {
			out = append(out, sp)
		}
	}
	return out
}
func (s *Service) teamDetail(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, e := s.visibleTeam(p, r.PathValue("team"))
	if e != nil {
		return e
	}
	out := s.teamSummary(p, t)
	spaces := s.visibleTeamSpaces(p, t)
	ids := []string{}
	roles := map[string]string{}
	approvals := map[string]bool{}
	for _, sp := range spaces {
		ids = append(ids, sp.ID)
		roles[sp.ID] = s.scopeRole(p, sp)
		approvals[sp.ID] = s.canApprove(p, sp, "") && !t.Archived
	}
	permissions := []SpacePermission{}
	files := []File{}
	agents := []Agent{}
	requests := []AccessRequest{}
	if len(ids) > 0 {
		s.DB.Where("space_id IN ?", ids).Find(&permissions)
		s.DB.Where("space_id IN ? AND trashed = ?", ids, false).Order("updated DESC").Limit(8).Find(&files)
		s.DB.Where("space_id IN ?", ids).Find(&agents)
		s.DB.Where("space_id IN ? AND status = ?", ids, "pending").Find(&requests)
	}
	visibleRequests := []AccessRequest{}
	for _, req := range requests {
		var sp Space
		s.DB.First(&sp, "id = ?", req.SpaceID)
		if req.Requester == p.UserID || s.canApprove(p, sp, req.Requester) {
			visibleRequests = append(visibleRequests, req)
		}
	}
	assignments := []TeamStorage{}
	s.DB.Where("team_id = ?", t.ID).Find(&assignments)
	connectionIDs := []string{}
	for _, a := range assignments {
		connectionIDs = append(connectionIDs, a.ConnectionID)
	}
	connections := []Connection{}
	if len(connectionIDs) > 0 {
		s.DB.Where("id IN ?", connectionIDs).Find(&connections)
	}
	candidates := []Connection{}
	if s.tenantAdmin(p.UserID, t.TenantID) {
		if m, e := s.membership(p.UserID, t.TenantID); e == nil {
			s.DB.Where("id = ? OR owner = ?", "platform", m.DataOwner).Find(&candidates)
		}
	}
	out["space_roles"] = roles
	out["can_grant"] = approvals
	out["permissions"] = permissions
	out["recent_files"] = files
	out["agents"] = agents
	out["pending_requests"] = visibleRequests
	out["assignments"] = assignments
	out["connections"] = connections
	out["storage_candidates"] = candidates
	respond(w, out)
	return nil
}
func (s *Service) teamSection(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, e := s.visibleTeam(p, r.PathValue("team"))
	if e != nil {
		return e
	}
	parts := strings.Split(r.URL.Path, "/")
	section := parts[len(parts)-1]
	switch section {
	case "members":
		query := s.DB.Model(&TeamMember{}).Where("team_id = ?", t.ID)
		page, limit, e := teamPage(r)
		if e != nil {
			return e
		}
		ms := []TeamMember{}
		if e = query.Order("user_id ASC").Offset((page - 1) * limit).Limit(limit + 1).Find(&ms).Error; e != nil {
			return e
		}
		next := 0
		if len(ms) > limit {
			ms = ms[:limit]
			next = page + 1
		}
		out := []map[string]any{}
		for _, m := range ms {
			var u User
			s.DB.First(&u, "id = ?", m.UserID)
			status := "active"
			if _, err := s.membership(m.UserID, t.TenantID); err != nil {
				status = "inactive"
			}
			out = append(out, map[string]any{"user_id": m.UserID, "name": u.Name, "role": m.Role, "status": status})
		}
		respond(w, map[string]any{"items": out, "next_page": next})
		return nil
	case "spaces":
		rows := s.visibleTeamSpaces(p, t)
		page, limit, e := teamPage(r)
		if e != nil {
			return e
		}
		start := (page - 1) * limit
		if start > len(rows) {
			start = len(rows)
		}
		end := start + limit
		next := 0
		if end < len(rows) {
			next = page + 1
		} else {
			end = len(rows)
		}
		respond(w, map[string]any{"items": rows[start:end], "next_page": next})
		return nil
	case "activity":
		ids := []string{}
		for _, sp := range s.visibleTeamSpaces(p, t) {
			ids = append(ids, sp.ID)
		}
		return pageRows[GovernanceEvent](w, r, s.DB.Model(&GovernanceEvent{}).Where("tenant_id = ? AND (resource_id = ? OR space_id IN ?)", t.TenantID, t.ID, ids).Order("at DESC, id DESC"))
	case "invitations":
		query := s.DB.Model(&TeamInvitation{}).Where("team_id = ?", t.ID)
		if !s.teamAdmin(p, t) {
			query = query.Where("user_id = ?", p.UserID)
		}
		page, limit, err := teamPage(r)
		if err != nil {
			return err
		}
		rows := []TeamInvitation{}
		if err = query.Order("created DESC, id DESC").Offset((page - 1) * limit).Limit(limit + 1).Find(&rows).Error; err != nil {
			return err
		}
		next := 0
		if len(rows) > limit {
			next = page + 1
			rows = rows[:limit]
		}
		out := []map[string]any{}
		for _, row := range rows {
			out = append(out, s.invitationView(p, row, t))
		}
		respond(w, map[string]any{"items": out, "next_page": next})
		return nil
	case "impact":
		var permissionSpace Space
		permitted := false
		if target := r.URL.Query().Get("space_id"); target != "" && s.DB.First(&permissionSpace, "id = ? AND team_id = ?", target, t.ID).Error == nil {
			permitted = s.canApprove(p, permissionSpace, r.URL.Query().Get("user_id"))
		}
		if !permitted && !s.teamAdmin(p, t) && r.URL.Query().Get("user_id") != p.UserID {
			return code("forbidden")
		}
		spaces := s.visibleTeamSpaces(p, t)
		ids := []string{}
		for _, sp := range spaces {
			ids = append(ids, sp.ID)
		}
		var members, shares, agents, operations int64
		s.DB.Model(&TeamMember{}).Where("team_id = ?", t.ID).Count(&members)
		s.DB.Model(&Share{}).Where("space_id IN ? AND revoked = ?", ids, false).Count(&shares)
		s.DB.Model(&Agent{}).Where("space_id IN ? AND revoked = ?", ids, false).Count(&agents)
		s.DB.Model(&Operation{}).Where("space_id IN ? AND status NOT IN ?", ids, []string{"completed", "cancelled", "failed"}).Count(&operations)
		blockers := []string{}
		if user := r.URL.Query().Get("user_id"); user != "" {
			var e error
			if permissionSpace.ID != "" {
				e = protectCustodian(s.DB, permissionSpace.ID, user, r.URL.Query().Get("role"))
			} else {
				e = protectTeamMember(s.DB, t, user, defaultString(r.URL.Query().Get("role"), "remove"))
			}
			if e != nil {
				blockers = append(blockers, safeCode(e))
			}
		}
		respond(w, map[string]any{"revision": t.Revision, "members": members, "spaces": spaces, "shares": shares, "agents": agents, "operations": operations, "blockers": blockers, "historical_links": "previously issued storage URLs remain valid until expiry"})
		return nil
	}
	return code("not_found")
}
func (s *Service) myTeamInvitations(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent != nil || p.ServiceIdentity != nil || !s.governanceActive() {
		return code("forbidden")
	}
	rows := []TeamInvitation{}
	if e := s.DB.Where("user_id = ? AND status = ?", p.UserID, "pending").Order("created DESC").Limit(100).Find(&rows).Error; e != nil {
		return e
	}
	out := []map[string]any{}
	for _, i := range rows {
		var t Team
		if s.DB.First(&t, "id = ?", i.TeamID).Error != nil || t.Archived {
			continue
		}
		view := s.invitationView(p, i, t)
		var membership Membership
		s.DB.First(&membership, "user_id = ? AND tenant_id = ?", p.UserID, t.TenantID)
		out = append(out, map[string]any{"invitation": view, "status": view["status"], "next_step": view["next_step"], "team": t, "membership": membership})
	}
	respond(w, map[string]any{"items": out})
	return nil
}

func invitationStatus(i TeamInvitation) string {
	if i.Status == "pending" && !time.Now().Before(i.Expires) {
		return "expired"
	}
	return i.Status
}

func (s *Service) invitationView(p Principal, i TeamInvitation, team Team) map[string]any {
	status := invitationStatus(i)
	var user, inviter User
	s.DB.First(&user, "id = ?", i.UserID)
	s.DB.First(&inviter, "id = ?", i.Inviter)
	membershipStatus := "none"
	var m Membership
	if s.DB.First(&m, "user_id = ? AND tenant_id = ?", i.UserID, i.TenantID).Error == nil && m.Status != "" {
		membershipStatus = m.Status
	}
	next := "accept_team"
	switch {
	case status == "expired":
		next = "expired"
	case status == "accepted" || status == "declined" || status == "cancelled":
		// Terminal invitations leave the invitee no next action.
		next = "none"
	case p.TenantID != i.TenantID:
		next = "switch_tenant"
	case membershipStatus != "active":
		next = "accept_tenant"
	}
	return map[string]any{
		"id":                i.ID,
		"team_id":           i.TeamID,
		"tenant_id":         i.TenantID,
		"user_id":           i.UserID,
		"inviter":           i.Inviter,
		"status":            status,
		"expires_at":        i.Expires,
		"created_at":        i.Created,
		"user_name":         user.Name,
		"inviter_name":      inviter.Name,
		"membership_status": membershipStatus,
		"next_step":         next,
	}
}
