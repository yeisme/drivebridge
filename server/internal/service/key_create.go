package service

import (
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// New login keys always belong to the authenticated user, never a supplied owner.
func (s *Service) createOwnKey(w http.ResponseWriter, r *http.Request, p Principal) error {
	w.Header().Set("Cache-Control", "no-store")
	if p.Agent != nil || p.ServiceIdentity != nil || p.UserID == "" {
		return code("forbidden")
	}
	var q struct {
		Name string `json:"name"`
		Days int    `json:"days"`
	}
	if err := decode(r, &q); err != nil {
		return err
	}
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" || utf8.RuneCountInString(q.Name) > 80 || strings.ContainsAny(q.Name, "\r\n\x00") || q.Days < 0 || q.Days > 3650 {
		return code("invalid_user_batch")
	}
	secret := userKeyPrefix + ID() + ID()
	key := UserKey{ID: ID(), Owner: p.UserID, Name: q.Name, TokenHash: Hash(secret), Admin: p.Admin, Created: time.Now().UTC()}
	if q.Days > 0 {
		key.Expires = key.Created.AddDate(0, 0, q.Days)
	}
	if err := s.sealUserKey(&key, secret); err != nil {
		return err
	}
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if tx.First(&user, "id = ?", p.UserID).Error != nil || user.Disabled || user.Archived {
			return code("forbidden")
		}
		return tx.Create(&key).Error
	}); err != nil {
		return err
	}
	markAuditTarget(r, key.ID)
	respond(w, map[string]any{"id": key.ID, "key": secret, "name": key.Name, "user_id": key.Owner, "expires_at": key.Expires})
	return nil
}
