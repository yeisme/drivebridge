package service

import (
	"archive/zip"
	"io"
	"net/http"
	"strings"
	"time"
)

// Streaming reads revalidate at bounded checkpoints without buffering files.
type governanceReader struct {
	io.ReadCloser
	s       *Service
	p       Principal
	f       File
	budget  int
	checked time.Time
}

func (g *governanceReader) Read(buf []byte) (int, error) {
	if g.budget <= 0 || time.Since(g.checked) >= time.Second {
		if e := g.s.allow(g.p, g.f.SpaceID, g.f.Path, "read"); e != nil {
			return 0, e
		}
		g.budget = 256 << 10
		g.checked = time.Now()
	}
	if len(buf) > g.budget {
		buf = buf[:g.budget]
	}
	n, e := g.ReadCloser.Read(buf)
	g.budget -= n
	return n, e
}
func (s *Service) guardedStream(p Principal, f File, body io.ReadCloser) io.ReadCloser {
	if !s.governanceActive() {
		return body
	}
	return &governanceReader{ReadCloser: body, s: s, p: p, f: f}
}
func (s *Service) exportFiles(w http.ResponseWriter, r *http.Request, p Principal) error {
	ids := strings.Split(r.URL.Query().Get("files"), ",")
	if len(ids) == 0 || len(ids) > 1000 {
		return code("invalid_selection")
	}
	files := []File{}
	seen := map[string]bool{}
	for _, id := range ids {
		f, e := s.file(p, id, "read")
		if e != nil {
			return e
		}
		if f.Trashed || seen[id] {
			return code("invalid_selection")
		}
		seen[id] = true
		files = append(files, f)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=drivebridge-export.zip")
	w.Header().Set("Cache-Control", "no-store")
	archive := zip.NewWriter(w)
	for _, f := range files {
		_, v, body, e := s.Download(r.Context(), p, f.ID, f.VersionID)
		if e != nil {
			return e
		}
		header := &zip.FileHeader{Name: f.Path, Method: zip.Store}
		header.UncompressedSize64 = uint64(v.Size)
		entry, e := archive.CreateHeader(header)
		if e == nil {
			_, e = io.Copy(entry, body)
		}
		body.Close()
		if e != nil {
			return code("transfer_failed")
		}
	}
	return archive.Close()
}
