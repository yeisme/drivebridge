package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/sdk/mediapreview"
)

func (s *Service) localPreviewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /files/preview", func(w http.ResponseWriter, r *http.Request) {
		fail := func(err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": localFileError(err)}})
		}
		var q struct {
			Ref  string `json:"ref"`
			Kind string `json:"kind"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil || q.Kind != "thumbnail" && q.Kind != "compatible" {
			fail(errors.New("invalid_preview"))
			return
		}
		f, err := s.Stat(r.Context(), q.Ref)
		if err != nil {
			fail(err)
			return
		}
		if f.Trashed != nil || f.Size > 2<<30 {
			fail(errors.New("preview_size_limit"))
			return
		}
		sum := sha256.Sum256([]byte(f.Ref + ":" + q.Kind))
		lock := &s.previewLocks[sum[0]%32]
		lock.Lock()
		defer lock.Unlock()
		key := hex.EncodeToString(sum[:])
		dir := filepath.Join(s.state, "preview-cache", key)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		select {
		case s.previewSlots <- struct{}{}:
			defer func() { <-s.previewSlots }()
		case <-ctx.Done():
			fail(errors.New("preview_interrupted"))
			return
		}
		filePath := filepath.Join(dir, "output")
		typePath := filepath.Join(dir, "media-type")
		mediaType, readErr := os.ReadFile(typePath)
		if _, err := os.Stat(filePath); err != nil || readErr != nil {
			if os.MkdirAll(dir, 0700) != nil {
				fail(errors.New("local_write_failed"))
				return
			}
			p, err := s.Space(f.SpaceID)
			if err != nil {
				fail(err)
				return
			}
			input := filepath.Join(dir, "source"+strings.ToLower(filepath.Ext(f.Name)))
			defer os.Remove(input)
			body, err := s.providerFor(p).Open(ctx, p, f)
			if err != nil {
				fail(err)
				return
			}
			defer body.Close()
			out, err := os.OpenFile(input, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				fail(errors.New("local_write_failed"))
				return
			}
			n, copyErr := io.Copy(out, io.LimitReader(body, f.Size+1))
			closeErr := out.Close()
			if n != f.Size || copyErr != nil || closeErr != nil {
				fail(errors.New("file_version_changed"))
				return
			}
			if _, err = s.Stat(ctx, f.Ref); err != nil {
				fail(err)
				return
			}
			media := f.MIME
			if media == "" {
				media = mime.TypeByExtension(filepath.Ext(f.Name))
			}
			media, err = mediapreview.Convert(ctx, input, filePath, q.Kind, media)
			if err != nil {
				os.Remove(filePath)
				fail(err)
				return
			}
			if os.WriteFile(typePath, []byte(media), 0600) != nil {
				fail(errors.New("local_write_failed"))
				return
			}
			os.Chmod(filePath, 0600)
			mediaType = []byte(media)
		}
		result, err := os.Open(filePath)
		if err != nil {
			fail(errors.New("preview_not_ready"))
			return
		}
		defer result.Close()
		w.Header().Set("Content-Type", string(mediaType))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		request := r.Clone(r.Context())
		request.Method = "GET"
		http.ServeContent(w, request, "preview", time.Time{}, result)
	})
}
