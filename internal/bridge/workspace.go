package bridge

import (
	"context"
	"errors"
	"path"
	"strings"
	"time"
)

const trashRoot = ".drivebridge-trash"

// emit appends one changes-feed event; failures never break the caller.
func (s *Service) emit(space, kind, ref, detail string) {
	_ = s.DB.Create(&Event{SpaceID: space, Kind: kind, Ref: ref, Detail: detail, At: time.Now().UTC()}).Error
}

// Search returns matching references from the local index. Hits are not
// remotely verified: consume them through stat (stale-index honesty).
func (s *Service) Search(space, query, dir string, options ...SearchOptions) ([]File, error) {
	if !s.Enabled {
		return nil, errors.New("disabled")
	}
	if _, err := s.Space(space); err != nil {
		return nil, err
	}
	if dir, err := remoteDir(dir); err != nil {
		return nil, err
	} else if dir != "" && query == "" {
		return nil, errors.New("invalid_search")
	}
	var files []File
	needle := strings.ToLower(query)
	tx := s.DB.Where("space_id = ? AND trashed_at IS NULL", space)
	if dir != "" {
		tx = tx.Where("dir = ?", dir)
	}
	if len(options) > 0 {
		if options[0].MIME != "" {
			tx = tx.Where("mime = ?", options[0].MIME)
		}
		if !options[0].After.IsZero() {
			tx = tx.Where("observed_at >= ?", options[0].After)
		}
	}
	if needle != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+needle+"%")
	}
	if err := tx.Order("name").Find(&files).Error; err != nil {
		return nil, errors.New("state_unavailable")
	}
	return files, nil
}

// Changes reports feed events strictly after the cursor plus the new cursor.
// A zero cursor returns the current position without replaying history.
func (s *Service) Changes(space string, since uint64) ([]Event, uint64, error) {
	if !s.Enabled {
		return nil, 0, errors.New("disabled")
	}
	if _, err := s.Space(space); err != nil {
		return nil, 0, err
	}
	var head struct{ Max uint64 }
	if err := s.DB.Model(&Event{}).Select("COALESCE(MAX(id), 0) AS max").Where("space_id = ?", space).Scan(&head).Error; err != nil {
		return nil, 0, errors.New("state_unavailable")
	}
	if since == 0 {
		return []Event{}, head.Max, nil
	}
	if since > head.Max {
		return nil, 0, errors.New("cursor_ahead")
	}
	var events []Event
	if err := s.DB.Where("space_id = ? AND id > ?", space, since).Order("id").Find(&events).Error; err != nil {
		return nil, 0, errors.New("state_unavailable")
	}
	return events, head.Max, nil
}

// Trash soft-deletes a reference inside the space: the object moves to a
// dated trash directory, the reference stays queryable, quota stays occupied.
func (s *Service) Trash(ctx context.Context, ref string) (result File, resultErr error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	var f File
	if s.DB.First(&f, "ref = ?", ref).Error != nil {
		return File{}, errors.New("reference_not_found")
	}
	if f.Trashed != nil {
		return f, nil
	}
	finishAudit, auditErr := s.beginFileAudit(f.SpaceID, "files.trash", ref)
	if auditErr != nil {
		return File{}, auditErr
	}
	defer func() { finishAudit(resultErr) }()
	defer s.CaptureUsage()
	p, err := s.Space(f.SpaceID)
	if err != nil {
		return File{}, err
	}
	trashDir := trashRoot + "/" + time.Now().UTC().Format("20060102")
	name := f.Name
	for n := 0; n < 8; n++ {
		if err = s.providerFor(p).Move(ctx, p, f, trashDir, name); err == nil {
			break
		}
		if err.Error() != "target_conflict" {
			return File{}, err
		}
		// Never overwrite: suffix and retry, mirroring restore semantics.
		ext := path.Ext(f.Name)
		name = name[:len(name)-len(ext)] + "-" + newID()[:8] + ext
	}
	if err != nil {
		return File{}, err
	}
	now := time.Now().UTC()
	f.Trashed, f.TrashPath = &now, trashDir+"/"+name
	if err = s.DB.Save(&f).Error; err != nil {
		return File{}, errors.New("state_unavailable")
	}
	s.emit(f.SpaceID, "file_trashed", f.Ref, trashDir+"/"+name)
	return f, nil
}

// Restore moves a trashed file back to its original directory; a same-name
// conflict resolves with a suffix, never an overwrite.
func (s *Service) Restore(ctx context.Context, ref string) (result File, resultErr error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	var f File
	if s.DB.First(&f, "ref = ?", ref).Error != nil {
		return File{}, errors.New("reference_not_found")
	}
	if f.Trashed == nil {
		return f, nil
	}
	finishAudit, auditErr := s.beginFileAudit(f.SpaceID, "files.restore", ref)
	if auditErr != nil {
		return File{}, auditErr
	}
	defer func() { finishAudit(resultErr) }()
	defer s.CaptureUsage()
	p, err := s.Space(f.SpaceID)
	if err != nil {
		return File{}, err
	}
	name := f.Name
	for n := 0; n < 8; n++ {
		if err = s.providerFor(p).Move(ctx, p, f, f.Dir, name); err == nil {
			break
		}
		if err.Error() != "target_conflict" {
			return File{}, err
		}
		ext := path.Ext(f.Name)
		name = name[:len(name)-len(ext)] + "-" + newID()[:8] + ext
	}
	if err != nil {
		return File{}, err
	}
	f.Trashed, f.TrashPath = nil, ""
	if err = s.DB.Save(&f).Error; err != nil {
		return File{}, errors.New("state_unavailable")
	}
	s.emit(f.SpaceID, "file_restored", f.Ref, f.Name)
	return f, nil
}

// Purge permanently deletes. It is deliberately NOT exposed as an MCP tool:
// permanent deletion stays with the human owner (CLI flag / web trash).
func (s *Service) Purge(ctx context.Context, ref string) (resultErr error) {
	if !s.Enabled {
		return errors.New("disabled")
	}
	var f File
	if s.DB.First(&f, "ref = ?", ref).Error != nil {
		return errors.New("reference_not_found")
	}
	finishAudit, auditErr := s.beginFileAudit(f.SpaceID, "files.purge", ref)
	if auditErr != nil {
		return auditErr
	}
	defer func() { finishAudit(resultErr) }()
	defer s.CaptureUsage()
	p, err := s.Space(f.SpaceID)
	if err != nil {
		return err
	}
	if f.Trashed == nil {
		// Purge only touches already-trashed files: a two-step contract.
		return errors.New("file_not_trashed")
	}
	if err = s.providerFor(p).Delete(ctx, p, f); err != nil {
		return err
	}
	if err = s.DB.Delete(&f).Error; err != nil {
		return errors.New("state_unavailable")
	}
	s.emit(f.SpaceID, "file_purged", ref, f.Name)
	return nil
}
