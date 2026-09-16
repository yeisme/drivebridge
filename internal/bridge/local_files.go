package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type LocalFileAction struct {
	Action    string `json:"action"`
	Space     string `json:"space_id"`
	Ref       string `json:"ref"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Key       string `json:"idempotency_key"`
	Operation string `json:"operation_id"`
}

func (s *Service) RunLocalFileAction(ctx context.Context, q LocalFileAction) (any, error) {
	if !s.Enabled {
		return nil, errors.New("disabled")
	}
	switch q.Action {
	case "mkdir":
		return map[string]bool{"created": true}, s.Mkdir(ctx, q.Space, q.Path)
	case "trash":
		return s.Trash(ctx, q.Ref)
	case "restore":
		return s.Restore(ctx, q.Ref)
	case "upload_path":
		return s.Upload(ctx, q.Space, "", q.Path, q.Name, q.Key)
	case "resume":
		return s.Resume(q.Operation)
	case "cancel":
		return s.Cancel(q.Operation)
	case "operations":
		if _, err := s.Space(q.Space); err != nil {
			return nil, err
		}
		out := []Operation{}
		err := s.DB.Where("space_id = ?", q.Space).Order("updated DESC").Limit(200).Find(&out).Error
		return out, err
	case "move":
		dir, err := remoteDir(q.Path)
		if err != nil {
			return nil, err
		}
		if !safeName(q.Name) {
			return nil, errors.New("invalid_name")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		f, err := s.Stat(ctx, q.Ref)
		if err != nil {
			return nil, err
		}
		if f.Trashed != nil {
			return nil, errors.New("file_trashed")
		}
		p, err := s.Space(f.SpaceID)
		if err != nil {
			return nil, err
		}
		finish, err := s.beginFileAudit(f.SpaceID, "files.move", f.Ref)
		if err != nil {
			return nil, err
		}
		err = s.providerFor(p).Move(ctx, p, f, dir, q.Name)
		finish(err)
		if err != nil {
			return nil, err
		}
		if p.Kind() != "onedrive" {
			f.ItemID = path.Join(dir, q.Name)
		}
		f.Name, f.Dir = q.Name, dir
		if s.DB.Save(&f).Error != nil {
			return nil, errors.New("state_unavailable")
		}
		s.emit(f.SpaceID, "file_moved", f.Ref, q.Name)
		return f, nil
	}
	return nil, errors.New("unsupported")
}
func (s *Service) localFileRoutes(mux *http.ServeMux) {
	fail := func(w http.ResponseWriter, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": localFileError(err)}})
	}
	mux.HandleFunc("POST /files/action", func(w http.ResponseWriter, r *http.Request) {
		var q LocalFileAction
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil {
			fail(w, errors.New("invalid_action"))
			return
		}
		out, err := s.RunLocalFileAction(r.Context(), q)
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": out})
	})
	mux.HandleFunc("POST /files/content", func(w http.ResponseWriter, r *http.Request) {
		var q LocalFileAction
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil {
			fail(w, errors.New("invalid_action"))
			return
		}
		f, err := s.Stat(r.Context(), q.Ref)
		if err != nil {
			fail(w, err)
			return
		}
		if f.Trashed != nil {
			fail(w, errors.New("file_trashed"))
			return
		}
		p, err := s.Space(f.SpaceID)
		if err != nil {
			fail(w, err)
			return
		}
		contentType := f.MIME
		if contentType == "" {
			contentType = mime.TypeByExtension(path.Ext(f.Name))
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		disposition := "inline"
		if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") && !strings.HasPrefix(contentType, "audio/") && !strings.HasPrefix(contentType, "text/") && contentType != "application/pdf" {
			disposition = "attachment"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		seeker := &localSeeker{size: f.Size, open: func() (io.ReadCloser, error) { return s.providerFor(p).Open(r.Context(), p, f) }}
		defer seeker.close()
		request := r.Clone(r.Context())
		request.Method = "GET"
		http.ServeContent(w, request, f.Name, time.Time{}, seeker)
	})
	mux.HandleFunc("PUT /files/upload", func(w http.ResponseWriter, r *http.Request) {
		if !s.Enabled {
			fail(w, errors.New("disabled"))
			return
		}
		p, err := s.Space(r.URL.Query().Get("space_id"))
		if err != nil {
			fail(w, err)
			return
		}
		name := r.URL.Query().Get("name")
		dir, err := remoteDir(r.URL.Query().Get("dir"))
		if err != nil || !safeName(name) {
			fail(w, errors.New("invalid_name"))
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" || len(key) > 200 {
			fail(w, errors.New("idempotency_key_required"))
			return
		}
		if r.ContentLength <= 0 || r.ContentLength > p.MaxBytes {
			fail(w, errors.New("file_size_invalid"))
			return
		}
		root, err := os.OpenRoot(p.LocalRoot)
		if err != nil {
			fail(w, errors.New("local_root_unavailable"))
			return
		}
		defer root.Close()
		if err = root.MkdirAll(".drivebridge-staging", 0700); err != nil {
			fail(w, errors.New("local_write_failed"))
			return
		}
		staged := ".drivebridge-staging/" + newID()
		out, err := root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			fail(w, errors.New("local_write_failed"))
			return
		}
		n, err := io.Copy(out, io.LimitReader(r.Body, p.MaxBytes+1))
		closeErr := out.Close()
		if err != nil || closeErr != nil || n != r.ContentLength {
			root.Remove(staged)
			fail(w, errors.New("upload_interrupted"))
			return
		}
		op, err := s.Upload(r.Context(), p.ID, dir, staged, name, key)
		if err != nil {
			root.Remove(staged)
			fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": op})
	})
}
func localFileError(err error) string {
	if err == nil {
		return ""
	}
	v := err.Error()
	if len(v) > 80 {
		return "local_file_failed"
	}
	for _, r := range v {
		if r != '_' && (r < 'a' || r > 'z') {
			return "local_file_failed"
		}
	}
	return v
}

// The reader reopens only after a seek; ordinary playback stays streaming.
type localSeeker struct {
	size, position int64
	body           io.ReadCloser
	open           func() (io.ReadCloser, error)
}

func (s *localSeeker) close() {
	if s.body != nil {
		s.body.Close()
		s.body = nil
	}
}
func (s *localSeeker) Read(p []byte) (int, error) {
	if s.body == nil {
		body, err := s.open()
		if err != nil {
			return 0, err
		}
		s.body = body
		if s.position > 0 {
			if _, err = io.CopyN(io.Discard, body, s.position); err != nil {
				s.close()
				return 0, err
			}
		}
	}
	n, err := s.body.Read(p)
	s.position += int64(n)
	return n, err
}
func (s *localSeeker) Seek(offset int64, whence int) (int64, error) {
	target := offset
	switch whence {
	case io.SeekCurrent:
		target += s.position
	case io.SeekEnd:
		target += s.size
	case io.SeekStart:
	default:
		return 0, errors.New("invalid_seek")
	}
	if target < 0 {
		return 0, errors.New("invalid_seek")
	}
	if target != s.position {
		s.close()
	}
	s.position = target
	return target, nil
}
