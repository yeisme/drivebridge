package service

import (
	"net/http"
	"time"

	"gorm.io/gorm"
)

func (s *Service) keyArchiveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/users/{id}/archive", s.wrap("users.archive", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !p.Admin || p.Agent != nil {
			return code("forbidden")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			var k UserKey
			if tx.First(&k, "id = ?", r.PathValue("id")).Error != nil {
				return code("not_found")
			}
			if k.Admin && activeUserKey(k) {
				var keys []UserKey
				if tx.Where("admin = ? AND revoked = ? AND archived = ? AND disabled = ? AND id <> ?", true, false, false, false, k.ID).Find(&keys).Error != nil {
					return code("database_unavailable")
				}
				usable := false
				for _, other := range keys {
					if other.Expires.IsZero() || time.Now().Before(other.Expires) {
						usable = true
					}
				}
				if !usable {
					return code("last_admin_required")
				}
			}
			if err := tx.Model(&k).Updates(map[string]any{"revoked": true, "archived": true}).Error; err != nil {
				return err
			}
			return tx.Where("origin_key = ?", k.ID).Delete(&Session{}).Error
		})
		if err != nil {
			return err
		}
		respond(w, map[string]bool{"archived": true, "revoked": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/agents/{id}/archive", s.wrap("agents.archive", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		result := s.DB.Model(&Agent{}).Where("id = ? AND owner = ?", r.PathValue("id"), p.UserID).Updates(map[string]any{"revoked": true, "archived": true})
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected == 0 {
			return code("not_found")
		}
		respond(w, map[string]bool{"archived": true, "revoked": true})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/usage", s.wrap("files.usage", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		f, err := s.file(p, r.PathValue("id"), "read")
		if err != nil {
			return err
		}
		out, err := s.fileUse(p, f)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
}
