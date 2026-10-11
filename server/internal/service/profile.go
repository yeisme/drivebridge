package service

import (
	"encoding/json"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Profile avatars stay private and reference an immutable version of a personal file.
const avatarMaxBytes = 2 << 20

var avatarMIMEs = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

func (s *Service) personalAvatarFiles(user string) *gorm.DB {
	owners := s.DB.Model(&Membership{}).Select("data_owner").Where("user_id = ? AND status = ?", user, "active")
	spaces := s.DB.Model(&Space{}).Select("id").Where("(team_id = ? OR team_id IS NULL) AND encrypted = ?", "", false).Where("owner = ? OR owner IN (?)", user, owners)
	return s.DB.Model(&File{}).Where("space_id IN (?) AND trashed = ? AND review_pending = ?", spaces, false, false).Where("mime IN ? AND size > ? AND size <= ?", avatarMIMEs, 0, avatarMaxBytes)
}

func (s *Service) avatarVersion(user, ref string) (File, Version, error) {
	var file File
	var version Version
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] != "file" || parts[1] == "" || parts[2] == "" || len(ref) > 240 {
		return file, version, code("invalid_avatar_ref")
	}
	if s.personalAvatarFiles(user).Where("id = ?", parts[1]).First(&file).Error != nil {
		return file, version, code("invalid_avatar_ref")
	}
	if s.DB.Where("id = ? AND file_id = ? AND owner = ? AND mime IN ? AND size > ? AND size <= ?", parts[2], file.ID, file.Owner, avatarMIMEs, 0, avatarMaxBytes).First(&version).Error != nil {
		return file, version, code("invalid_avatar_ref")
	}
	return file, version, nil
}

func profileSession(p Principal) bool {
	return p.SessionHash != "" && p.Agent == nil && p.ServiceIdentity == nil && p.ShareID == "" && p.AccessGrantHash == ""
}
func (s *Service) profileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PATCH /api/v1/me", s.wrap("me.update", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !profileSession(p) {
			return code("forbidden")
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var q struct {
			DisplayName *string         `json:"display_name"`
			AvatarRef   json.RawMessage `json:"avatar_ref"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		updates := map[string]any{}
		if q.DisplayName != nil {
			name := strings.TrimSpace(*q.DisplayName)
			if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 || strings.ContainsFunc(name, unicode.IsControl) {
				return code("invalid_display_name")
			}
			updates["display_name"] = name
		}
		if q.AvatarRef != nil {
			ref := ""
			if string(q.AvatarRef) != "null" {
				if json.Unmarshal(q.AvatarRef, &ref) != nil {
					return code("invalid_avatar_ref")
				}
			}
			if ref != "" {
				if _, _, err := s.avatarVersion(p.UserID, ref); err != nil {
					return err
				}
			}
			updates["avatar_ref"] = ref
		}
		if len(updates) == 0 {
			return code("invalid_profile_update")
		}
		result := s.DB.Model(&User{}).Where("id = ? AND archived = ? AND disabled = ?", p.UserID, false, false).Updates(updates)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return code("unauthorized")
		}
		var user User
		if s.DB.First(&user, "id = ?", p.UserID).Error != nil {
			return code("database_unavailable")
		}
		respond(w, map[string]any{"user": user})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/me/avatar-options", s.wrap("me.avatar.options", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !profileSession(p) {
			return code("forbidden")
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if utf8.RuneCountInString(q) > 80 {
			return code("invalid_avatar_ref")
		}
		query := s.personalAvatarFiles(p.UserID)
		if q != "" {
			query = query.Where("path LIKE ?", "%"+q+"%")
		}
		var files []File
		if query.Order("updated DESC").Limit(30).Find(&files).Error != nil {
			return code("database_unavailable")
		}
		items := []map[string]string{}
		for _, f := range files {
			items = append(items, map[string]string{"value": "file:" + f.ID + ":" + f.VersionID, "label": f.Path})
		}
		respond(w, map[string]any{"items": items})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/me/avatar", s.wrap("me.avatar.read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !profileSession(p) {
			return code("forbidden")
		}
		var user User
		if s.DB.First(&user, "id = ?", p.UserID).Error != nil {
			return code("unauthorized")
		}
		f, v, err := s.avatarVersion(p.UserID, user.AvatarRef)
		if err != nil {
			return code("not_found")
		}
		p.DataOwner = f.Owner
		p.ScopeSpace = f.SpaceID
		var sp Space
		if s.DB.First(&sp, "id = ?", f.SpaceID).Error != nil {
			return code("not_found")
		}
		p.TenantID = s.spaceTenant(sp)
		return s.mediaContent(w, r, p, f.ID, v.ID, false)
	}))
}
