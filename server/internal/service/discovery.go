package service

import (
	"strings"
	"time"
)

// PinnedFile preserves the requested version; a ref never silently upgrades.
func (s *Service) PinnedFile(p Principal, ref string) (map[string]any, error) {
	id, version, err := s.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	f, err := s.file(p, id, "read")
	if err != nil {
		return nil, err
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", version, id, p.dataOwner()).Error != nil {
		return nil, code("not_found")
	}
	f.VersionID, f.Size, f.MIME, f.Caller, f.TaskRef = v.ID, v.Size, v.MIME, v.Caller, v.TaskRef
	return map[string]any{"file": f, "ref": s.FileRef(f), "sha256": v.SHA256}, nil
}
func (s *Service) SearchFiles(p Principal, space, query, prefix, mime, after string, trash bool) ([]File, error) {
	if _, err := s.space(p, space); err != nil {
		return nil, err
	}
	if prefix != "" {
		if _, err := cleanPath(prefix); err != nil {
			return nil, err
		}
	}
	tx := s.DB.Where("owner = ? AND space_id = ? AND trashed = ?", p.dataOwner(), space, trash)
	if mime != "" {
		tx = tx.Where("mime = ?", mime)
	}
	if after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return nil, code("invalid_time")
		}
		tx = tx.Where("updated >= ?", t)
	}
	all, out := []File{}, []File{}
	if tx.Order("updated DESC").Find(&all).Error != nil {
		return nil, code("database_unavailable")
	}
	for _, f := range all {
		if contains(prefix, f.Path) && strings.Contains(strings.ToLower(f.Path), strings.ToLower(query)) && s.allow(p, space, f.Path, "read") == nil {
			out = append(out, f)
		}
	}
	s.attachPurgeAfter(out)
	return out, nil
}
