package service

import (
	"bytes"
	"context"
	"gorm.io/gorm"
	"io"
	"strconv"
	"strings"
)

type ArchiveRequest struct {
	SourceFileID          string `json:"source_file_id"`
	SourceVersionID       string `json:"source_version_id"`
	TargetSpaceID         string `json:"target_space_id"`
	TargetPath            string `json:"target_path"`
	ExpectedSourceVersion string `json:"expected_source_version,omitempty"`
}

// ingestOrigin is a server-side copy of an existing version. archive excludes
// CAS reuse; copy allows it. AutoRename never overwrites a live path.
type ingestOrigin struct {
	Kind       string
	Source     ArchiveRequest
	AutoRename bool
	Workspace  *workspaceArchiveIntent `json:",omitempty"`
}

type CopyRequest struct {
	TargetSpaceID   string `json:"target_space_id"`
	TargetPath      string `json:"target_path"`
	ExpectedVersion string `json:"expected_version"`
	Conflict        string `json:"conflict,omitempty"`
}

// pathTaken reports whether a file or directory already occupies the exact
// path, or a directory sits at or beneath it: the same collision rules
// pathAvailable enforces, scoped space-wide like the file_path index.
func pathTaken(tx *gorm.DB, space, path string) (bool, error) {
	var f File
	if e := tx.Where("space_id = ? AND path = ?", space, path).First(&f).Error; e == nil {
		return true, nil
	}
	var dirs []Directory
	if e := tx.Where("space_id = ?", space).Find(&dirs).Error; e != nil {
		return false, code("database_unavailable")
	}
	for _, d := range dirs {
		if d.Path == path || strings.HasPrefix(d.Path, path+"/") {
			return true, nil
		}
	}
	return false, nil
}

func archivePath(tx *gorm.DB, space, requested string) (string, error) {
	path := requested
	for n := 2; ; n++ {
		taken, e := pathTaken(tx, space, path)
		if e != nil {
			return "", e
		}
		if !taken {
			return path, nil
		}
		dot := strings.LastIndex(requested, ".")
		if dot < 0 {
			dot = len(requested)
		}
		path = requested[:dot] + " (" + strconv.Itoa(n) + ")" + requested[dot:]
	}
}

// requireManagedTarget rejects destinations that cannot receive a managed
// copy before any bytes move or quota is reserved: the archive pipeline only
// writes through server-side S3-backed storage.
func (s *Service) requireManagedTarget(p Principal, spaceID string) error {
	var sp Space
	if s.DB.First(&sp, "id = ? AND owner = ?", spaceID, p.dataOwner()).Error != nil {
		return code("not_found")
	}
	var conn Connection
	if s.DB.First(&conn, "id = ? AND (owner = ? OR owner = ?) AND disabled = ?", sp.ConnectionID, p.dataOwner(), "", false).Error != nil || conn.Kind != "s3" {
		return code("unsupported")
	}
	return nil
}

func (s *Service) validateArchiveSource(p Principal, op Operation) error {
	f, e := s.file(p, op.ArchiveFileID, "read")
	if e != nil {
		return e
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", op.ArchiveVersionID, f.ID, p.dataOwner()).Error != nil {
		return code("not_found")
	}
	if op.ArchiveHeadVersion != "" && f.VersionID != op.ArchiveHeadVersion {
		return code("version_conflict")
	}
	return nil
}

func (s *Service) ArchiveVersion(ctx context.Context, p Principal, q ArchiveRequest, key string) (Operation, error) {
	return s.archiveVersion(ctx, p, q, key, true)
}

// archiveVersion preserves legacy rename behavior while allowing workspace
// archives to require an explicit destination conflict instead.
func (s *Service) archiveVersion(ctx context.Context, p Principal, q ArchiveRequest, key string, autoRename bool) (Operation, error) {
	if key == "" || len(key) > 200 || q.SourceFileID == "" || q.SourceVersionID == "" || q.TargetSpaceID == "" {
		return Operation{}, code("invalid_archive")
	}
	if e := s.requireManagedTarget(p, q.TargetSpaceID); e != nil {
		return Operation{}, e
	}
	src, e := s.file(p, q.SourceFileID, "read")
	if e != nil {
		return Operation{}, e
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", q.SourceVersionID, src.ID, p.dataOwner()).Error != nil {
		return Operation{}, code("not_found")
	}
	if q.ExpectedSourceVersion == "" {
		q.ExpectedSourceVersion = src.VersionID
	}
	if _, e = cleanPath(q.TargetPath); e != nil {
		return Operation{}, e
	}
	if e = s.allow(p, q.TargetSpaceID, q.TargetPath, "write"); e != nil {
		return Operation{}, e
	}
	op, e := s.createUpload(p, UploadRequest{Caller: src.Caller, TaskRef: src.TaskRef, SpaceID: q.TargetSpaceID, Path: q.TargetPath, Size: v.Size, SHA256: v.SHA256}, key, &ingestOrigin{Kind: "archive", Source: q, AutoRename: autoRename})
	if e != nil || op.Status == "completed" {
		return op, e
	}
	return s.streamExistingVersion(ctx, p, op, src.ID, v.ID)
}

func (s *Service) CopyFile(ctx context.Context, p Principal, sourceID string, q CopyRequest, key string) (Operation, error) {
	if key == "" || len(key) > 200 || sourceID == "" || q.TargetSpaceID == "" || q.ExpectedVersion == "" {
		return Operation{}, code("invalid_copy")
	}
	conflict := q.Conflict
	if conflict == "" {
		conflict = "rename"
	}
	if conflict != "rename" && conflict != "fail" {
		return Operation{}, code("invalid_copy")
	}
	if e := s.requireManagedTarget(p, q.TargetSpaceID); e != nil {
		return Operation{}, e
	}
	src, e := s.file(p, sourceID, "read")
	if e != nil {
		return Operation{}, e
	}
	if src.Trashed {
		return Operation{}, code("file_trashed")
	}
	if src.VersionID != q.ExpectedVersion {
		return Operation{}, code("version_conflict")
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", q.ExpectedVersion, src.ID, p.dataOwner()).Error != nil {
		return Operation{}, code("not_found")
	}
	if _, e = cleanPath(q.TargetPath); e != nil {
		return Operation{}, e
	}
	if e = s.allow(p, q.TargetSpaceID, q.TargetPath, "write"); e != nil {
		return Operation{}, e
	}
	var active int64
	if s.DB.Model(&Operation{}).Where("owner = ? AND file_id = ? AND status NOT IN ?", p.dataOwner(), src.ID, []string{"completed", "cancelled", "failed"}).Count(&active).Error != nil {
		return Operation{}, code("database_unavailable")
	}
	if active > 0 {
		return Operation{}, code("file_in_use")
	}
	origin := &ingestOrigin{
		Kind: "copy",
		Source: ArchiveRequest{
			SourceFileID:          src.ID,
			SourceVersionID:       v.ID,
			TargetSpaceID:         q.TargetSpaceID,
			TargetPath:            q.TargetPath,
			ExpectedSourceVersion: q.ExpectedVersion,
		},
		AutoRename: conflict == "rename",
	}
	op, e := s.createUpload(p, UploadRequest{Caller: src.Caller, TaskRef: src.TaskRef, SpaceID: q.TargetSpaceID, Path: q.TargetPath, Size: v.Size, SHA256: v.SHA256}, key, origin)
	if e != nil || op.Status == "completed" {
		return op, e
	}
	return s.streamExistingVersion(ctx, p, op, src.ID, v.ID)
}

func (s *Service) streamExistingVersion(ctx context.Context, p Principal, op Operation, fileID, versionID string) (Operation, error) {
	s.archiveMu.Lock()
	defer s.archiveMu.Unlock()
	_, v, source, e := s.Download(ctx, p, fileID, versionID)
	if e != nil {
		return op, e
	}
	defer source.Close()
	for i := int64(0); i*PartSize < v.Size; i++ {
		n := PartSize
		if remain := v.Size - i*PartSize; remain < n {
			n = remain
		}
		buf := make([]byte, n)
		if _, e = io.ReadFull(source, buf); e != nil {
			return op, e
		}
		if _, e = s.putPart(p, op.ID, int(i), bytes.NewReader(buf), true); e != nil {
			return op, e
		}
	}
	return s.Complete(ctx, p, op.ID)
}
