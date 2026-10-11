package service

import (
	"net/http"
	"time"

	"gorm.io/gorm"
)

func activeUserKey(k UserKey) bool {
	return !k.Revoked && !k.Archived && !k.Disabled && (k.Expires.IsZero() || time.Now().Before(k.Expires))
}

func keyCopyReason(k UserKey, owner string) string {
	if k.Owner != owner {
		return "owner_only"
	}
	if !activeUserKey(k) {
		return "key_inactive"
	}
	if len(k.SealedKey) == 0 {
		return "key_material_unavailable"
	}
	return ""
}

func (s *Service) sealUserKey(k *UserKey, secret string) error {
	if Hash(secret) != k.TokenHash {
		return code("invalid_user_key")
	}
	blob, err := s.seal("user-key:"+k.Owner+":"+k.ID, secret)
	if err != nil {
		return err
	}
	k.SealedKey = blob
	return nil
}

// RevealUserKey retrieves only the actor's active login credential. It is
// separate from metadata listing and never grants cross-user admin access.
func (s *Service) RevealUserKey(p Principal, id string) (string, error) {
	if p.Agent != nil || p.ServiceIdentity != nil {
		return "", code("forbidden")
	}
	var k UserKey
	if s.DB.First(&k, "id = ? AND owner = ?", id, p.UserID).Error != nil {
		return "", code("not_found")
	}
	if !activeUserKey(k) {
		return "", code("key_inactive")
	}
	if len(k.SealedKey) == 0 {
		return "", code("key_material_unavailable")
	}
	var secret string
	if err := s.unseal("user-key:"+k.Owner+":"+k.ID, k.SealedKey, &secret); err != nil {
		return "", err
	}
	if Hash(secret) != k.TokenHash {
		return "", code("credential_unavailable")
	}
	return secret, nil
}

func (s *Service) keySecretRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/key-directory/{id}/reveal", s.wrap("keys.reveal", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		w.Header().Set("Cache-Control", "no-store")
		secret, err := s.RevealUserKey(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]string{"key": secret})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/key-directory/{id}/rotate", s.wrap("keys.rotate", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		w.Header().Set("Cache-Control", "no-store")
		if p.Agent != nil || p.ServiceIdentity != nil {
			return code("forbidden")
		}
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if !q.Confirm {
			return code("confirmation_required")
		}
		issued, id, err := s.rotateUserKey(r.PathValue("id"), 0, p.UserID, p.SessionHash)
		if err != nil {
			return err
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]any{"id": id, "key": issued.Key, "name": issued.Name, "user_id": issued.UserID, "expires_at": issued.ExpiresAt})
		return nil
	}))
}

func (s *Service) rotateUserKey(id string, days int, owner, sessionHash string) (UserKeyIssued, string, error) {
	if days < 0 {
		return UserKeyIssued{}, "", code("invalid_user_batch")
	}
	var issued UserKeyIssued
	var freshID string
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var old UserKey
		query := tx.Where("id = ?", id)
		if owner != "" {
			query = query.Where("owner = ?", owner)
		}
		if query.First(&old).Error != nil {
			return code("not_found")
		}
		if !activeUserKey(old) {
			return code("not_found")
		}
		secret := userKeyPrefix + ID() + ID()
		fresh := UserKey{ID: ID(), Owner: old.Owner, Name: old.Name, TokenHash: Hash(secret), Admin: old.Admin, Created: time.Now().UTC(), Expires: old.Expires}
		if days > 0 {
			fresh.Expires = fresh.Created.AddDate(0, 0, days)
		}
		if err := s.sealUserKey(&fresh, secret); err != nil {
			return err
		}
		// Compare-and-swap prevents duplicate successors and racing revocation.
		result := tx.Model(&UserKey{}).Where("id = ? AND revoked = ? AND archived = ?", old.ID, false, false).Updates(map[string]any{"revoked": true, "sealed_key": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return code("version_conflict")
		}
		if err := tx.Create(&fresh).Error; err != nil {
			return err
		}
		// Only the requesting browser survives rotation. Other sessions keep
		// their old origin and are rejected on their next authentication.
		if sessionHash != "" {
			if err := tx.Model(&Session{}).Where("hash = ? AND owner = ? AND origin_key = ?", sessionHash, old.Owner, old.ID).Update("origin_key", fresh.ID).Error; err != nil {
				return err
			}
		}
		issued = UserKeyIssued{UserID: fresh.Owner, Name: fresh.Name, Key: secret, Admin: fresh.Admin, ExpiresAt: fresh.Expires}
		freshID = fresh.ID
		return nil
	})
	if err != nil {
		return UserKeyIssued{}, "", err
	}
	return issued, freshID, nil
}
