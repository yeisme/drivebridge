package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const directoryCopyLimit = 1000

type DirectoryCopyRequest struct {
	SpaceID       string `json:"space_id"`
	Path          string `json:"path"`
	TargetSpaceID string `json:"target_space_id"`
	TargetPath    string `json:"target_path"`
	Conflict      string `json:"conflict,omitempty"`
}

type DirectoryMoveRequest struct {
	SpaceID    string `json:"space_id"`
	Path       string `json:"path"`
	TargetPath string `json:"target_path"`
	Conflict   string `json:"conflict,omitempty"`
}

type DirectoryTrashRequest struct {
	SpaceID string `json:"space_id"`
	Path    string `json:"path"`
}

func prefixOf(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func remapPrefix(path, src, dest string) string {
	if path == src {
		return dest
	}
	return dest + strings.TrimPrefix(path, src)
}

func insidePath(dest, src string) bool {
	return dest == src || strings.HasPrefix(dest, src+"/")
}

func directoryConflict(conflict string) (string, error) {
	if conflict == "" {
		return "rename", nil
	}
	if conflict != "rename" && conflict != "fail" {
		return "", code("invalid_directory")
	}
	return conflict, nil
}

func uniqueDirPath(tx *gorm.DB, owner, space, requested string) (string, error) {
	path := requested
	for n := 2; n < 1000; n++ {
		if err := pathAvailable(tx, owner, space, path, ""); err == nil {
			return path, nil
		} else if err.Error() != "name_conflict" {
			return "", err
		}
		path = requested + " (" + strconv.Itoa(n) + ")"
	}
	return "", code("name_conflict")
}

func (s *Service) listPrefix(p Principal, space, prefix string) ([]File, []Directory, error) {
	var files []File
	var dirs []Directory
	if s.DB.Where("owner = ? AND space_id = ? AND trashed = ?", p.dataOwner(), space, false).Find(&files).Error != nil ||
		s.DB.Where("owner = ? AND space_id = ?", p.dataOwner(), space).Find(&dirs).Error != nil {
		return nil, nil, code("database_unavailable")
	}
	outFiles := files[:0]
	for _, f := range files {
		if strings.HasPrefix(f.Path, prefix+"/") && s.allow(p, space, f.Path, "read") == nil {
			outFiles = append(outFiles, f)
		}
	}
	outDirs := dirs[:0]
	for _, d := range dirs {
		if prefixOf(d.Path, prefix) && s.allow(p, space, d.Path, "read") == nil {
			outDirs = append(outDirs, d)
		}
	}
	return outFiles, outDirs, nil
}

func (s *Service) CopyDirectory(ctx context.Context, p Principal, q DirectoryCopyRequest, key string) (Operation, error) {
	if key == "" || len(key) > 200 || q.SpaceID == "" || q.TargetSpaceID == "" {
		return Operation{}, code("invalid_directory")
	}
	src, err := cleanPath(q.Path)
	if err != nil {
		return Operation{}, err
	}
	destReq, err := cleanPath(q.TargetPath)
	if err != nil {
		return Operation{}, err
	}
	conflict, err := directoryConflict(q.Conflict)
	if err != nil {
		return Operation{}, err
	}
	if e := s.requireManagedTarget(p, q.TargetSpaceID); e != nil {
		return Operation{}, e
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return Operation{}, e
	}
	if e := s.allow(p, q.SpaceID, src, "read"); e != nil {
		return Operation{}, e
	}
	if e := s.allow(p, q.TargetSpaceID, destReq, "write"); e != nil {
		return Operation{}, e
	}
	files, dirs, err := s.listPrefix(p, q.SpaceID, src)
	if err != nil {
		return Operation{}, err
	}
	if len(files) > directoryCopyLimit {
		return Operation{}, code("directory_too_large")
	}
	body, _ := json.Marshal(q)
	fingerprint := Hash("directory_copy:" + string(body))
	s.mu.Lock()
	var op Operation
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, q.TargetSpaceID); e != nil {
			return e
		}
		var old Operation
		if tx.First(&old, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
			if old.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			op = old
			return nil
		}
		dest := destReq
		if conflict == "rename" {
			var e error
			dest, e = uniqueDirPath(tx, p.dataOwner(), q.TargetSpaceID, destReq)
			if e != nil {
				return e
			}
		} else if e := pathAvailable(tx, p.dataOwner(), q.TargetSpaceID, destReq, ""); e != nil {
			return e
		}
		if e := s.allow(p, q.TargetSpaceID, dest, "write"); e != nil {
			return e
		}
		agent := ""
		if p.Agent != nil {
			agent = p.Agent.ID
		}
		var size int64
		for _, f := range files {
			size += f.Size
		}
		op = Operation{
			ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), AgentID: agent, Key: key,
			Fingerprint: fingerprint, Kind: "directory_copy", SpaceID: q.TargetSpaceID, Path: dest,
			SourceSpaceID: q.SpaceID, SourcePath: src, Status: "running", Size: size,
			TotalFiles: len(files), Updated: time.Now().UTC(),
		}
		return tx.Create(&op).Error
	})
	s.mu.Unlock()
	if err != nil {
		return Operation{}, err
	}
	if op.Status == "completed" || op.Status == "cancelled" {
		return op, nil
	}
	return s.runDirectoryCopy(ctx, p, op, files, dirs)
}

func (s *Service) runDirectoryCopy(ctx context.Context, p Principal, op Operation, files []File, dirs []Directory) (Operation, error) {
	dest, src := op.Path, op.SourcePath
	seen := map[string]bool{dest: true}
	toCreate := []Directory{{ID: ID(), Owner: p.dataOwner(), SpaceID: op.SpaceID, Path: dest}}
	for _, d := range dirs {
		next := remapPrefix(d.Path, src, dest)
		if seen[next] {
			continue
		}
		seen[next] = true
		toCreate = append(toCreate, Directory{ID: ID(), Owner: p.dataOwner(), SpaceID: op.SpaceID, Path: next})
	}
	s.mu.Lock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, op.SpaceID); e != nil {
			return e
		}
		for _, d := range toCreate {
			var existing Directory
			if tx.Where("space_id = ? AND path = ?", op.SpaceID, d.Path).First(&existing).Error == nil {
				continue
			}
			if e := tx.Create(&d).Error; e != nil {
				return code("name_conflict")
			}
		}
		return nil
	})
	s.mu.Unlock()
	if err != nil {
		s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": safeCode(err), "updated": time.Now().UTC()})
		return op, err
	}
	done := 0
	for _, f := range files {
		sub := op.Key + "/" + f.ID
		if len(sub) > 200 {
			sub = Hash(op.Key + ":" + f.ID)
		}
		child, e := s.CopyFile(ctx, p, f.ID, CopyRequest{
			TargetSpaceID:   op.SpaceID,
			TargetPath:      remapPrefix(f.Path, src, dest),
			ExpectedVersion: f.VersionID,
			Conflict:        "fail",
		}, sub)
		if e != nil {
			s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": safeCode(e), "progress_files": done, "updated": time.Now().UTC()})
			op.Status, op.Error, op.ProgressFiles = "failed", safeCode(e), done
			return op, e
		}
		if child.Status != "completed" && child.Status != "" {
			s.DB.Model(&op).Updates(map[string]any{"status": "running", "progress_files": done, "updated": time.Now().UTC()})
		}
		done++
		s.DB.Model(&op).Updates(map[string]any{"progress_files": done, "updated": time.Now().UTC()})
	}
	now := time.Now().UTC()
	s.DB.Model(&op).Updates(map[string]any{"status": "completed", "progress_files": done, "error": "", "updated": now})
	op.Status, op.ProgressFiles, op.Error, op.Updated = "completed", done, "", now
	return op, nil
}

func (s *Service) MoveDirectory(p Principal, q DirectoryMoveRequest) (map[string]any, error) {
	src, err := cleanPath(q.Path)
	if err != nil {
		return nil, err
	}
	destReq, err := cleanPath(q.TargetPath)
	if err != nil {
		return nil, err
	}
	conflict, err := directoryConflict(q.Conflict)
	if err != nil {
		return nil, err
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return nil, e
	}
	if e := s.allow(p, q.SpaceID, src, "write"); e != nil {
		return nil, e
	}
	if insidePath(destReq, src) {
		return nil, code("invalid_directory")
	}
	files, dirs, err := s.listPrefix(p, q.SpaceID, src)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if e := s.allow(p, q.SpaceID, f.Path, "write"); e != nil {
			return nil, e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var dest string
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, q.SpaceID); e != nil {
			return e
		}
		dest = destReq
		if conflict == "rename" {
			var e error
			dest, e = uniqueDirPath(tx, p.dataOwner(), q.SpaceID, destReq)
			if e != nil {
				return e
			}
		} else if e := pathAvailable(tx, p.dataOwner(), q.SpaceID, destReq, ""); e != nil {
			return e
		}
		if e := s.allow(p, q.SpaceID, dest, "write"); e != nil {
			return e
		}
		byDepth := func(i, j int, pathAt func(int) string) bool {
			return strings.Count(pathAt(i), "/") > strings.Count(pathAt(j), "/")
		}
		for i := 0; i < len(files); i++ {
			for j := i + 1; j < len(files); j++ {
				if byDepth(j, i, func(n int) string { return files[n].Path }) {
					files[i], files[j] = files[j], files[i]
				}
			}
		}
		for i := range files {
			next := remapPrefix(files[i].Path, src, dest)
			if e := tx.Model(&File{}).Where("id = ?", files[i].ID).Updates(map[string]any{"path": next, "updated": time.Now().UTC()}).Error; e != nil {
				return code("name_conflict")
			}
		}
		for i := 0; i < len(dirs); i++ {
			for j := i + 1; j < len(dirs); j++ {
				if byDepth(j, i, func(n int) string { return dirs[n].Path }) {
					dirs[i], dirs[j] = dirs[j], dirs[i]
				}
			}
		}
		for i := range dirs {
			next := remapPrefix(dirs[i].Path, src, dest)
			if e := tx.Model(&Directory{}).Where("id = ?", dirs[i].ID).Update("path", next).Error; e != nil {
				return code("name_conflict")
			}
		}
		if len(dirs) == 0 {
			d := Directory{ID: ID(), Owner: p.dataOwner(), SpaceID: q.SpaceID, Path: dest}
			if e := tx.Create(&d).Error; e != nil {
				return code("name_conflict")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": dest, "moved": len(files), "directories": len(dirs)}, nil
}

func (s *Service) TrashDirectory(p Principal, q DirectoryTrashRequest) (map[string]any, error) {
	src, err := cleanPath(q.Path)
	if err != nil {
		return nil, err
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return nil, e
	}
	if e := s.allow(p, q.SpaceID, src, "delete"); e != nil {
		return nil, e
	}
	files, dirs, err := s.listPrefix(p, q.SpaceID, src)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if e := s.Trash(p, f.ID, false); e != nil {
			return nil, e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, q.SpaceID); e != nil {
			return e
		}
		ids := make([]string, len(dirs))
		for i, d := range dirs {
			ids[i] = d.ID
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Where("id IN ?", ids).Delete(&Directory{}).Error
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": src, "trashed": len(files), "directories": len(dirs)}, nil
}
