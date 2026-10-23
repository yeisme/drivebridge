package service

import (
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Directory preserves empty folders. File identities remain independent of paths.
type Directory struct {
	ID      string `json:"id" gorm:"primaryKey"`
	Owner   string `json:"-" gorm:"index"`
	SpaceID string `json:"space_id" gorm:"uniqueIndex:directory_path"`
	Path    string `json:"path" gorm:"uniqueIndex:directory_path"`
}

type BrowserEntry struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	File *File  `json:"file,omitempty"`
}

func (s *Service) browserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/browser", s.wrap("browser.list", s.browseDirectory))
	mux.HandleFunc("POST /api/v1/directories", s.wrap("directories.create", s.createDirectoryHTTP))
	mux.HandleFunc("POST /api/v1/directories/lookup", s.wrapObserved("directories.create_lookup", s.lookupDirectoryHTTP, false))
	mux.HandleFunc("POST /api/v1/directories/copy", s.wrap("directories.copy", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q DirectoryCopyRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		op, err := s.CopyDirectory(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/directories/move", s.wrap("directories.move", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q DirectoryMoveRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		out, err := s.MoveDirectory(p, q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/directories/export", s.wrap("directories.export", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q DirectoryExportRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		op, err := s.ExportDirectory(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/directories/trash", s.wrap("directories.trash", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q DirectoryTrashRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		out, err := s.TrashDirectory(p, q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/copy", s.wrap("files.copy", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CopyRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		op, err := s.CopyFile(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/move", s.wrap("files.move", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Path            string `json:"path"`
			ExpectedVersion string `json:"expected_version"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		if _, err := cleanPath(q.Path); err != nil {
			return err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		f, err := s.file(p, r.PathValue("id"), "write")
		if err != nil {
			return err
		}
		if f.Trashed {
			return code("file_trashed")
		}
		if q.ExpectedVersion == "" || f.VersionID != q.ExpectedVersion {
			return code("version_conflict")
		}
		if err = s.allow(p, f.SpaceID, q.Path, "write"); err != nil {
			return err
		}
		if err = pathAvailable(s.DB, p.dataOwner(), f.SpaceID, q.Path, f.ID); err != nil {
			return err
		}
		var active int64
		if s.DB.Model(&Operation{}).Where("owner = ? AND file_id = ? AND status NOT IN ?", p.dataOwner(), f.ID, []string{"completed", "cancelled", "failed"}).Count(&active).Error != nil {
			return code("database_unavailable")
		}
		if active > 0 {
			return code("file_in_use")
		}
		f.Path, f.Updated = q.Path, time.Now().UTC()
		if err = s.DB.Transaction(func(tx *gorm.DB) error {
			if e := teamWritable(tx, f.SpaceID); e != nil {
				return e
			}
			// Targeted columns only: a full-row Save would clobber a
			// concurrent trash (and its trashed_at / version pointers).
			res := tx.Model(&File{}).Where("id = ? AND trashed = ?", f.ID, false).Updates(map[string]any{"path": f.Path, "updated": f.Updated})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return code("file_trashed")
			}
			return nil
		}); err != nil {
			if c := safeCode(err); c != "file_trashed" && c != "not_found" {
				return code("name_conflict")
			}
			return err
		}
		respond(w, f)
		return nil
	}))
}

func pathAvailable(db *gorm.DB, owner, space, target, except string) error {
	var fs []File
	var ds []Directory
	if db.Where("owner = ? AND space_id = ? AND id <> ?", owner, space, except).Find(&fs).Error != nil || db.Where("owner = ? AND space_id = ?", owner, space).Find(&ds).Error != nil {
		return code("database_unavailable")
	}
	for _, f := range fs {
		if contains(f.Path, target) || contains(target, f.Path) {
			return code("name_conflict")
		}
	}
	for _, d := range ds {
		if d.Path == target || strings.HasPrefix(d.Path, target+"/") {
			return code("name_conflict")
		}
	}
	return nil
}

func (s *Service) browseDirectory(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	space, dir := q.Get("space_id"), q.Get("path")
	if _, err := s.space(p, space); err != nil {
		return err
	}
	if dir != "" {
		if _, err := cleanPath(dir); err != nil {
			return err
		}
	}
	if err := s.allow(p, space, dir, "read"); err != nil {
		return err
	}
	var files []File
	var directories []Directory
	if s.DB.Where("owner = ? AND space_id = ? AND trashed = ?", p.dataOwner(), space, false).Find(&files).Error != nil || s.DB.Where("owner = ? AND space_id = ?", p.dataOwner(), space).Find(&directories).Error != nil {
		return code("database_unavailable")
	}
	entries := []BrowserEntry{}
	folders := map[string]bool{}
	search := strings.ToLower(q.Get("q"))
	sortKey := q.Get("sort")
	if sortKey == "" {
		sortKey = "name"
	}
	if sortKey != "name" && sortKey != "size" && sortKey != "date" {
		return code("invalid_sort")
	}
	order := q.Get("order")
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return code("invalid_order")
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	addFolder := func(full string) {
		if !strings.HasPrefix(full, prefix) || full == dir {
			return
		}
		rel := strings.TrimPrefix(full, prefix)
		name, _, _ := strings.Cut(rel, "/")
		if name != "" && (search == "" || strings.Contains(strings.ToLower(name), search)) {
			folders[prefix+name] = true
		}
	}
	for i := range files {
		f := &files[i]
		if !strings.HasPrefix(f.Path, prefix) || s.allow(p, space, f.Path, "read") != nil {
			continue
		}
		rel := strings.TrimPrefix(f.Path, prefix)
		if strings.Contains(rel, "/") {
			addFolder(path.Dir(f.Path))
		}
		if (search != "" && strings.Contains(strings.ToLower(rel), search)) || (search == "" && !strings.Contains(rel, "/")) {
			entries = append(entries, BrowserEntry{Kind: "file", Path: f.Path, File: f})
		}
	}
	for _, d := range directories {
		if s.allow(p, space, d.Path, "read") == nil {
			addFolder(d.Path)
		}
	}
	for folder := range folders {
		entries = append(entries, BrowserEntry{Kind: "directory", Path: folder})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == "directory"
		}
		less := entries[i].Path < entries[j].Path
		if sortKey != "name" && entries[i].File != nil && entries[j].File != nil {
			if sortKey == "size" && entries[i].File.Size != entries[j].File.Size {
				less = entries[i].File.Size < entries[j].File.Size
			} else if sortKey == "date" && !entries[i].File.Updated.Equal(entries[j].File.Updated) {
				less = entries[i].File.Updated.Before(entries[j].File.Updated)
			}
		}
		if order == "desc" {
			return !less && entries[i].Path != entries[j].Path
		}
		return less
	})
	start := 0
	if q.Get("cursor") != "" {
		n, err := strconv.Atoi(q.Get("cursor"))
		if err != nil || n < 0 || n > len(entries) {
			return code("invalid_cursor")
		}
		start = n
	}
	end := min(start+200, len(entries))
	next := ""
	if end < len(entries) {
		next = strconv.Itoa(end)
	}
	respond(w, map[string]any{"entries": entries[start:end], "next_cursor": next, "coverage": "managed_directory", "path": dir, "total": len(entries)})
	return nil
}
