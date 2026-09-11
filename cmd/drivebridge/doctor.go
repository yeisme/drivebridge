package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/glebarez/sqlite"
)

// errDoctorNotReady keeps the CLI exit code non-zero after the report has
// already been printed; the generic failure banner is suppressed for it.
var errDoctorNotReady = errors.New("doctor_not_ready")

type doctorConfig struct {
	State           string
	RcloneConfig    string
	ScaenaURL       string
	ScaenaTokenFile string
	Handoff         bool
	RemoteURL       string
	RemoteTokenFile string
	LocalRoot       string
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type doctorReport struct {
	Mode   string        `json:"mode"`
	Ready  bool          `json:"ready"`
	Checks []doctorCheck `json:"checks"`
}

func runDoctor(m outputMode, cfg doctorConfig) error {
	report := buildDoctorReport(cfg)
	var failed []string
	for _, c := range report.Checks {
		if c.Status == "fail" {
			failed = append(failed, c.Name)
		}
	}
	report.Ready = len(failed) == 0
	status, summary := "success", fmt.Sprintf("DriveBridge %s preflight: %d checks, ready.", report.Mode, len(report.Checks))
	if !report.Ready {
		status = "not_ready"
		summary = fmt.Sprintf("DriveBridge %s preflight: %d checks, not ready (failed: %s).", report.Mode, len(report.Checks), joinNames(failed))
	}
	if !(m.JSON || m.Agent || m.Events || m.Explain) {
		for _, c := range report.Checks {
			line := fmt.Sprintf("%-30s %s", c.Name, c.Status)
			if c.Detail != "" {
				line += "  " + c.Detail
			}
			fmt.Println(line)
		}
	}
	// A map keeps --agent fact extraction working; JSON keys match the report.
	data := map[string]any{"mode": report.Mode, "ready": report.Ready, "checks": report.Checks}
	if err := m.write("doctor", status, summary, data); err != nil {
		return err
	}
	if !report.Ready {
		return errDoctorNotReady
	}
	return nil
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

type doctorSpace struct {
	ID, Remote, LocalRoot, Project, Actor string
	MaxBytes                              int64
}

func buildDoctorReport(cfg doctorConfig) doctorReport {
	if cfg.RemoteURL != "" {
		return doctorRemote(cfg)
	}
	return doctorLocal(cfg)
}

func doctorLocal(cfg doctorConfig) doctorReport {
	r := doctorReport{Mode: "local", Checks: []doctorCheck{}}
	add := func(name, status, detail string) { r.Checks = append(r.Checks, doctorCheck{name, status, detail}) }

	// Private state directory; doctor must not create it.
	info, err := os.Lstat(cfg.State)
	switch {
	case err != nil:
		add("state_dir", "fail", "state_dir_missing: run a DriveBridge command once or create it with 0700")
	case !info.IsDir():
		add("state_dir", "fail", "state_dir_invalid: not a directory")
	case info.Mode().Perm()&0077 != 0:
		add("state_dir", "fail", "state_dir_not_private: chmod 0700 required")
	default:
		add("state_dir", "pass", "")
	}

	// rclone config: presence, privacy, and remote section names (names only).
	sections, cfgErr := rcloneSections(cfg.RcloneConfig)
	switch {
	case cfgErr != nil:
		add("rclone_config", "fail", "rclone_config_missing: run 'drivebridge auth' first")
	default:
		if permErr := privateFile(cfg.RcloneConfig, 8192<<10); permErr != "" {
			add("rclone_config", "fail", permErr)
		} else {
			add("rclone_config", "pass", fmt.Sprintf("%d remote(s) configured", len(sections)))
		}
	}

	// Spaces from a read-only database snapshot.
	spaces, dbErr := readSpaces(filepath.Join(cfg.State, "state.sqlite"))
	switch {
	case dbErr == "state_database_missing":
		add("spaces", "fail", "state_database_missing: run 'drivebridge space-add' first")
	case dbErr != "":
		add("spaces", "fail", dbErr)
	case len(spaces) == 0:
		add("spaces", "fail", "no_spaces: run 'drivebridge space-add' first")
	default:
		add("spaces", "pass", fmt.Sprintf("%d space(s)", len(spaces)))
	}
	for _, p := range spaces {
		if !sections[p.Remote] {
			add("space."+p.ID+".remote", "fail", "rclone_remote_missing: name not found in the rclone config; re-run auth")
		} else {
			add("space."+p.ID+".remote", "pass", "")
		}
		detail, ok := writableRoot(p.LocalRoot)
		if !ok {
			add("space."+p.ID+".local_root", "fail", detail)
		} else {
			add("space."+p.ID+".local_root", "pass", "")
		}
		if p.MaxBytes <= 0 {
			add("space."+p.ID+".limit", "fail", "max_bytes_not_set")
		}
		if p.Project == "" || p.Actor == "" {
			add("space."+p.ID+".save_output", "info", "project/actor not set; save_output unavailable for this space")
		}
	}

	// Video reference validation dependency.
	if _, err := exec.LookPath("ffprobe"); err != nil {
		add("ffprobe", "fail", "ffprobe_missing: required for MP4 reference validation")
	} else {
		add("ffprobe", "pass", "")
	}

	// Private owner socket.
	socket := filepath.Join(cfg.State, "handoff.sock")
	if _, err := os.Lstat(socket); err != nil {
		if cfg.Handoff {
			add("handoff_socket", "fail", "handoff_socket_missing: start 'drivebridge mcp --enabled --handoff'")
		} else {
			add("handoff_socket", "info", "not started; required only for Scaena reference handoff")
		}
	} else if conn, err := net.DialTimeout("unix", socket, time.Second); err != nil {
		add("handoff_socket", "fail", "handoff_socket_unreachable: confirm the old process stopped, then remove the stale socket")
	} else {
		conn.Close()
		add("handoff_socket", "pass", "listener alive")
	}

	// Optional Scaena output adapter.
	if cfg.ScaenaURL != "" {
		if !validEndpoint(cfg.ScaenaURL) {
			add("scaena_endpoint", "fail", "scaena_endpoint_invalid: https origin (or loopback http) without path required")
		} else {
			add("scaena_endpoint", "pass", "")
			if detail, ok := credentialReady(cfg.ScaenaTokenFile); !ok {
				add("scaena_token", "fail", detail)
			} else {
				add("scaena_token", "pass", "authorization not probed")
			}
			if detail, ok := endpointReachable(cfg.ScaenaURL); !ok {
				add("scaena_reachable", "fail", detail)
			} else {
				add("scaena_reachable", "pass", detail)
			}
		}
	}
	add("onedrive_cloud", "info", "global cloud only in this version")
	return r
}

func doctorRemote(cfg doctorConfig) doctorReport {
	r := doctorReport{Mode: "remote", Checks: []doctorCheck{}}
	add := func(name, status, detail string) { r.Checks = append(r.Checks, doctorCheck{name, status, detail}) }

	if !validEndpoint(cfg.RemoteURL) {
		add("remote_url", "fail", "invalid_service_url: https origin (or loopback http) without path required")
	} else {
		add("remote_url", "pass", "")
	}
	if detail, ok := credentialReady(cfg.RemoteTokenFile); !ok {
		add("remote_token_file", "fail", detail)
	} else {
		add("remote_token_file", "pass", "authorization not probed")
	}
	if !filepath.IsAbs(cfg.LocalRoot) {
		add("local_root", "fail", "absolute_local_root_required")
	} else if detail, ok := writableRoot(cfg.LocalRoot); !ok {
		add("local_root", "fail", detail)
	} else {
		add("local_root", "pass", "")
	}
	if validEndpoint(cfg.RemoteURL) {
		if detail, ok := endpointReachable(cfg.RemoteURL); !ok {
			add("remote_reachable", "fail", detail)
		} else {
			add("remote_reachable", "pass", detail)
		}
		add("remote_authorization", "info", "not probed; use 'capabilities' for live service status")
	}
	return r
}

// rcloneSections parses INI section headers only; values and obscured
// credentials are never read.
func rcloneSections(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sections := map[string]bool{}
	for _, line := range splitLines(b) {
		if len(line) > 2 && line[0] == '[' && line[len(line)-1] == ']' {
			name := line[1 : len(line)-1]
			if name != "RCLONE" {
				sections[name] = true
			}
		}
	}
	return sections, nil
}

func splitLines(b []byte) []string {
	var out []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			line := b[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, string(line))
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, string(b[start:]))
	}
	return out
}

func privateFile(path string, maxBytes int64) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "credential_file_unavailable: regular file required"
	}
	if info.Mode().Perm()&0077 != 0 {
		return "credential_file_not_private: chmod 0600 required"
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return "credential_file_unavailable: file too large"
	}
	return ""
}

func credentialReady(tokenFile string) (string, bool) {
	if tokenFile != "" {
		if detail := privateFile(tokenFile, 8192); detail != "" {
			return detail, false
		}
		return "", true
	}
	if os.Getenv("SCAENA_API_TOKEN") != "" {
		return "", true
	}
	return "credential_file_unavailable: provide --scaena-token-file or SCAENA_API_TOKEN", false
}

func writableRoot(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "local_root_unavailable: directory required", false
	}
	if err := syscall.Access(path, 2 /* W_OK, identical on linux and darwin */); err != nil {
		return "local_root_not_writable", false
	}
	return "", true
}

func validEndpoint(base string) bool {
	u, err := url.Parse(base)
	return err == nil && u.Host != "" && u.Path == "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" &&
		(u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")))
}

// endpointReachable reports transport reachability only; any HTTP status
// proves a listener and authorization stays not probed.
func endpointReachable(base string) (string, bool) {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(base + "/healthz")
	if err != nil {
		return "service_unreachable: check the address and listener", false
	}
	defer resp.Body.Close()
	return fmt.Sprintf("http_status=%d; authorization not probed", resp.StatusCode), true
}

// readSpaces opens the state database strictly read-only so a running MCP
// process is never blocked or mutated by a preflight.
func readSpaces(dbPath string) ([]doctorSpace, string) {
	if _, err := os.Lstat(dbPath); err != nil {
		return nil, "state_database_missing"
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return nil, "state_database_unreadable"
	}
	defer db.Close()
	rows, err := db.Query("SELECT id, remote, local_root, max_bytes, project, actor FROM spaces")
	if err != nil {
		return nil, "state_database_unreadable"
	}
	defer rows.Close()
	var out []doctorSpace
	for rows.Next() {
		var p doctorSpace
		if err := rows.Scan(&p.ID, &p.Remote, &p.LocalRoot, &p.MaxBytes, &p.Project, &p.Actor); err != nil {
			return nil, "state_database_unreadable"
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "state_database_unreadable"
	}
	return out, ""
}
