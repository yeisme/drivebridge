package service

import (
	"context"
	"errors"
	"io"
	"time"

	"gorm.io/gorm"
)

// workspaceReadVersionOn resolves a fixed version without inheriting legacy
// read scope. File paths and grant state are reloaded on every invocation.
func (s *Service) workspaceReadVersionOn(tx *gorm.DB, p Principal, id, versionID string) (File, Version, error) {
	var file File
	var version Version
	if id == "" || versionID == "" {
		return file, version, code("invalid_request")
	}
	if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return File{}, version, code("not_found")
		}
		return File{}, version, code("database_unavailable")
	}
	if file.Trashed || file.PurgeStartedAt != nil {
		return File{}, version, code("not_found")
	}
	if err := tx.Where("id = ? AND owner = ? AND file_id = ?", versionID, p.UserID, id).First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return File{}, Version{}, code("not_found")
		}
		return File{}, Version{}, code("database_unavailable")
	}
	if _, err := s.authorizeWorkspaceActionOn(tx, p, file.SpaceID, version.ConnectionID, file.Path, "files.read", time.Now().UTC()); err != nil {
		return File{}, Version{}, err
	}
	return file, version, nil
}

func (s *Service) downloadWorkspaceVersion(ctx context.Context, p Principal, id, versionID string) (File, Version, io.ReadCloser, error) {
	var file File
	var version Version
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		file, version, err = s.workspaceReadVersionOn(tx, p, id, versionID)
		return err
	})
	if err != nil {
		return File{}, Version{}, nil, err
	}
	return s.downloadResolvedVersion(ctx, p, file, version, true)
}

// Recheck every bounded read, even when governance is not enabled. This keeps
// revocation, scope changes and file moves effective for an already-open body.
type workspaceVersionReader struct {
	io.ReadCloser
	service   *Service
	principal Principal
	fileID    string
	versionID string
}

func (r *workspaceVersionReader) Read(buf []byte) (int, error) {
	if _, _, err := r.service.workspaceReadVersionOn(r.service.DB, r.principal, r.fileID, r.versionID); err != nil {
		return 0, err
	}
	if len(buf) > 256<<10 {
		buf = buf[:256<<10]
	}
	return r.ReadCloser.Read(buf)
}
