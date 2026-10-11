package service

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

// Governance is activated explicitly by a state-directory operator.
type GovernanceState struct {
	ID     uint `gorm:"primaryKey"`
	Active bool
}
type Tenant struct {
	ID       string `json:"id" gorm:"primaryKey"`
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}
type Membership struct {
	ID            string `json:"id" gorm:"primaryKey"`
	TenantID      string `json:"tenant_id" gorm:"uniqueIndex:tenant_member"`
	UserID        string `json:"user_id" gorm:"uniqueIndex:tenant_member"`
	DataOwner     string `json:"-" gorm:"uniqueIndex"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	SuspendedRole string `json:"-"`
}
type PlatformRole struct {
	UserID    string `json:"user_id" gorm:"primaryKey"`
	Suspended bool   `json:"suspended"`
}
type Team struct {
	Description string    `json:"description"`
	Archived    bool      `json:"archived" gorm:"not null;default:false"`
	Revision    uint64    `json:"revision" gorm:"not null;default:1"`
	UpdatedAt   time.Time `json:"updated_at"`
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index"`
	Name        string    `json:"name"`
}
type TeamMember struct {
	TeamID        string `json:"team_id" gorm:"primaryKey"`
	UserID        string `json:"user_id" gorm:"primaryKey"`
	Role          string `json:"role"`
	SuspendedRole string `json:"-"`
}
type SpacePermission struct {
	SpaceID       string `json:"space_id" gorm:"primaryKey"`
	UserID        string `json:"user_id" gorm:"primaryKey"`
	Role          string `json:"role"`
	SuspendedRole string `json:"-"`
}
type GovernanceEvent struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	TenantID   string    `json:"tenant_id" gorm:"index"`
	Actor      string    `json:"actor"`
	SpaceID    string    `json:"space_id"`
	ResourceID string    `json:"resource_id"`
	RequestID  string    `json:"request_id"`
	Action     string    `json:"action"`
	Outcome    string    `json:"outcome"`
	At         time.Time `json:"at"`
	Dedupe     string    `json:"-" gorm:"index"`
}
type Notification struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	TenantID   string    `json:"tenant_id" gorm:"index"`
	UserID     string    `json:"-" gorm:"index"`
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	ResourceID string    `json:"resource_id"`
	Actor      string    `json:"actor"`
	Read       bool      `json:"read"`
	Created    time.Time `json:"created_at"`
}
type AccessRequest struct {
	ID         string            `json:"id" gorm:"primaryKey"`
	TenantID   string            `json:"tenant_id" gorm:"index"`
	SpaceID    string            `json:"space_id"`
	Requester  string            `json:"requester"`
	Approver   string            `json:"approver"`
	Reason     string            `json:"reason"`
	Kind       string            `json:"kind"`
	Action     string            `json:"action"`
	Prefix     string            `json:"prefix"`
	Status     string            `json:"status"`
	Hours      int               `json:"hours"`
	Created    time.Time         `json:"created_at"`
	Expires    time.Time         `json:"expires_at"`
	GrantUntil time.Time         `json:"grant_until"`
	Items      []RequestedAction `json:"items" gorm:"serializer:json"`
}
type RequestedAction struct {
	SourceFileID    string `json:"source_file_id,omitempty"`
	SourceVersionID string `json:"source_version_id,omitempty"`
	ResultID        string `json:"result_id,omitempty"`
	FileID          string `json:"file_id"`
	VersionID       string `json:"version_id"`
	Action          string `json:"action"`
	Path            string `json:"path,omitempty"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

func (p Principal) dataOwner() string {
	if p.DataOwner != "" {
		return p.DataOwner
	}
	return p.UserID
}
func (s *Service) governanceActive() bool { return s.governance.Load() }
func (s *Service) dbHandle(dbs []*gorm.DB) *gorm.DB {
	if len(dbs) > 0 && dbs[0] != nil {
		return dbs[0]
	}
	return s.DB
}

func (s *Service) platformAdmin(user string, dbs ...*gorm.DB) bool {
	db := s.dbHandle(dbs)
	var u User
	if db.First(&u, "id = ? AND disabled = ?", user, false).Error != nil {
		return false
	}
	var v PlatformRole
	return db.First(&v, "user_id = ? AND suspended = ?", user, false).Error == nil
}
func (s *Service) membership(user, tenant string, dbs ...*gorm.DB) (Membership, error) {
	db := s.dbHandle(dbs)
	var m Membership
	var t Tenant
	if db.First(&t, "id = ? AND disabled = ?", tenant, false).Error != nil || db.First(&m, "tenant_id = ? AND user_id = ? AND status = ?", tenant, user, "active").Error != nil {
		return m, code("forbidden")
	}
	return m, nil
}
func (s *Service) tenantAdmin(user, tenant string, dbs ...*gorm.DB) bool {
	if s.platformAdmin(user, dbs...) {
		return true
	}
	m, e := s.membership(user, tenant, dbs...)
	return e == nil && m.Role == "admin"
}
func (s *Service) teamRole(user, team string, dbs ...*gorm.DB) string {
	db := s.dbHandle(dbs)
	var t Team
	if db.First(&t, "id = ?", team).Error != nil {
		return ""
	}
	if _, e := s.membership(user, t.TenantID, dbs...); e != nil {
		return ""
	}
	var m TeamMember
	db.First(&m, "team_id = ? AND user_id = ?", team, user)
	return m.Role
}
func (s *Service) permission(user, space string, dbs ...*gorm.DB) string {
	db := s.dbHandle(dbs)
	var v SpacePermission
	db.First(&v, "space_id = ? AND user_id = ?", space, user)
	return v.Role
}
func (s *Service) spaceTenant(sp Space) string {
	if sp.TenantID == "" {
		return s.Instance
	}
	return sp.TenantID
}
func (s *Service) scopeRole(p Principal, sp Space, dbs ...*gorm.DB) string {
	db := s.dbHandle(dbs)
	role := s.activeScopeRole(p, sp, dbs...)
	if role != "" && sp.TeamID != "" {
		var t Team
		if db.First(&t, "id = ?", sp.TeamID).Error != nil {
			return ""
		}
		if t.Archived {
			return "viewer"
		}
	}
	return role
}
func (s *Service) activeScopeRole(p Principal, sp Space, dbs ...*gorm.DB) string {
	db := s.dbHandle(dbs)
	var actor User
	if db.First(&actor, "id = ? AND disabled = ?", p.UserID, false).Error != nil {
		return ""
	}
	tenant := s.spaceTenant(sp)
	if p.TenantID != "" && tenant != p.TenantID {
		return ""
	}
	var t Tenant
	if db.First(&t, "id = ? AND disabled = ?", tenant, false).Error != nil {
		return ""
	}
	m, e := s.membership(p.UserID, tenant, dbs...)
	if e == nil && sp.TeamID == "" && m.DataOwner == sp.Owner {
		return "owner"
	}
	if e != nil {
		if s.platformAdmin(p.UserID, dbs...) {
			return "admin"
		}
		return ""
	}
	if sp.TeamID != "" && s.teamRole(p.UserID, sp.TeamID, dbs...) != "" {
		if role := s.permission(p.UserID, sp.ID, dbs...); role != "" {
			return role
		}
	}
	if s.platformAdmin(p.UserID, dbs...) || m.Role == "admin" || (sp.TeamID != "" && s.teamRole(p.UserID, sp.TeamID, dbs...) == "admin") {
		return "admin"
	}
	return ""
}

// governanceAllow runs on the caller's DB handle: inside an open
// transaction every lookup must use that handle or the single pooled
// connection deadlocks against the transaction holding it.
func (s *Service) governanceAllow(db *gorm.DB, p Principal, space, path, action string) error {
	if !s.governanceActive() {
		return nil
	}
	if err := s.governanceCredential(p, db); err != nil {
		return err
	}
	var sp Space
	if err := db.First(&sp, "id = ? AND owner = ?", space, p.dataOwner()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		return code("database_unavailable")
	}
	if p.ScopeSpace != "" && p.ScopeSpace != space {
		return code("not_found")
	}
	role := s.scopeRole(p, sp, db)
	if role == "" {
		return code("forbidden")
	}
	if sp.TeamID != "" {
		var team Team
		if err := db.First(&team, "id = ?", sp.TeamID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		if team.Archived && (action != "read" || p.Agent != nil || p.ExternalRead) {
			return code("team_archived")
		}
	}
	if action == "read" && (role != "admin" || !s.Config.RequireExplicitAdminRead) {
		return nil
	}
	if role == "owner" || ((role == "editor" || role == "custodian") && action != "permission") {
		return nil
	}
	if p.Agent != nil {
		return code("forbidden")
	}
	if p.ExecutionID != "" {
		var req AccessRequest
		if db.First(&req, "id = ? AND requester = ? AND space_id = ? AND status = ?", p.ExecutionID, p.UserID, space, "executing").Error == nil && time.Now().Before(req.GrantUntil) {
			if !s.canApprove(Principal{UserID: req.Approver, TenantID: req.TenantID}, sp, req.Requester, db) {
				return code("approval_required")
			}
			for _, item := range req.Items {
				if item.Status == "running" && (actionPermission(item.Action) == action || item.Action == "overwrite" && action == "overwrite") {
					var f File
					if db.First(&f, "id = ? AND space_id = ? AND version_id = ?", item.FileID, space, item.VersionID).Error == nil && f.Path == path {
						return nil
					}
				}
			}
		}
	}
	var grants []AccessRequest
	if db.Where("requester = ? AND space_id = ? AND status = ?", p.UserID, space, "approved").Find(&grants).Error != nil {
		return code("database_unavailable")
	}
	for _, g := range grants {
		// Compare instants, not SQLite timestamp text with mixed UTC offsets.
		if !time.Now().Before(g.GrantUntil) {
			continue
		}
		if g.Kind == "emergency" && s.platformAdmin(p.UserID, db) && contains(g.Prefix, path) && (g.Action == action || g.Action == "all" || action == "overwrite" && g.Action == "write") {
			return nil
		}
		if g.Kind == "range" && g.Action == action && contains(g.Prefix, path) && s.canApprove(Principal{UserID: g.Approver, TenantID: g.TenantID}, sp, g.Requester, db) {
			return nil
		}
	}
	return code("approval_required")
}
func (s *Service) canApprove(p Principal, sp Space, requester string, dbs ...*gorm.DB) bool {
	db := s.dbHandle(dbs)
	if sp.TeamID != "" && teamWritable(db, sp.ID) != nil {
		return false
	}
	if p.UserID == requester || p.Agent != nil {
		return false
	}
	if sp.TeamID == "" {
		m, e := s.membership(p.UserID, s.spaceTenant(sp), dbs...)
		return e == nil && m.DataOwner == sp.Owner
	}
	if s.tenantAdmin(p.UserID, s.spaceTenant(sp), dbs...) {
		return true
	}
	_, e := s.membership(p.UserID, s.spaceTenant(sp), dbs...)
	return e == nil && s.teamRole(p.UserID, sp.TeamID, dbs...) != "" && s.permission(p.UserID, sp.ID, dbs...) == "custodian"
}

// bindContext validates the requested target without replacing the real actor.
func (s *Service) bindContext(p Principal, r *http.Request) (Principal, error) {
	if !s.governanceActive() {
		if r.Header.Get("X-DriveBridge-Tenant") != "" || r.Header.Get("X-DriveBridge-Space") != "" || r.URL.Query().Get("tenant_context") != "" || r.URL.Query().Get("space_context") != "" {
			return p, code("context_unavailable")
		}
		return p, nil
	}
	var u User
	if s.DB.First(&u, "id = ?", p.UserID).Error != nil || u.Disabled {
		return p, code("unauthorized")
	}
	tenant := r.Header.Get("X-DriveBridge-Tenant")
	if tenant == "" {
		tenant = r.URL.Query().Get("tenant_context")
	}
	if tenant == "" {
		if p.ServiceIdentity != nil {
			tenant = p.ServiceIdentity.localTenant
		} else {
			tenant = s.Instance
		}
	}
	if p.ServiceIdentity != nil && tenant != p.ServiceIdentity.localTenant {
		return p, code("cross_tenant_admin_context_required")
	}
	p.TenantID = tenant
	p.Admin = p.Agent == nil && s.platformAdmin(p.UserID)
	member, e := s.membership(p.UserID, tenant)
	if e == nil {
		p.DataOwner = member.DataOwner
	}
	target := r.Header.Get("X-DriveBridge-Space")
	if target == "" {
		target = r.URL.Query().Get("space_context")
	}
	if p.Agent != nil {
		if target != "" && target != p.Agent.SpaceID {
			return p, code("forbidden")
		}
		var sp Space
		if s.DB.First(&sp, "id = ?", p.Agent.SpaceID).Error != nil {
			return p, code("forbidden")
		}
		if r.Header.Get("X-DriveBridge-Tenant") == "" && r.URL.Query().Get("tenant_context") == "" {
			p.TenantID = s.spaceTenant(sp)
		}
		member, err := s.membership(p.UserID, s.spaceTenant(sp))
		if err != nil || member.DataOwner != sp.Owner {
			return p, code("forbidden")
		}
		target = sp.ID
	}
	if target != "" {
		var sp Space
		if s.DB.First(&sp, "id = ?", target).Error != nil || s.scopeRole(p, sp) == "" {
			return p, code("forbidden")
		}
		p.DataOwner, p.ScopeSpace = sp.Owner, sp.ID
	}
	// Identity and invitations remain accessible after a membership is removed.
	identity := strings.HasPrefix(r.URL.Path, "/api/v1/governance") || r.URL.Path == "/api/v1/me" || r.URL.Path == "/auth/logout"
	if e != nil && !identity && p.ScopeSpace == "" {
		return p, code("forbidden")
	}
	return p, nil
}
func (s *Service) governanceGuard(p Principal, action string, r *http.Request) error {
	if err := s.serviceIdentityGuard(p, action); err != nil {
		return err
	}
	if !s.governanceActive() {
		return nil
	}
	if p.ScopeSpace != "" {
		var sp Space
		if s.DB.First(&sp, "id = ?", p.ScopeSpace).Error != nil {
			return code("forbidden")
		}
		role := s.scopeRole(p, sp)
		if role != "owner" && strings.HasPrefix(action, "creative.") {
			return code("forbidden")
		}
		if role == "" {
			return code("forbidden")
		}
		// Connection and credential authority never follows data access.
		if role != "owner" && ((strings.HasPrefix(action, "connections.") && action != "connections.list") || (strings.HasPrefix(action, "agents.") && action != "agents.list") || strings.HasPrefix(action, "spaces.") && action != "spaces.list" || strings.HasPrefix(action, "users.") || (action == "files.access" && sp.TeamID == "") || action == "usage.scan") {
			return code("forbidden")
		}
		if role != "owner" && (action == "shares.link" || action == "shares.revoke") {
			var share Share
			if s.DB.First(&share, "id = ? AND owner = ?", r.PathValue("id"), p.dataOwner()).Error != nil {
				return code("not_found")
			}
			if share.ActorID != p.UserID {
				if e := s.allow(p, sp.ID, share.Path, "share"); e != nil {
					return e
				}
			}
		}
		if role != "owner" {
			kind := "view"
			if strings.Contains(action, "content") || strings.Contains(action, "download") || strings.Contains(action, "export") {
				kind = "download"
			}
			if err := s.recordAccess(p, sp, kind, r.PathValue("id")); err != nil {
				return err
			}
		}
	}
	if strings.HasPrefix(action, "mcp.users_") && action != "mcp.users_list" {
		return code("account_recovery_required")
	}
	if strings.HasPrefix(action, "users.") && action != "users.list" {
		return code("account_recovery_required")
	}
	if (action == "users.list" || action == "mcp.users_list") && (p.Agent != nil || !s.platformAdmin(p.UserID)) {
		return code("forbidden")
	}
	return nil
}

// BootstrapGovernance is a deployment-side migration, never an HTTP action.
func (s *Service) BootstrapGovernance(admin string) error {
	s.governanceMu.Lock()
	defer s.governanceMu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var u User
		if tx.First(&u, "id = ?", admin).Error != nil {
			return code("not_found")
		}
		t := Tenant{ID: s.Instance, Name: defaultString(s.Config.TenantName, "默认租户")}
		if err := tx.Where("id = ?", t.ID).FirstOrCreate(&t).Error; err != nil {
			return err
		}
		var users []User
		if err := tx.Where("? = ?", s.governanceActive(), false).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			m := Membership{ID: ID(), TenantID: s.Instance, UserID: user.ID, DataOwner: user.ID, Role: "member", Status: "active"}
			var existing Membership
			if e := tx.Where("tenant_id = ? AND user_id = ?", m.TenantID, m.UserID).First(&existing).Error; e == gorm.ErrRecordNotFound {
				if e = tx.Create(&m).Error; e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
		}
		if err := tx.Model(&Space{}).Where("tenant_id = ? OR tenant_id IS NULL", "").Update("tenant_id", s.Instance).Error; err != nil {
			return err
		}
		if err := tx.Save(&PlatformRole{UserID: admin}).Error; err != nil {
			return err
		}
		if err := tx.Save(&GovernanceState{ID: 1, Active: true}).Error; err != nil {
			return err
		}
		return tx.Create(&GovernanceEvent{ID: ID(), TenantID: s.Instance, Actor: admin, Action: "governance.bootstrap", Outcome: "success", At: time.Now().UTC()}).Error
	})
	if err == nil {
		s.governance.Store(true)
	}
	return err
}

func (s *Service) governanceCredential(p Principal, dbs ...*gorm.DB) error {
	db := s.dbHandle(dbs)
	if p.SessionHash != "" {
		var v Session
		if db.First(&v, "hash = ? AND owner = ?", p.SessionHash, p.UserID).Error != nil || !time.Now().Before(v.Expires) {
			return code("unauthorized")
		}
		if v.OriginKey != "" {
			var k UserKey
			if db.First(&k, "id = ? AND revoked = ?", v.OriginKey, false).Error != nil || !k.Expires.IsZero() && !time.Now().Before(k.Expires) {
				return code("unauthorized")
			}
		}
	}
	if p.Key != nil {
		var k UserKey
		if db.First(&k, "id = ? AND revoked = ?", p.Key.ID, false).Error != nil || !k.Expires.IsZero() && !time.Now().Before(k.Expires) {
			return code("unauthorized")
		}
	}
	return nil
}
