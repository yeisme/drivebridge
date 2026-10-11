package service

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

func (s *Service) keyDirectoryRoutes(mux *http.ServeMux) {
	s.keySecretRoutes(mux)
	mux.HandleFunc("POST /api/v1/key-directory", s.wrap("keys.create", s.createOwnKey))
	mux.HandleFunc("GET /api/v1/key-directory", s.wrap("keys.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil || p.ServiceIdentity != nil {
			return code("forbidden")
		}
		query := s.DB.Model(&UserKey{})
		if !p.Admin || s.governanceActive() {
			query = query.Where("owner = ?", p.UserID)
		}
		keys := []UserKey{}
		if query.Order("created_at DESC").Find(&keys).Error != nil {
			return code("database_unavailable")
		}
		ids := []string{}
		for _, k := range keys {
			ids = append(ids, k.Owner)
		}
		users := []User{}
		if len(ids) > 0 && s.DB.Where("id IN ?", ids).Find(&users).Error != nil {
			return code("database_unavailable")
		}
		names := map[string]string{}
		for _, u := range users {
			names[u.ID] = u.Name
		}
		current := ""
		if p.Key != nil {
			current = p.Key.ID
		} else if p.SessionHash != "" {
			var session Session
			if s.DB.First(&session, "hash = ?", p.SessionHash).Error == nil {
				current = session.OriginKey
			}
		}
		activeByOwner := map[string]int{}
		for _, k := range keys {
			if activeUserKey(k) {
				activeByOwner[k.Owner]++
			}
		}
		out := []map[string]any{}
		for _, k := range keys {
			revoke, del, disable, enable := s.keyLifecycleAllowed(p, k, activeByOwner[k.Owner])
			out = append(out, map[string]any{"id": k.ID, "name": k.Name, "user": k.Owner, "user_name": names[k.Owner], "admin": k.Admin, "created_at": k.Created, "expires_at": k.Expires, "revoked": k.Revoked, "archived": k.Archived, "disabled": k.Disabled, "current": k.ID == current, "can_copy": k.Owner == p.UserID && activeUserKey(k) && len(k.SealedKey) > 0, "can_rotate": k.Owner == p.UserID && activeUserKey(k), "can_revoke": revoke, "can_delete": del, "can_disable": disable, "can_enable": enable, "copy_unavailable_reason": keyCopyReason(k, p.UserID)})
		}
		respond(w, map[string]any{"keys": out})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/label", s.wrap("keys.label", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil || p.ServiceIdentity != nil {
			return code("forbidden")
		}
		var q struct {
			Name string `json:"name"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		q.Name = strings.TrimSpace(q.Name)
		if q.Name == "" || utf8.RuneCountInString(q.Name) > 80 || strings.ContainsAny(q.Name, "\r\n\x00") {
			return code("invalid_user_batch")
		}
		query := s.DB.Model(&UserKey{}).Where("id = ? AND archived = ?", r.PathValue("id"), false)
		if !p.Admin || s.governanceActive() {
			query = query.Where("owner = ?", p.UserID)
		}
		result := query.Update("name", q.Name)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected == 0 {
			return code("not_found")
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"updated": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/revoke", s.wrap("keys.revoke", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if !q.Confirm {
			return code("confirmation_required")
		}
		if e := s.selfRevokeKey(p, r.PathValue("id"), false); e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"revoked": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/delete", s.wrap("keys.delete", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if !q.Confirm {
			return code("confirmation_required")
		}
		if e := s.selfRevokeKey(p, r.PathValue("id"), true); e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"deleted": true, "revoked": true, "archived": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/disable", s.wrap("keys.disable", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if !q.Confirm {
			return code("confirmation_required")
		}
		if e := s.setKeyDisabled(p, r.PathValue("id"), true); e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"disabled": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/enable", s.wrap("keys.enable", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if e := s.setKeyDisabled(p, r.PathValue("id"), false); e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]bool{"disabled": false})
		return nil
	}))
}

func (s *Service) keyLifecycleAllowed(p Principal, k UserKey, activeCount int) (revoke, del, disable, enable bool) {
	if p.Agent != nil || p.ServiceIdentity != nil {
		return false, false, false, false
	}
	owner := k.Owner == p.UserID
	adminScope := p.Admin && !s.governanceActive()
	if !owner && !adminScope {
		return false, false, false, false
	}
	if k.Archived || k.Revoked {
		return false, true, false, false
	}
	expired := !k.Expires.IsZero() && !time.Now().Before(k.Expires)
	if k.Disabled {
		return true, true, false, !expired
	}
	if expired {
		return true, true, false, false
	}
	if activeCount < 2 {
		return false, false, false, false
	}
	return true, true, true, false
}

func (s *Service) selfRevokeKey(p Principal, id string, destroy bool) error {
	if p.Agent != nil || p.ServiceIdentity != nil {
		return code("forbidden")
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var k UserKey
		query := tx.Where("id = ?", id)
		if !p.Admin || s.governanceActive() {
			query = query.Where("owner = ?", p.UserID)
		}
		if query.First(&k).Error != nil {
			return code("not_found")
		}
		if k.Owner != p.UserID && (!p.Admin || s.governanceActive()) {
			return code("forbidden")
		}
		var siblings []UserKey
		if tx.Where("owner = ?", k.Owner).Find(&siblings).Error != nil {
			return code("database_unavailable")
		}
		active := 0
		adminActive := 0
		for _, other := range siblings {
			if activeUserKey(other) {
				active++
				if other.Admin {
					adminActive++
				}
			}
		}
		if activeUserKey(k) && active < 2 {
			return code("last_active_key")
		}
		if k.Admin && activeUserKey(k) && adminActive < 2 {
			return code("last_admin_required")
		}
		updates := map[string]any{"revoked": true, "sealed_key": nil}
		if destroy {
			updates["archived"] = true
		}
		if e := tx.Model(&k).Updates(updates).Error; e != nil {
			return e
		}
		return tx.Where("origin_key = ?", k.ID).Delete(&Session{}).Error
	})
}

func (s *Service) setKeyDisabled(p Principal, id string, disabled bool) error {
	if p.Agent != nil || p.ServiceIdentity != nil {
		return code("forbidden")
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var k UserKey
		operator := p.Admin && p.UserID == "" && p.Agent == nil
		query := tx.Where("id = ?", id)
		if !operator && (!p.Admin || s.governanceActive()) {
			query = query.Where("owner = ?", p.UserID)
		}
		if query.First(&k).Error != nil {
			return code("not_found")
		}
		if !operator && k.Owner != p.UserID && (!p.Admin || s.governanceActive()) {
			return code("forbidden")
		}
		if k.Revoked || k.Archived || (!k.Expires.IsZero() && !time.Now().Before(k.Expires)) {
			return code("key_inactive")
		}
		if k.Disabled == disabled {
			return nil
		}
		if disabled {
			var siblings []UserKey
			if tx.Where("owner = ?", k.Owner).Find(&siblings).Error != nil {
				return code("database_unavailable")
			}
			active, adminActive := 0, 0
			for _, other := range siblings {
				if activeUserKey(other) {
					active++
					if other.Admin {
						adminActive++
					}
				}
			}
			if active < 2 {
				return code("last_active_key")
			}
			if k.Admin && adminActive < 2 {
				return code("last_admin_required")
			}
			if e := tx.Model(&k).Update("disabled", true).Error; e != nil {
				return e
			}
			return tx.Where("origin_key = ?", k.ID).Delete(&Session{}).Error
		}
		return tx.Model(&k).Update("disabled", false).Error
	})
}
