package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type HandoffRequest struct {
	Ref     string `json:"ref"`
	Project string `json:"project"`
	Actor   string `json:"actor"`
}
type HandoffResponse struct {
	File File   `json:"file"`
	URL  string `json:"url,omitempty"`
}

func (s *Service) reference(ctx context.Context, q HandoffRequest) (File, Space, error) {
	f, err := s.Stat(ctx, q.Ref)
	if err != nil {
		return f, Space{}, err
	}
	p, err := s.Space(f.SpaceID)
	if err != nil {
		return f, p, err
	}
	if p.Kind() != "onedrive" {
		return f, p, errors.New("handoff_not_available")
	}
	if f.Trashed != nil {
		return f, p, errors.New("file_trashed")
	}
	if !s.Handoff || p.Project == "" || p.Actor == "" || p.Project != q.Project || p.Actor != q.Actor {
		return f, p, errors.New("secure_handoff_unavailable")
	}
	if f.Size <= 0 || f.Size > p.MaxBytes {
		return f, p, errors.New("file_size_invalid")
	}
	// Pin a verified media digest, not the Graph MIME declaration. This validation
	// cache is reusable only for this exact item version.
	var old File
	s.DB.First(&old, "ref = ?", f.Ref)
	if old.Verified {
		return old, p, nil
	}
	in, err := s.providerFor(p).Open(ctx, p, f)
	if err != nil {
		return f, p, err
	}
	defer in.Close()
	temp, err := os.CreateTemp("", "drivebridge-probe-*")
	if err != nil {
		return f, p, errors.New("probe_unavailable")
	}
	defer os.Remove(temp.Name())
	hash, n, err := digestReader(io.TeeReader(io.LimitReader(in, p.MaxBytes+1), temp))
	temp.Close()
	if err != nil || n != f.Size {
		return f, p, errors.New("content_mismatch")
	}
	mime, duration, err := probeMedia(ctx, temp.Name())
	if err != nil {
		return f, p, err
	}
	if _, err = s.Stat(ctx, f.Ref); err != nil {
		return f, p, err
	}
	f.MIME = mime
	f.Duration = duration
	f.SHA256 = hash
	f.Verified = true
	if err = s.DB.Save(&f).Error; err != nil {
		return f, p, errors.New("state_unavailable")
	}
	return f, p, nil
}
func probeMedia(ctx context.Context, path string) (string, int, error) {
	in, e := os.Open(path)
	if e != nil {
		return "", 0, errors.New("probe_unavailable")
	}
	defer in.Close()
	cfg, format, e := image.DecodeConfig(in)
	if e == nil && cfg.Width > 0 && cfg.Height > 0 && int64(cfg.Width)*int64(cfg.Height) <= 20_000_000 && (format == "png" || format == "jpeg") {
		if _, e = in.Seek(0, 0); e != nil {
			return "", 0, e
		}
		if _, _, e = image.Decode(in); e == nil {
			return "image/" + format, 0, nil
		}
	}
	child, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(child, "ffprobe", "-v", "error", "-protocol_whitelist", "file", "-format_whitelist", "mov", "-enable_drefs", "0", "-show_entries", "format=format_name,duration:stream=codec_type", "-of", "json", path)
	// ffprobe only reports selected metadata; stderr and raw input never escape.
	bytes, e := command.Output()
	if e != nil || len(bytes) > 64<<10 {
		return "", 0, errors.New("media_probe_failed")
	}
	var result struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type string `json:"codec_type"`
		} `json:"streams"`
	}
	if json.Unmarshal(bytes, &result) != nil || !strings.Contains(result.Format.Name, "mp4") {
		return "", 0, errors.New("media_type_unsupported")
	}
	d, e := strconv.ParseFloat(result.Format.Duration, 64)
	if e != nil || d <= 0 || d > 86400 {
		return "", 0, errors.New("media_duration_invalid")
	}
	for _, stream := range result.Streams {
		if stream.Type == "video" {
			return "video/mp4", int(d + 0.999), nil
		}
	}
	return "", 0, errors.New("media_type_unsupported")
}
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, route := range []string{"metadata", "acquire"} {
		route := route
		mux.HandleFunc("POST /v1/"+route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			var q HandoffRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&q) != nil {
				http.Error(w, "invalid_request", 400)
				return
			}
			f, p, err := s.reference(r.Context(), q)
			if err != nil {
				http.Error(w, safeError(err), 403)
				return
			}
			out := HandoffResponse{File: f}
			if route == "acquire" {
				out.URL, err = s.providerFor(p).Access(r.Context(), p, f)
				if err != nil {
					http.Error(w, safeError(err), 409)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(out)
		})
	}
	return mux
}

// dialUnixSocket reports whether the socket still answers; nil error means
// a live consumer is on the other end.
func dialUnixSocket(socket string) error {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err == nil {
		conn.Close()
	}
	return err
}

func (s *Service) Listen(socket string) (*http.Server, error) {
	if !s.Enabled || !s.Handoff {
		return nil, errors.New("secure_handoff_unavailable")
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		// A socket left behind by an abnormal stop would block every restart.
		// Only a provably dead one (dial fails) is reclaimed; a live or
		// unknown socket keeps the explicit operator-cleanup rule.
		if dialUnixSocket(socket) == nil {
			return nil, errors.New("socket_unavailable")
		}
		_ = os.Remove(socket)
		listener, err = net.Listen("unix", socket)
		if err != nil {
			return nil, errors.New("socket_unavailable")
		}
	}
	if err = os.Chmod(socket, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	server := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return server, nil
}
