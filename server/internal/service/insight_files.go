package service

import (
	"net/http"
	"strings"
)

// Occupancy drill-down resolves current names through the same authorization
// rules as ordinary file reads. Names are not copied into audit records.
func (s *Service) usageFiles(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	space := q.Get("space_id")
	if space != "" {
		if _, e := s.space(p, space); e != nil {
			return e
		}
	}
	category := q.Get("category")
	if category == "" {
		category = "current"
	}
	if category != "current" && category != "history" && category != "trash" {
		return code("invalid_category")
	}
	files := []File{}
	query := s.DB.Where("owner = ?", p.dataOwner())
	if space != "" {
		query = query.Where("space_id = ?", space)
	}
	if query.Order("updated DESC").Find(&files).Error != nil {
		return code("database_unavailable")
	}
	out := []map[string]any{}
	partial := false
	for _, f := range files {
		if s.allow(p, f.SpaceID, f.Path, "read") != nil {
			continue
		}
		if category == "trash" && !f.Trashed || category != "trash" && f.Trashed {
			continue
		}
		versions := []Version{}
		vq := s.DB.Where("owner = ? AND file_id = ?", p.dataOwner(), f.ID)
		if category == "current" {
			vq = vq.Where("id = ?", f.VersionID)
		} else if category == "history" {
			vq = vq.Where("id <> ?", f.VersionID)
		}
		if vq.Order("created DESC").Find(&versions).Error != nil {
			return code("database_unavailable")
		}
		for _, v := range versions {
			if connection := q.Get("connection_id"); connection != "" && connection != v.ConnectionID {
				continue
			}
			if name := q.Get("q"); name != "" && !strings.Contains(strings.ToLower(f.Path), strings.ToLower(name)) {
				continue
			}
			pinned := f
			pinned.VersionID = v.ID
			out = append(out, map[string]any{"resource_id": f.ID, "version_id": v.ID, "ref": s.FileRef(pinned), "name": f.Path, "size_bytes": v.Size, "space_id": f.SpaceID, "category": category, "caller": v.Caller})
			if len(out) >= 200 {
				partial = true
				break
			}
		}
		if partial {
			break
		}
	}
	respond(w, map[string]any{"items": out, "partial": partial, "coverage": "authorized_managed_versions", "category": category})
	return nil
}
