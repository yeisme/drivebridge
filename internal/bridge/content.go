package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// remoteCopier is the optional provider capability behind content-addressed
// reuse: duplicating an already-stored object under a new name without moving
// bytes through this process.
type remoteCopier interface {
	Copy(ctx context.Context, p Space, src File, target string) (File, error)
}

// ReuseUpload is the content-addressed upload entry. When the local file's
// SHA-256 already matches a live reference in the same space, the remote
// object is duplicated under the operation-prefixed name and the operation
// completes with zero transferred bytes; otherwise it falls back to the
// ordinary byte-transfer upload.
func (s *Service) ReuseUpload(ctx context.Context, space, dir, pathName, name, key string) (Operation, error) {
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
	rel, err := localPath(p, pathName)
	if err != nil {
		return Operation{}, err
	}
	root, err := os.OpenRoot(p.LocalRoot)
	if err != nil {
		return Operation{}, errors.New("local_root_unavailable")
	}
	defer root.Close()
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
	copier, canCopy := s.providerFor(p).(remoteCopier)
	var src File
	if canCopy {
		if e := s.DB.Where("space_id = ? AND sha256 = ? AND size = ? AND trashed_at IS NULL AND item_id != ''", space, hash, n).Order("observed_at DESC").First(&src).Error; e != nil {
			canCopy = false
		}
	}
	id := newID()
	target := id + "-" + name
	if dir != "" {
		target = dir + "/" + target
	}
	fingerprint := hash + ":" + name
	opKey := operationKey(space, "upload", key)
	if canCopy {
		s.mu.Lock()
		var old Operation
		if s.DB.First(&old, "key = ?", opKey).Error == nil {
			s.mu.Unlock()
			if old.Fingerprint != fingerprint || path.Dir(old.Target) != path.Dir(target) || old.LocalPath != rel {
				return old, errors.New("idempotency_conflict")
			}
			return old, nil
		}
		s.mu.Unlock()
		remote, err := copier.Copy(ctx, p, src, target)
		if err == nil {
			// Reusing the stored object is reference activity for the source.
			s.touchRetention(space, src.Ref)
		}
		if err == nil && (remote.ItemID == "" || remote.Size != n) {
			err = errors.New("content_mismatch")
		}
		var stored File
		if err == nil {
			remote.SHA256 = hash
			remote.Dir = ""
			if d := path.Dir(target); d != "." && d != "/" {
				remote.Dir = d
			}
			stored, err = s.remember(p, remote)
			if err == nil {
				s.touchRetention(space, stored.Ref)
			}
		}
		if err == nil {
			op := Operation{ID: id, SpaceID: space, Kind: "upload", Key: opKey, Status: "completed", LocalPath: rel, Target: target, Fingerprint: fingerprint, Bytes: n, Transferred: 0, FileRef: stored.Ref, Updated: time.Now().UTC()}
			s.mu.Lock()
			if e := s.DB.Create(&op).Error; e != nil {
				s.mu.Unlock()
				// A concurrent identical request won the unique key; its
				// receipt is the answer, the duplicate copy stays unused.
				var twin Operation
				if s.DB.First(&twin, "key = ?", opKey).Error == nil {
					return twin, nil
				}
				return op, errors.New("state_unavailable")
			}
			s.mu.Unlock()
			s.emit(op.SpaceID, "operation_completed", op.FileRef, op.ID)
			return op, nil
		}
		// Server-side copy is an optimization: when the backend cannot copy
		// or the indexed object is no longer remotely present, an ordinary
		// byte transfer is the honest fallback. Any other error (auth,
		// throttle, quota) would equally fail the transfer, so it surfaces.
		if errors.Is(err, fs.ErrorCantCopy) || err.Error() == "remote_not_found" {
			return s.Upload(ctx, space, dir, pathName, name, key)
		}
		return Operation{}, err
	}
	// No reusable object (or no server-side copy capability): an honest
	// byte transfer is the fallback, never a fabricated reference.
	return s.Upload(ctx, space, dir, pathName, name, key)
}

// LinkRef registers an explicit reference to an existing remote object in a
// (different) space. No bytes are copied and the remote object is never
// modified: the target space gains a queryable reference that resolves to the
// source object's identity.
func (s *Service) LinkRef(ctx context.Context, sourceRef, targetSpace, dir, name string) (File, error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	// The source is remotely verified first: a stale index row must not be
	// linked into another space.
	src, err := s.Stat(ctx, sourceRef)
	if err != nil {
		return File{}, err
	}
	if src.Trashed != nil {
		return File{}, errors.New("file_trashed")
	}
	if name == "" {
		name = src.Name
	}
	if !safeName(name) {
		return File{}, errors.New("invalid_upload")
	}
	p, err := s.Space(targetSpace)
	if err != nil {
		return File{}, err
	}
	if dir, err = remoteDir(dir); err != nil {
		return File{}, err
	}
	if src.DriveID != p.DriveID {
		// A reference must stay reachable through the target space's provider
		// scope; cross-drive links are refused instead of degrading later.
		return File{}, errors.New("remote_outside_space")
	}
	linked := File{SpaceID: p.ID, DriveID: src.DriveID, ItemID: src.ItemID, Version: src.Version, Name: name, Dir: dir, Size: src.Size, SHA256: src.SHA256, MIME: src.MIME, Duration: src.Duration}
	f, err := s.remember(p, linked)
	if err != nil {
		return File{}, err
	}
	s.touchRetention(p.ID, f.Ref)
	return f, nil
}

// Copy duplicates the item through the Graph copy action. OneDrive performs
// the copy server-side; the returned monitor is polled until the new item
// exists, then the new item is verified like any other space member.
func (r *OneDrive) Copy(ctx context.Context, p Space, src File, target string) (File, error) {
	dir, name := path.Split(target)
	if !safeName(name) {
		return File{}, errors.New("path_not_allowed")
	}
	folderID, err := r.folder(ctx, p, strings.TrimSuffix(dir, "/"))
	if err != nil {
		return File{}, err
	}
	getToken := r.token
	if r.authToken != nil {
		getToken = r.authToken
	}
	token, err := getToken(ctx, p)
	if err != nil {
		return File{}, err
	}
	body := `{"name":` + strconv.Quote(name) + `,"parentReference":{"id":` + strconv.Quote(folderID) + `}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.microsoft.com/v1.0/drives/"+url.PathEscape(p.DriveID)+"/items/"+url.PathEscape(src.ItemID)+"/copy", strings.NewReader(body))
	if err != nil {
		return File{}, errors.New("invalid_graph_request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := r.HTTP.Do(req)
	if err != nil {
		return File{}, errors.New("graph_unreachable")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200, 201:
	case 202:
	default:
		return File{}, graphStatusError(response.StatusCode)
	}
	var created driveItem
	if response.StatusCode != 202 {
		if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&created) != nil {
			return File{}, errors.New("invalid_graph_response")
		}
		return r.Stat(ctx, p, created.ID)
	}
	monitor := response.Header.Get("Location")
	// The monitor URL must stay on the Graph host; anything else is refused
	// rather than followed.
	u, e := url.Parse(monitor)
	if e != nil || u.Scheme != "https" || u.Host != "graph.microsoft.com" || u.User != nil {
		return File{}, errors.New("graph_unavailable")
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if err = ctx.Err(); err != nil {
			return File{}, errors.New("operation_cancelled")
		}
		if time.Now().After(deadline) {
			return File{}, errors.New("graph_unavailable")
		}
		poll, e := http.NewRequestWithContext(ctx, http.MethodGet, monitor, nil)
		if e != nil {
			return File{}, errors.New("invalid_graph_request")
		}
		poll.Header.Set("Authorization", "Bearer "+token)
		resp, e := r.HTTP.Do(poll)
		if e != nil {
			return File{}, errors.New("graph_unreachable")
		}
		switch resp.StatusCode {
		case 200:
			e2 := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&created)
			resp.Body.Close()
			if e2 != nil || created.ID == "" {
				return File{}, errors.New("invalid_graph_response")
			}
			return r.Stat(ctx, p, created.ID)
		case 202:
			resp.Body.Close()
		default:
			resp.Body.Close()
			return File{}, graphStatusError(resp.StatusCode)
		}
		select {
		case <-ctx.Done():
			return File{}, errors.New("operation_cancelled")
		case <-time.After(time.Second):
		}
	}
}

// graphStatusError maps a Graph HTTP status to the fixed code set.
func graphStatusError(status int) error {
	switch status {
	case 401, 403:
		return errors.New("authentication_required")
	case 404:
		return errors.New("remote_not_found")
	case 409:
		return errors.New("target_conflict")
	case 429:
		return errors.New("throttled")
	case 507:
		return errors.New("quota_exceeded")
	}
	return errors.New("graph_unavailable")
}

// Copy uses the backend's advertised server-side copy. Backends without that
// capability report backend_unavailable and the caller falls back to a full
// transfer instead of pretending the object was reused.
func (g *Generic) Copy(ctx context.Context, p Space, src File, target string) (File, error) {
	dir, name := path.Split(target)
	if _, err := remoteDir(strings.TrimSuffix(dir, "/")); err != nil {
		return File{}, err
	}
	if !safeName(name) {
		return File{}, errors.New("path_not_allowed")
	}
	remote, err := g.remote(p, ctx)
	if err != nil {
		return File{}, err
	}
	source, err := remote.NewObject(ctx, g.sourcePath(src))
	if err != nil {
		return File{}, errors.New("remote_not_found")
	}
	serverCopy := remote.Features().Copy
	if serverCopy == nil {
		// The backend does not advertise a server-side copy; the caller
		// decides between a byte transfer and refusing the reuse.
		return File{}, fs.ErrorCantCopy
	}
	duplicated, err := serverCopy(ctx, source, target)
	if err != nil {
		if errors.Is(err, fs.ErrorCantCopy) {
			return File{}, fs.ErrorCantCopy
		}
		return File{}, errors.New("backend_unavailable")
	}
	return fileFromObject(ctx, p, duplicated), nil
}
