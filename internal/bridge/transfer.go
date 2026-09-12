package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (s *Service) Upload(ctx context.Context, space, dir, path, name, key string) (Operation, error) {
	if !s.Enabled {
		return Operation{}, errors.New("disabled")
	}
	if !safeName(name) || key == "" {
		return Operation{}, errors.New("invalid_upload")
	}
	p, err := s.Space(space)
	if err != nil {
		return Operation{}, err
	}
	if dir, err = remoteDir(dir); err != nil {
		return Operation{}, err
	}
	rel, err := localPath(p, path)
	if err != nil {
		return Operation{}, err
	}
	root, err := os.OpenRoot(p.LocalRoot)
	if err != nil {
		return Operation{}, errors.New("local_root_unavailable")
	}
	defer root.Close()
	// Symbolic links are rejected by default to prevent escapes; explicit
	// opt-in materializes the target content (os.Root still blocks any link
	// that would resolve outside the allowed root).
	if info, e := root.Lstat(rel); e == nil && info.Mode()&os.ModeSymlink != 0 && !p.FollowSymlinks {
		return Operation{}, errors.New("symlink_rejected")
	}
	f, err := root.Open(rel)
	if err != nil {
		return Operation{}, errors.New("local_file_unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > p.MaxBytes {
		return Operation{}, errors.New("file_size_invalid")
	}
	hash, n, err := digestReader(io.LimitReader(f, p.MaxBytes+1))
	if err != nil || n != info.Size() {
		return Operation{}, errors.New("source_changed")
	}
	fingerprint := hash + ":" + name
	id := newID()
	target := id + "-" + name
	if dir != "" {
		target = dir + "/" + target
	}
	op := Operation{ID: id, SpaceID: space, Kind: "upload", Key: operationKey(space, "upload", key), Status: "queued", LocalPath: rel, Target: target, Fingerprint: fingerprint, Bytes: n}
	return s.enqueue(op)
}
func operationKey(space, kind, key string) string {
	h := sha256.Sum256([]byte(space + "\x00" + kind + "\x00" + key))
	return hex.EncodeToString(h[:])
}
func (s *Service) Download(ctx context.Context, ref, path, key string) (Operation, error) {
	if key == "" {
		return Operation{}, errors.New("idempotency_key_required")
	}
	f, err := s.Stat(ctx, ref)
	if err != nil {
		return Operation{}, err
	}
	if f.Trashed != nil {
		return Operation{}, errors.New("file_trashed")
	}
	p, err := s.Space(f.SpaceID)
	if err != nil {
		return Operation{}, err
	}
	rel, err := localPath(p, path)
	if err != nil {
		return Operation{}, err
	}
	return s.enqueue(Operation{ID: newID(), SpaceID: p.ID, Kind: "download", Key: operationKey(p.ID, "download", key), Status: "queued", LocalPath: rel, FileRef: ref, Bytes: f.Size, Fingerprint: ref + ":" + rel})
}
func (s *Service) enqueue(op Operation) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var old Operation
	if s.DB.First(&old, "key = ?", op.Key).Error == nil {
		if old.Fingerprint != op.Fingerprint || (op.Kind != "save_output" && old.LocalPath != op.LocalPath) {
			return old, errors.New("idempotency_conflict")
		}
		return old, nil
	}
	if len(s.jobs) >= 2 {
		return op, errors.New("transfer_capacity_unavailable")
	}
	op.Updated = time.Now().UTC()
	if s.DB.Create(&op).Error != nil {
		return op, errors.New("state_unavailable")
	}
	s.start(op)
	return op, nil
}
func (s *Service) start(op Operation) {
	ctx, cancel := context.WithCancel(context.Background())
	s.jobs[op.ID] = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.run(ctx, op)
		s.mu.Lock()
		delete(s.jobs, op.ID)
		s.mu.Unlock()
	}()
}
func (s *Service) Resume(id string) (Operation, error) {
	if !s.Enabled {
		return Operation{}, errors.New("disabled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	op, err := s.Get(id)
	if err != nil {
		return op, err
	}
	if _, ok := s.jobs[id]; ok {
		return op, nil
	}
	if op.Status == "completed" {
		return op, nil
	}
	if op.Status == "cancelled" {
		return op, errors.New("operation_cancelled")
	}
	if len(s.jobs) >= 2 {
		return op, errors.New("transfer_capacity_unavailable")
	}
	op.Status = "queued"
	op.Error = ""
	if s.DB.Save(&op).Error != nil {
		return op, errors.New("state_unavailable")
	}
	s.start(op)
	return op, nil
}
func (s *Service) Cancel(id string) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, err := s.Get(id)
	if err != nil {
		return op, err
	}
	if op.Status == "completed" {
		return op, nil
	}
	if cancel, ok := s.jobs[id]; ok {
		cancel()
		op.Status = "cancelling"
		s.DB.Model(&Operation{}).Where("id = ?", id).Update("status", "cancelling")
		return op, nil
	}
	op.Status = "cancelled"
	err = s.DB.Save(&op).Error
	return op, err
}
func (s *Service) run(ctx context.Context, op Operation) {
	s.mu.Lock()
	if ctx.Err() != nil {
		current, _ := s.Get(op.ID)
		op.Status = "interrupted"
		op.Error = "resume_required"
		if current.Status == "cancelling" {
			op.Status = "cancelled"
			op.Error = "operation_cancelled"
		}
		_ = s.DB.Save(&op).Error
		s.mu.Unlock()
		return
	}
	op.Status = "transferring"
	op.Transferred = 0
	op.Updated = time.Now().UTC()
	if s.DB.Save(&op).Error != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	var err error
	if op.Kind == "save_output" {
		err = s.saveOutput(ctx, &op)
	} else {
		err = s.transfer(ctx, &op)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	op.Updated = time.Now().UTC()
	if err != nil {
		op.Status = "failed"
		op.Error = safeError(err)
		if ctx.Err() != nil {
			op.Status = "interrupted"
			op.Error = "resume_required"
			if current, e := s.Get(op.ID); e == nil && current.Status == "cancelling" {
				op.Status = "cancelled"
				op.Error = "operation_cancelled"
			}
		}
	} else {
		op.Status = "completed"
		op.Error = ""
		op.Transferred = op.Bytes
	}
	_ = s.DB.Save(&op).Error
	s.emit(op.SpaceID, "operation_"+op.Status, op.FileRef, op.ID)
}
func safeError(err error) string {
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "file_trashed", "file_not_trashed", "cursor_ahead", "invalid_search", "transfer_capacity_unavailable", "disabled", "idempotency_conflict", "idempotency_key_required", "invalid_upload", "file_size_invalid", "local_file_unavailable", "local_root_unavailable", "state_unavailable", "operation_not_found", "operation_cancelled", "unknown_tool", "output_adapter_unavailable", "output_not_ready", "output_not_allowed", "output_grant_invalid", "scaena_unavailable", "graph_unavailable", "graph_unreachable", "download_unavailable", "download_access_unavailable", "media_probe_failed", "media_type_unsupported", "media_duration_invalid", "region_not_supported", "source_changed", "file_version_changed", "target_conflict", "space_not_allowed", "reference_not_found", "quota_exceeded", "throttled", "authentication_required", "path_not_allowed", "invalid_file_metadata", "remote_not_found", "remote_outside_space", "secure_handoff_unavailable", "handoff_not_available", "access_unsupported", "backend_unavailable", "symlink_rejected", "invalid_space", "space_conflict", "content_mismatch":
		return err.Error()
	}
	return "transfer_failed"
}

type progressReader struct {
	r    io.Reader
	s    *Service
	id   string
	n    int64
	last time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, e := p.r.Read(b)
	p.n += int64(n)
	if time.Since(p.last) > time.Second {
		p.last = time.Now()
		p.s.DB.Model(&Operation{}).Where("id = ?", p.id).Update("transferred", p.n)
	}
	return n, e
}
func (s *Service) transfer(ctx context.Context, op *Operation) error {
	p, err := s.Space(op.SpaceID)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(p.LocalRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if op.Kind == "download" {
		f, e := s.Stat(ctx, op.FileRef)
		if e != nil {
			return e
		}
		// An operation-owned partial file is retained across failures. Final publication
		// uses Link, which atomically refuses to replace an existing destination.
		temp := filepath.Join(filepath.Dir(op.LocalPath), ".drivebridge-"+op.ID+"-"+newID())
		out, e := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		in, e := s.providerFor(p).Open(ctx, p, f)
		if e != nil {
			out.Close()
			return e
		}
		h, n, e := digestReader(io.TeeReader(io.LimitReader(&progressReader{r: in, s: s, id: op.ID}, p.MaxBytes+1), out))
		in.Close()
		syncErr := out.Sync()
		out.Close()
		if e != nil || syncErr != nil || n != f.Size {
			return errors.New("content_mismatch")
		}
		if f.SHA256 != "" && h != f.SHA256 {
			return errors.New("content_mismatch")
		}
		if _, e = s.Stat(ctx, op.FileRef); e != nil {
			return e
		}
		if e = root.Link(temp, op.LocalPath); e != nil {
			existing, openErr := root.Open(op.LocalPath)
			if openErr != nil {
				return errors.New("target_conflict")
			}
			old, oldN, hashErr := digestReader(io.LimitReader(existing, p.MaxBytes+1))
			existing.Close()
			if hashErr != nil || old != h || oldN != n {
				return errors.New("target_conflict")
			}
		}
		_ = root.Remove(temp)
		return nil
	}
	source, e := root.Open(op.LocalPath)
	if e != nil {
		return e
	}
	defer source.Close()
	h, n, e := digestReader(io.LimitReader(source, p.MaxBytes+1))
	targetName := filepath.Base(op.Target)
	if len(targetName) < 34 {
		return errors.New("source_changed")
	}
	if e != nil || n != op.Bytes || op.Fingerprint != h+":"+targetName[33:] {
		return errors.New("source_changed")
	}
	if _, e = source.Seek(0, 0); e != nil {
		return e
	}
	// The operation-specific name prevents replacing a user-selected existing
	// file; the scan stays inside the operation's own directory.
	scanDir := ""
	if d := filepath.Dir(op.Target); d != "." {
		scanDir = d
	}
	items, e := s.providerFor(p).List(ctx, p, scanDir)
	if e != nil {
		return e
	}
	var remote File
	for _, candidate := range items {
		if candidate.Name == op.Target {
			remote = candidate
			break
		}
	}
	if remote.ItemID == "" {
		remote, e = s.providerFor(p).Upload(ctx, p, op.Target, &progressReader{r: source, s: s, id: op.ID}, n)
		if e != nil {
			return e
		}
	}
	in, e := s.providerFor(p).Open(ctx, p, remote)
	if e != nil {
		return e
	}
	remoteHash, remoteN, e := digestReader(io.LimitReader(in, p.MaxBytes+1))
	in.Close()
	if e != nil || remoteHash != h || remoteN != n {
		return errors.New("content_mismatch")
	}
	remote.SHA256 = h
	remote.Dir = scanDir
	f, e := s.remember(p, remote)
	if e != nil {
		return e
	}
	op.FileRef = f.Ref
	return nil
}
