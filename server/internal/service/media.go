package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/yeisme/drivebridge-server/sdk/mediapreview"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Preview struct {
	Encrypted bool      `json:"-"`
	Size      int64     `json:"size_bytes"`
	ID        string    `json:"id" gorm:"primaryKey"`
	Owner     string    `json:"-" gorm:"index"`
	FileID    string    `json:"file_id"`
	VersionID string    `json:"version_id" gorm:"uniqueIndex:preview_variant"`
	Kind      string    `json:"kind" gorm:"uniqueIndex:preview_variant"`
	Status    string    `json:"status"`
	MIME      string    `json:"mime_type"`
	Error     string    `json:"error,omitempty"`
	Updated   time.Time `json:"updated_at"`
}

func (s *Service) mediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/media", s.wrap("media.read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		return s.mediaContent(w, r, p, r.PathValue("id"), r.URL.Query().Get("version"), false)
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/previews", s.wrap("previews.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		f, err := s.file(p, r.PathValue("id"), "read")
		if err != nil {
			return err
		}
		out := []Preview{}
		if s.DB.Where("owner = ? AND file_id = ? AND version_id = ?", p.dataOwner(), f.ID, r.URL.Query().Get("version")).Find(&out).Error != nil {
			return code("database_unavailable")
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/previews", s.wrap("previews.prepare", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Version string `json:"version_id"`
			Kind    string `json:"kind"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		out, err := s.preparePreview(p, r.PathValue("id"), q.Version, q.Kind)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/previews/{id}/content", s.wrap("previews.read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var preview Preview
		if s.DB.First(&preview, "id = ? AND owner = ?", r.PathValue("id"), p.dataOwner()).Error != nil {
			return code("not_found")
		}
		f, err := s.file(p, preview.FileID, "read")
		if err != nil {
			return err
		}
		if f.Trashed {
			return code("file_trashed")
		}
		return s.servePreview(w, r, preview)
	}))
}

// mediaContent preserves the attachment endpoint and adds a safe inline lane.
// Native object ranges avoid fetching the entire video before a seek can finish.
func (s *Service) mediaContent(w http.ResponseWriter, r *http.Request, p Principal, id, version string, attachment bool) error {
	f, err := s.file(p, id, "read")
	if err != nil {
		return err
	}
	if f.Trashed {
		return code("file_trashed")
	}
	if version == "" {
		return code("version_required")
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", version, id, p.dataOwner()).Error != nil {
		return code("not_found")
	}
	m := v.MIME
	if !attachment {
		m = spreadsheetPreviewMIME(m, f.Path)
	}
	interactive := r.URL.Query().Get("preview") == "interactive"
	if interactive && (attachment || !strings.HasPrefix(m, "text/html") || v.Size > 2*1024*1024) {
		return code("preview_unsupported")
	}
	if !attachment && !inlineMIME(m) {
		return code("preview_unsupported")
	}
	start, end, partial, err := byteRange(r.Header.Get("Range"), v.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", v.Size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	var body io.ReadCloser
	sp, err := s.space(p, f.SpaceID)
	if err != nil {
		return err
	}
	if partial && !sp.Encrypted {
		st, e := s.storageForSpace(p.dataOwner(), f.SpaceID, v.ConnectionID)
		if e != nil {
			return e
		}
		if ranged, ok := st.(interface {
			GetRange(context.Context, string, int64, int64) (io.ReadCloser, error)
		}); ok {
			body, err = ranged.GetRange(r.Context(), v.ObjectKey, start, end)
		}
	}
	if body == nil && err == nil {
		_, _, body, err = s.Download(r.Context(), p, id, version)
		if err == nil && start > 0 {
			_, err = io.CopyN(io.Discard, body, start)
		}
	}
	if err != nil {
		if body != nil {
			body.Close()
		}
		return err
	}
	defer body.Close()
	body = s.guardedStream(p, f, body)
	if v.Size == 0 {
		var probe [1]byte
		n, e := body.Read(probe[:])
		if n != 0 || e != io.EOF {
			return code("content_mismatch")
		}
	}
	w.Header().Set("Content-Type", m)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	// Sandboxed inline rendering: scripts and network subresources stay
	// blocked; data: images and fonts keep static chart HTML self-contained.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:")
	if interactive {
		// A separate document response avoids inheriting the application's script
		// policy. Keep an opaque sandbox origin even when opened outside an iframe.
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), clipboard-read=(), clipboard-write=()")
	}
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": path.Base(f.Path)}))
	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, v.Size))
		w.WriteHeader(206)
	}
	if r.Method == "HEAD" {
		return nil
	}
	if _, err = io.CopyN(w, body, end-start+1); err != nil {
		panic(http.ErrAbortHandler)
	}
	return nil
}

// Spreadsheet previews fetch original bytes for a local, non-executing reader.
// Refine only generic upload MIME types; never admit arbitrary ZIP/binary files.
func spreadsheetPreviewMIME(m, name string) string {
	base, _, _ := mime.ParseMediaType(m)
	switch strings.ToLower(path.Ext(name)) {
	case ".xlsx":
		if base == "application/zip" || base == "application/octet-stream" || base == "" {
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
	case ".csv":
		if base == "application/octet-stream" || base == "" {
			return "text/csv"
		}
	case ".tsv":
		if base == "application/octet-stream" || base == "" {
			return "text/tab-separated-values"
		}
	}
	return m
}

func inlineMIME(m string) bool {
	m, _, _ = mime.ParseMediaType(m)
	return strings.HasPrefix(m, "video/") || strings.HasPrefix(m, "audio/") || m == "image/png" || m == "image/jpeg" || m == "image/gif" || m == "image/webp" || m == "image/avif" || m == "image/bmp" || m == "image/svg+xml" || m == "image/x-icon" || m == "image/vnd.microsoft.icon" || m == "application/pdf" || m == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || m == "application/csv" || strings.HasPrefix(m, "text/") || m == "application/json" || m == "application/xml"
}

// detectMIME refines a sniffed content type with filename hints the sniffer
// cannot express: SVG text sniffs as text/xml, text/plain or application/xml.
func detectMIME(sniffed, name string) string {
	if !strings.EqualFold(path.Ext(name), ".svg") {
		return sniffed
	}
	for _, prefix := range []string{"text/xml", "text/plain", "application/xml"} {
		if strings.HasPrefix(sniffed, prefix) {
			return "image/svg+xml"
		}
	}
	return sniffed
}
func byteRange(value string, size int64) (start, end int64, partial bool, err error) {
	if size == 0 && value == "" {
		return 0, -1, false, nil
	}
	if size <= 0 {
		return 0, 0, false, code("invalid_range")
	}
	end = size - 1
	if value == "" {
		return
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		err = code("invalid_range")
		return
	}
	a, b, ok := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
	if !ok {
		err = code("invalid_range")
		return
	}
	partial = true
	if a == "" {
		var n int64
		n, err = strconv.ParseInt(b, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false, code("invalid_range")
		}
		start = max(0, size-n)
		return
	}
	start, err = strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, code("invalid_range")
	}
	if b != "" {
		end, err = strconv.ParseInt(b, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, code("invalid_range")
		}
		end = min(end, size-1)
	}
	return
}

func (s *Service) preparePreview(p Principal, id, version, kind string) (Preview, error) {
	f, err := s.file(p, id, "read")
	if err != nil {
		return Preview{}, err
	}
	if f.Trashed {
		return Preview{}, code("file_trashed")
	}
	if kind != "thumbnail" && kind != "compatible" {
		return Preview{}, code("invalid_preview")
	}
	var v Version
	if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", version, id, p.dataOwner()).Error != nil {
		return Preview{}, code("not_found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out Preview
	if s.DB.First(&out, "version_id = ? AND kind = ? AND owner = ?", version, kind, p.dataOwner()).Error == nil && (out.Status == "processing" || out.Status == "ready") {
		return out, nil
	}
	if out.ID == "" {
		out = Preview{ID: ID(), Owner: p.dataOwner(), FileID: id, VersionID: version, Kind: kind}
	}
	out.Status, out.Error, out.Updated = "processing", "", time.Now().UTC()
	if v.Size > 2<<30 {
		out.Status, out.Error = "unsupported", "preview_size_limit"
	}
	converter := mediapreview.Converter(f.Path)
	if _, err := exec.LookPath(converter); err != nil {
		out.Status, out.Error = "unsupported", "preview_converter_unavailable"
	}
	if s.DB.Save(&out).Error != nil {
		return out, code("database_unavailable")
	}
	if out.Status == "processing" {
		var queued int64
		if s.DB.Model(&Preview{}).Where("status = ?", "processing").Count(&queued).Error != nil || queued > 32 {
			s.DB.Model(&out).Updates(map[string]any{"status": "failed", "error": "preview_busy"})
			return out, code("preview_busy")
		}
		s.previewWG.Add(1)
		go func() { defer s.previewWG.Done(); s.convertPreview(p, f, v, out) }()
	}
	return out, nil
}

func (s *Service) convertPreview(p Principal, f File, v Version, out Preview) {
	ctx, cancel := context.WithTimeout(s.previewCtx, 5*time.Minute)
	defer cancel()
	status, failure := "failed", "preview_conversion_failed"
	defer func() {
		s.DB.Model(&out).Updates(map[string]any{"status": status, "error": failure, "mime": out.MIME, "encrypted": out.Encrypted, "size": out.Size, "updated": time.Now().UTC()})
	}()
	select {
	case s.previewSlots <- struct{}{}:
		defer func() { <-s.previewSlots }()
	case <-ctx.Done():
		return
	}
	cacheDir := filepath.Join(s.Config.State, "previews", out.ID)
	if os.MkdirAll(cacheDir, 0700) != nil {
		return
	}
	dir := cacheDir
	sp, err := s.space(p, f.SpaceID)
	if err != nil {
		return
	}
	if sp.Encrypted {
		if v.Size > 32<<20 {
			failure = "preview_encrypted_scratch_limit"
			return
		}
		dir, err = os.MkdirTemp("/dev/shm", "drivebridge-preview-")
		if err != nil {
			failure = "preview_encrypted_scratch_unavailable"
			return
		}
		defer os.RemoveAll(dir)
	}

	input := filepath.Join(dir, "source"+strings.ToLower(filepath.Ext(f.Path)))
	dest := filepath.Join(dir, "output")
	defer func() {
		if !out.Encrypted {
			_ = os.Remove(dest)
		}
	}()
	_, _, body, err := s.Download(ctx, p, f.ID, v.ID)
	if err != nil {
		return
	}
	defer body.Close()
	file, err := os.OpenFile(input, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	n, err := io.Copy(file, io.LimitReader(body, v.Size+1))
	closeErr := file.Close()
	defer os.Remove(input)
	if err != nil || closeErr != nil || n != v.Size {
		return
	}
	if verifier, ok := body.(interface{ Verify() error }); ok && verifier.Verify() != nil {
		return
	}
	out.MIME, err = mediapreview.Convert(ctx, input, dest, out.Kind, v.MIME)
	if err != nil {
		failure = safeCode(err)
		return
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		return
	}
	out.Size = info.Size()
	sealed := filepath.Join(cacheDir, "output.enc")
	if err = s.encryptPreview(dest, sealed, out); err != nil {
		return
	}
	_ = os.Remove(dest)
	if err = os.Rename(sealed, filepath.Join(cacheDir, "output")); err != nil {
		return
	}
	out.Encrypted = true
	status, failure = "ready", ""

}

func (s *Service) servePreview(w http.ResponseWriter, r *http.Request, out Preview) error {
	if out.Status != "ready" {
		return code("preview_not_ready")
	}
	if out.Encrypted {
		aead, err := s.previewAEAD(out.ID)
		if err != nil {
			return err
		}
		seeker := &previewSeeker{size: out.Size, open: func() (io.ReadCloser, error) {
			file, err := os.Open(filepath.Join(s.Config.State, "previews", out.ID, "output"))
			if err != nil {
				return nil, err
			}
			reader, err := newContentOpener(aead, file, out.Size)
			if err != nil {
				file.Close()
				return nil, err
			}
			return &previewStream{Reader: reader, Closer: file}, nil
		}}
		defer seeker.close()
		w.Header().Set("Content-Type", out.MIME)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		http.ServeContent(w, r, "preview", out.Updated, seeker)
		return nil
	}
	f, err := os.Open(filepath.Join(s.Config.State, "previews", out.ID, "output"))
	if err != nil {
		return code("preview_not_ready")
	}
	defer f.Close()
	w.Header().Set("Content-Type", out.MIME)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	http.ServeContent(w, r, "preview", out.Updated, f)
	return nil
}

func (s *Service) previewAEAD(id string) (cipher.AEAD, error) {
	key, err := deriveKey(s.key, "preview:"+id)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Service) encryptPreview(source, target string, out Preview) error {
	aead, err := s.previewAEAD(out.ID)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	nonce, err := secureRandomBytes(12)
	if err != nil {
		return err
	}
	header := append([]byte(contentMagic+"\x01"), nonce...)
	if _, err = output.Write(header); err != nil {
		return err
	}
	writer := newContentSealer(aead, nonce, 0, (out.Size-1)/contentChunk, out.Size, output)
	if _, err = io.Copy(writer, input); err != nil {
		return err
	}
	return writer.Close()
}

type previewStream struct {
	io.Reader
	io.Closer
}
type previewSeeker struct {
	size, position int64
	body           io.ReadCloser
	open           func() (io.ReadCloser, error)
}

func (s *previewSeeker) close() {
	if s.body != nil {
		s.body.Close()
		s.body = nil
	}
}
func (s *previewSeeker) Read(p []byte) (int, error) {
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
func (s *previewSeeker) Seek(offset int64, whence int) (int64, error) {
	target := offset
	switch whence {
	case io.SeekCurrent:
		target += s.position
	case io.SeekEnd:
		target += s.size
	case io.SeekStart:
	default:
		return 0, code("invalid_seek")
	}
	if target < 0 {
		return 0, code("invalid_seek")
	}
	if target != s.position {
		s.close()
	}
	s.position = target
	return target, nil
}
