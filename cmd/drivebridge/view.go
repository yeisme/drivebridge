package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newViewCommand(c *cli) *cobra.Command {
	var web, launchFile string
	var port int
	var noOpen bool
	cmd := &cobra.Command{Use: "view", Short: "Open the loopback-only resource and audit view; file actions use the enabled private owner.", RunE: func(cmd *cobra.Command, _ []string) error {
		if c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain {
			return fmt.Errorf("view_owns_process_output")
		}
		if c.effective.Values["remote_url"] != "" {
			return fail("local_view_only", "view serves the local read-only projection; use the existing remote website for the shared instance.")
		}
		store, release, e := c.readInsights()
		if e != nil {
			return e
		}
		defer release()
		if web == "" {
			binary, e := os.Executable()
			if e != nil {
				return e
			}
			web = filepath.Join(filepath.Dir(binary), "web")
		}
		web, e = filepath.Abs(web)
		if e != nil {
			return e
		}
		if _, e = os.Stat(filepath.Join(web, "insights.html")); e != nil {
			return fail("viewer_assets_missing", "Provide --web-dir pointing at the built DriveBridge Client directory.")
		}
		ln, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if e != nil {
			return fmt.Errorf("viewer_listener_unavailable")
		}
		origin := "http://" + ln.Addr().String()
		bootstrap := insights.ID() + insights.ID()
		session := insights.ID() + insights.ID()
		csrf := insights.ID()
		if launchFile == "" {
			launchFile = filepath.Join(c.effective.Values["state"], "view-"+insights.ID()+".url")
		}
		file, e := os.OpenFile(launchFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			ln.Close()
			return fmt.Errorf("private_launch_file_unavailable")
		}
		launch := origin + "/insights.html#launch=" + url.QueryEscape(bootstrap)
		_, e = file.WriteString(launch + "\n")
		file.Close()
		if e != nil {
			ln.Close()
			os.Remove(launchFile)
			return fmt.Errorf("private_launch_file_unavailable")
		}
		defer os.Remove(launchFile)
		handler := viewerHandler(c, store, web, origin, bootstrap, session, csrf)
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
		if !noOpen {
			var child *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				child = exec.Command("open", launch)
			case "windows":
				// start treats a quoted first argument as the window title;
				// pass an empty title before the private launch URL.
				child = exec.Command("cmd", "/c", "start", "", launch)
			default:
				child = exec.Command("xdg-open", launch)
			}
			if e = child.Start(); e == nil {
				go child.Wait()
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Browser launch unavailable; open the private launch file.")
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Local insights view: %s\nPrivate launch file: %s\n", origin, launchFile)
		go func() {
			<-cmd.Context().Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
		}()
		e = srv.Serve(ln)
		if e == http.ErrServerClosed {
			return nil
		}
		return e
	}}
	cmd.Flags().StringVar(&web, "web-dir", "", "Built Web assets (defaults to web beside the executable)")
	cmd.Flags().StringVar(&launchFile, "launch-file", "", "New private bootstrap URL file; never printed as a URL")
	cmd.Flags().IntVar(&port, "port", 0, "Loopback port; 0 chooses a free port")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not launch a browser; bootstrap URL is only in the private launch file")
	return cmd
}
func viewerHandler(c *cli, store *insights.Store, web, origin, bootstrap, session, csrf string) http.Handler {
	expected := strings.TrimPrefix(origin, "http://")
	cookieName := "drivebridge_view_" + strings.ReplaceAll(expected, ":", "_")
	created := time.Now()
	var mu sync.Mutex
	used := false
	valid := func(a, b string) bool {
		x := sha256.Sum256([]byte(a))
		y := sha256.Sum256([]byte(b))
		return subtle.ConstantTimeCompare(x[:], y[:]) == 1
	}
	respond := func(w http.ResponseWriter, out any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": out})
	}
	fail := func(w http.ResponseWriter, code string, status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
	}
	assets := http.FileServer(http.Dir(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'; object-src 'none'")
		if r.Host != expected || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin {
			fail(w, "origin_rejected", 403)
			return
		}
		site := r.Header.Get("Sec-Fetch-Site")
		if site != "" && site != "same-origin" && site != "none" {
			fail(w, "origin_rejected", 403)
			return
		}
		if r.URL.Path == "/api/v1/local-session" && r.Method == "POST" {
			var q struct {
				Launch string `json:"launch"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
				fail(w, "unauthorized", 401)
				return
			}
			mu.Lock()
			ok := !used && time.Since(created) < 5*time.Minute && valid(q.Launch, bootstrap)
			if ok {
				used = true
			}
			mu.Unlock()
			if !ok {
				fail(w, "unauthorized", 401)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: session, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
			respond(w, map[string]string{"csrf_token": csrf})
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/insights.html" || strings.HasPrefix(r.URL.Path, "/assets/") {
			if r.Method != "GET" {
				fail(w, "read_only", 405)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/") && r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/insights.html", 302)
				return
			}
			assets.ServeHTTP(w, r)
			return
		}
		cookie, e := r.Cookie(cookieName)
		if e != nil || !valid(cookie.Value, session) || time.Since(created) > 12*time.Hour {
			fail(w, "unauthorized", 401)
			return
		}
		if r.Method != "GET" && (r.Header.Get("Origin") != origin || !valid(r.Header.Get("X-CSRF-Token"), csrf)) {
			fail(w, "csrf_rejected", 403)
			return
		}
		if r.URL.Path == "/api/v1/me" && r.Method == "GET" {
			respond(w, map[string]any{"user": map[string]string{"id": "local", "name": "本机文件空间"}, "csrf_token": csrf, "local_view": true})
			return
		}
		if r.URL.Path == "/api/v1/spaces" && r.Method == "GET" {
			spaces := []bridge.Space{}
			if store.DB.Find(&spaces).Error != nil {
				fail(w, "insights_unavailable", 503)
				return
			}
			out := []map[string]string{}
			for _, p := range spaces {
				out = append(out, map[string]string{"id": p.ID, "name": p.ID, "connection_id": p.Remote, "backend": p.Kind()})
			}
			respond(w, out)
			return
		}
		if r.URL.Path == "/api/v1/usage/scans" && r.Method == "POST" {
			var q bridge.ScanRequest
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
				fail(w, "invalid_scan", 400)
				return
			}
			out, e := c.requestScan(r.Context(), q)
			if e != nil {
				fail(w, insightError(e), 400)
				return
			}
			respond(w, out)
			return
		}
		if r.URL.Path == "/api/v1/local/list" && r.Method == "POST" {
			var q struct {
				SpaceID string `json:"space_id"`
				Path    string `json:"path"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil || q.SpaceID == "" || len(q.Path) > 1024 {
				fail(w, "invalid_list", 400)
				return
			}
			out, e := c.requestList(r.Context(), q.SpaceID, q.Path)
			if e != nil {
				code := insightError(e)
				if code == "" {
					code = "list_failed"
				}
				if code == "insights_owner_unavailable" {
					fail(w, code, 503)
					return
				}
				fail(w, code, 400)
				return
			}
			respond(w, out)
			return
		}
		if c.proxyLocalFiles(w, r) {
			return
		}
		if r.Method != "GET" {
			fail(w, "read_only", 403)
			return
		}
		kind := ""
		if strings.HasPrefix(r.URL.Path, "/api/v1/usage/") {
			kind = strings.TrimPrefix(r.URL.Path, "/api/v1/usage/")
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/audit/") {
			kind = "audit_" + strings.TrimPrefix(r.URL.Path, "/api/v1/audit/")
		}
		if kind == "" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		out, e := bridge.QueryInsights(store, kind, q)
		if e != nil {
			fail(w, insightError(e), 400)
			return
		}
		if kind == "export" || kind == "audit_export" {
			out = map[string]any{"export": out, "audit_recorded": c.noteInsightExport(r.Context(), kind)}
		}
		respond(w, out)
	})
}
