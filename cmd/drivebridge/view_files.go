package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// The loopback viewer forwards only these file lanes to the private owner.
// It never opens a second mutable database or accepts a caller-supplied origin.
func (c *cli) proxyLocalFiles(w http.ResponseWriter, r *http.Request) bool {
	target := ""
	method := "POST"
	var body io.Reader = r.Body
	switch r.URL.Path {
	case "/api/v1/local/file/preview":
		if r.Method == "GET" {
			target = "/files/preview"
			b, _ := json.Marshal(map[string]string{"ref": r.URL.Query().Get("ref"), "kind": r.URL.Query().Get("kind")})
			body = bytes.NewReader(b)
		}
	case "/api/v1/local/file/creative":
		if r.Method == "POST" {
			target = "/files/creative"
		}
	case "/api/v1/local/file/share":
		if r.Method == "POST" {
			target = "/files/share"
		}
	case "/api/v1/local/file/action":
		if r.Method == "POST" {
			target = "/files/action"
		}
	case "/api/v1/local/file/content":
		if r.Method == "GET" || r.Method == "HEAD" {
			target = "/files/content"
			b, _ := json.Marshal(map[string]string{"ref": r.URL.Query().Get("ref")})
			body = bytes.NewReader(b)
		}
	case "/api/v1/local/file/upload":
		if r.Method == "PUT" {
			target = "/files/upload?" + r.URL.RawQuery
			method = "PUT"
		}
	}
	if target == "" {
		return false
	}
	fail := func() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		io.WriteString(w, `{"error":{"code":"insights_owner_unavailable"}}`)
	}
	socket := filepath.Join(c.effective.Values["state"], "insights.sock")
	info, err := os.Lstat(socket)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 {
		fail()
		return true
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	req, err := http.NewRequestWithContext(r.Context(), method, "http://localhost"+target, body)
	if err != nil {
		fail()
		return true
	}
	req.Header.Set("Range", r.Header.Get("Range"))
	req.Header.Set("Idempotency-Key", r.Header.Get("Idempotency-Key"))
	if method == "PUT" {
		req.ContentLength = r.ContentLength
	}
	resp, err := (&http.Client{Transport: tr, Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		fail()
		return true
	}
	defer resp.Body.Close()
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Content-Range", "Accept-Ranges", "Content-Security-Policy"} {
		if v := resp.Header.Get(key); v != "" {
			w.Header().Set(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method != "HEAD" {
		_, _ = io.Copy(w, resp.Body)
	}
	return true
}
