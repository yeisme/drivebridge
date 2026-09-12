package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "github.com/glebarez/sqlite"
	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
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
	ConfigPath      string
	ProbeAuth       bool
}

// newDoctorCommand wires the read-only preflight; config-file defaults are
// resolved through the shared configuration surface.
func newDoctorCommand(c *cli) *cobra.Command {
	var handoff, probeAuth bool
	var scaenaURL, scaenaTokenFile, remoteURL, remoteToken, localRoot string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Read-only preflight: report named pass/fail checks, never probe accounts.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(c.mode, doctorConfig{
				State:           c.effective.Values["state"],
				RcloneConfig:    c.effective.Values["rclone_config"],
				ScaenaURL:       c.effective.Values["scaena_url"],
				ScaenaTokenFile: c.effective.Values["scaena_token_file"],
				Handoff:         handoff,
				RemoteURL:       c.effective.Values["remote_url"],
				RemoteTokenFile: c.effective.Values["remote_token_file"],
				LocalRoot:       c.effective.Values["local_root"],
				ConfigPath:      c.effective.Path,
				ProbeAuth:       probeAuth,
			})
		},
	}
	cmd.Flags().BoolVar(&handoff, "handoff", false, "Declare handoff intent so a missing listener becomes a failure")
	cmd.Flags().BoolVar(&probeAuth, "probe-auth", false, "Opt in to one authenticated request against the configured OneDrive remote; the default stays not probed")
	cmd.Flags().StringVar(&scaenaURL, "scaena-url", "", "Scaena owner URL for output recovery")
	cmd.Flags().StringVar(&scaenaTokenFile, "scaena-token-file", "", "Private file containing the existing Scaena API credential")
	cmd.Flags().StringVar(&remoteURL, "remote-url", "", "Remote DriveBridge service origin")
	cmd.Flags().StringVar(&remoteToken, "remote-token-file", "", "Private scoped Agent token file")
	cmd.Flags().StringVar(&localRoot, "local-root", "", "Allowed absolute local directory")
	return cmd
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
	Backend, RemotePath                   string
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
		if p.Backend == "generic" {
			add("space."+p.ID+".backend", "info", "generic rclone backend (transfer-only; no direct URL or reference handoff)")
		}
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

	// Video reference validation dependency: only OneDrive spaces can take
	// part in reference handoff, so a generic-only setup degrades to info.
	oneDriveSpaces := false
	for _, p := range spaces {
		if p.Backend == "" || p.Backend == "onedrive" {
			oneDriveSpaces = true
		}
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		if oneDriveSpaces {
			add("ffprobe", "fail", "ffprobe_missing: required for MP4 reference validation")
		} else {
			add("ffprobe", "info", "ffprobe_missing: reference validation unused (no OneDrive space)")
		}
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

	// Optional credential probe: the default report stays read-only with no
	// account access; --probe-auth is an explicit opt-in to one authenticated
	// request through the rclone credential path.
	if cfg.ProbeAuth {
		remote, detail, ok := onedriveRemote(cfg.RcloneConfig)
		if !ok {
			add("onedrive_auth", "info", detail)
		} else {
			if e := probeOneDriveAuth(cfg.RcloneConfig, remote); e != "" {
				add("onedrive_auth", "fail", e)
			} else {
				add("onedrive_auth", "pass", "probe succeeded through "+remote)
			}
		}
	} else {
		add("onedrive_auth", "info", "not probed; pass --probe-auth to verify the stored credential")
	}

	// Optional configuration file: informational only, never a gate.
	if cfg.ConfigPath == "" {
		add("config_file", "info", "no config file; flags, environment and defaults only")
	} else if info, e := os.Lstat(cfg.ConfigPath); e != nil || !info.Mode().IsRegular() {
		add("config_file", "info", "path resolved but unreadable: "+cfg.ConfigPath)
	} else if info.Mode().Perm()&0077 != 0 {
		add("config_file", "pass", cfg.ConfigPath+" (group/other readable; 0600 recommended)")
	} else {
		add("config_file", "pass", cfg.ConfigPath)
	}
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

// onedriveRemote finds the first OneDrive-typed remote that stores both a
// token and a drive id. Only key names are inspected; credential values are
// never read.
func onedriveRemote(path string) (string, string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "no_onedrive_remote: run 'drivebridge auth' first", false
	}
	type sectionState struct {
		onedrive, token, driveID bool
	}
	order := []string{}
	states := map[string]*sectionState{}
	current := ""
	for _, line := range splitLines(b) {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			current = trimmed[1 : len(trimmed)-1]
			if current != "RCLONE" && states[current] == nil {
				states[current] = &sectionState{}
				order = append(order, current)
			}
			continue
		}
		if current == "" || current == "RCLONE" {
			continue
		}
		st := states[current]
		i := strings.Index(trimmed, "=")
		if st == nil || i <= 0 {
			continue
		}
		switch strings.TrimSpace(trimmed[:i]) {
		case "type":
			if strings.TrimSpace(trimmed[i+1:]) == "onedrive" {
				st.onedrive = true
			}
		case "token":
			st.token = true
		case "drive_id":
			st.driveID = true
		}
	}
	for _, name := range order {
		if st := states[name]; st.onedrive && st.token && st.driveID {
			return name, "", true
		}
	}
	return "", "no_onedrive_remote: run 'drivebridge auth --auto' first", false
}

// probeOneDriveAuth performs one authenticated request through the embedded
// rclone credential path; tokens refresh inside rclone as usual. The empty
// string means the probe passed.
func probeOneDriveAuth(rcloneConfigPath, remote string) string {
	configfile.Install()
	if err := rcloneconfig.SetConfigPath(rcloneConfigPath); err != nil {
		return "authentication_required: rclone config unavailable"
	}
	provider := bridge.NewOneDrive()
	if err := provider.Probe(context.Background(), remote); err != nil {
		return "authentication_required: " + err.Error() + " (re-run 'drivebridge auth')"
	}
	return ""
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
// process is never blocked or mutated by a preflight. Older databases without
// the generic-backend columns fall back to the v1 column set.
func readSpaces(dbPath string) ([]doctorSpace, string) {
	if _, err := os.Lstat(dbPath); err != nil {
		return nil, "state_database_missing"
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return nil, "state_database_unreadable"
	}
	defer db.Close()
	rows, err := db.Query("SELECT id, remote, local_root, max_bytes, project, actor, backend, root_id FROM spaces")
	if err != nil {
		rows, err = db.Query("SELECT id, remote, local_root, max_bytes, project, actor, '' AS backend, '' AS root_id FROM spaces")
		if err != nil {
			return nil, "state_database_unreadable"
		}
	}
	defer rows.Close()
	var out []doctorSpace
	for rows.Next() {
		var p doctorSpace
		if err := rows.Scan(&p.ID, &p.Remote, &p.LocalRoot, &p.MaxBytes, &p.Project, &p.Actor, &p.Backend, &p.RemotePath); err != nil {
			return nil, "state_database_unreadable"
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "state_database_unreadable"
	}
	return out, ""
}
