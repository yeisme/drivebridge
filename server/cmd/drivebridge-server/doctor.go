package main

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/internal/service"
)

// errDoctorNotReady keeps the exit code non-zero after the report has already
// been printed; the generic failure banner is suppressed for it.
var errDoctorNotReady = errors.New("doctor_not_ready")

type doctorConfig struct {
	State      string
	WebDir     string
	Issuer     string
	Backup     string
	ConfigPath string
}

// newDoctorCommand wires the read-only deployment preflight through the
// shared configuration surface (state/web-dir/issuer may come from config).
func newDoctorCommand(c *cli) *cobra.Command {
	var webDir, issuer, backup string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Read-only deployment preflight: state consistency, web assets, platform storage, issuer discovery.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(c.mode, doctorConfig{
				State:      c.effective.Values["state"],
				WebDir:     c.effective.Values["web_dir"],
				Issuer:     c.effective.Values["issuer"],
				Backup:     backup,
				ConfigPath: c.effective.Path,
			})
		},
	}
	cmd.Flags().StringVar(&webDir, "web-dir", "", "Built web client directory")
	cmd.Flags().StringVar(&issuer, "issuer", "", "OIDC issuer")
	cmd.Flags().StringVar(&backup, "backup", "", "Backup directory for doctor --backup cross-checks")
	return cmd
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type doctorReport struct {
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
	status, message := "success", fmt.Sprintf("DriveBridge server preflight: %d checks, ready.", len(report.Checks))
	if !report.Ready {
		status = "not_ready"
		message = fmt.Sprintf("DriveBridge server preflight: %d checks, not ready (failed: %s).", len(report.Checks), strings.Join(failed, ", "))
	}
	if !(m.json || m.agent || m.events || m.explain) {
		for _, c := range report.Checks {
			line := fmt.Sprintf("%-26s %s", c.Name, c.Status)
			if c.Detail != "" {
				line += "  " + c.Detail
			}
			fmt.Println(line)
		}
	}
	data := map[string]any{"ready": report.Ready, "checks": report.Checks}
	if e := m.emitData("doctor", status, message, data); e != nil {
		return e
	}
	if !report.Ready {
		return errDoctorNotReady
	}
	return nil
}

// buildDoctorReport never writes, never takes the instance lock, and never
// calls service.Open, whose startup repairs would mutate live operations.
// expiredExpiry tolerates the driver's stored time formats: zero values mean
// the key never expires.
func expiredExpiry(v sql.NullString) bool {
	if !v.Valid || v.String == "" || strings.HasPrefix(v.String, "0001-01-01") {
		return false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05+00:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, v.String); err == nil {
			return time.Now().After(t)
		}
	}
	return false
}

// loginableUserKeys counts non-revoked, non-expired user keys through the
// read-only snapshot; the second return value is a failure detail.
func loginableUserKeys(dbPath string) (int, string) {
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if e != nil {
		return 0, "state_unavailable"
	}
	defer db.Close()
	rows, e := db.Query("SELECT revoked, expires_at, COALESCE(disabled, 0), COALESCE(archived, 0) FROM user_keys")
	if e != nil {
		rows, e = db.Query("SELECT revoked, expires_at FROM user_keys")
	}
	if e != nil {
		// A pre-users schema has no loginable keys to count either.
		return 0, "no_loginable_user_keys: run 'drivebridge-server users import' or configure an OIDC issuer"
	}
	defer rows.Close()
	n := 0
	cols, _ := rows.Columns()
	wide := len(cols) >= 4
	for rows.Next() {
		var revoked, disabled, archived bool
		var expires sql.NullString
		var scanErr error
		if wide {
			scanErr = rows.Scan(&revoked, &expires, &disabled, &archived)
		} else {
			scanErr = rows.Scan(&revoked, &expires)
		}
		if scanErr != nil {
			return 0, "state_unavailable"
		}
		if revoked || archived || disabled || expiredExpiry(expires) {
			continue
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, "state_unavailable"
	}
	return n, ""
}

func buildDoctorReport(cfg doctorConfig) doctorReport {
	r := doctorReport{Checks: []doctorCheck{}}
	add := func(name, status, detail string) { r.Checks = append(r.Checks, doctorCheck{name, status, detail}) }

	stateOK := false
	if info, e := os.Lstat(cfg.State); e != nil {
		add("state_dir", "fail", "state_dir_missing: serve once to initialize, or create it with 0700")
	} else if !info.IsDir() {
		add("state_dir", "fail", "state_dir_invalid: not a directory")
	} else if info.Mode().Perm()&0077 != 0 {
		add("state_dir", "fail", "state_dir_not_private: chmod 0700 required")
	} else {
		add("state_dir", "pass", "")
		stateOK = true
	}

	dbPath := filepath.Join(cfg.State, "metadata.sqlite")
	dbExists := false
	if stateOK {
		if _, e := os.Lstat(dbPath); e == nil {
			dbExists = true
		}
	}

	keyOK := checkInstanceFile(add, "instance_key", filepath.Join(cfg.State, "instance.key"), 32, dbExists)
	checkInstanceFile(add, "instance_id", filepath.Join(cfg.State, "instance.id"), 48, dbExists)

	switch {
	case !stateOK:
		add("state_consistency", "fail", "cannot inspect state; fix state_dir first")
	case dbExists && !keyOK:
		add("state_consistency", "fail", "instance_key_missing_restore_required: restore instance.key from the secret backup; regenerating would orphan encrypted credentials")
	case !dbExists:
		add("state_consistency", "info", "fresh state: the first serve initializes key, identity and database")
	default:
		add("state_consistency", "pass", "")
	}

	if dbExists {
		if detail := privateMode(dbPath); detail != "" {
			add("metadata_db", "fail", detail)
		} else {
			add("metadata_db", "pass", "")
		}
		if state, disabled, err := platformStorage(dbPath); err != "" {
			add("platform_storage", "fail", err)
		} else if disabled {
			add("platform_storage", "fail", "platform_storage_disabled: reconfigure storage")
		} else if state == "" {
			add("platform_storage", "fail", "platform_storage_missing: run 'drivebridge-server platform-connect' with the S3 configuration on stdin")
		} else {
			add("platform_storage", "pass", "configured (status="+state+")")
		}
	}

	if cfg.WebDir != "" {
		if info, e := os.Stat(cfg.WebDir); e != nil || !info.IsDir() {
			add("web_dir", "fail", "web_dir_unavailable: run make build in the DriveBridge repository and point --web-dir at web/dist")
		} else if _, e := os.Stat(filepath.Join(cfg.WebDir, "index.html")); e != nil {
			add("web_dir", "fail", "web_dir_missing_index: expected built assets containing index.html")
		} else {
			add("web_dir", "pass", "")
		}
	}

	if cfg.Issuer != "" {
		if detail, ok := issuerDiscovery(cfg.Issuer); !ok {
			add("oidc_issuer", "fail", detail)
		} else {
			add("oidc_issuer", "pass", detail)
		}
	}
	if dbExists {
		if detail, ok := encryptionKeyVersions(filepath.Join(cfg.State, "metadata.sqlite")); !ok {
			add("content_encryption", "fail", detail)
		} else if detail != "" {
			add("content_encryption", "pass", detail)
		}
		// A deployment with no loginable identity looks healthy but locks
		// everyone out: warn before the first serve, not after.
		if n, err := loginableUserKeys(dbPath); err != "" {
			add("login_identities", "fail", err)
		} else if n == 0 && cfg.Issuer == "" {
			add("login_identities", "fail", "no_loginable_user_keys: run 'drivebridge-server users import' or configure an OIDC issuer")
		} else if n == 0 {
			add("login_identities", "info", "no user keys yet; OIDC issuer configured")
		} else {
			add("login_identities", "pass", "")
		}
	}
	if cfg.Backup != "" {
		for _, c := range backupChecks(cfg.Backup) {
			add(c.Name, c.Status, c.Detail)
		}
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

// encryptionKeyVersions reports the wrapped-DEK key version distribution so
// operators can see rotation coverage at a glance.
func encryptionKeyVersions(dbPath string) (string, bool) {
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if e != nil {
		return "", true
	}
	defer db.Close()
	rows, e := db.Query("SELECT wrapped_dek FROM spaces WHERE encrypted = 1")
	if e != nil {
		return "", true
	}
	defer rows.Close()
	distribution := map[byte]int{}
	total := 0
	for rows.Next() {
		var blob []byte
		if e := rows.Scan(&blob); e != nil || len(blob) < 1 {
			return "encrypted_space_dek_unreadable: wrapped key material is malformed", false
		}
		distribution[blob[0]]++
		total++
	}
	if err := rows.Err(); err != nil {
		return "encrypted_space_dek_unreadable: metadata unreadable", false
	}
	if total == 0 {
		return "", true
	}
	versions := make([]string, 0, len(distribution))
	for version, count := range distribution {
		versions = append(versions, fmt.Sprintf("v%d:%d", version, count))
	}
	sort.Strings(versions)
	return fmt.Sprintf("%d encrypted space(s); wrapped DEK versions %s", total, strings.Join(versions, ",")), true
}

// backupChecks validates a backup directory read-only: the three pieces, the
// manifest schema and the manifest↔metadata cross-check.
func backupChecks(dir string) []doctorCheck {
	checks := []doctorCheck{}
	add := func(name, status, detail string) { checks = append(checks, doctorCheck{name, status, detail}) }
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() {
		add("backup_dir", "fail", "backup_dir_missing: point --backup at an existing backup directory")
		return checks
	}
	if info.Mode().Perm()&0077 != 0 {
		add("backup_dir", "fail", "backup_dir_not_private: chmod 0700 required")
		return checks
	}
	add("backup_dir", "pass", "")
	dbOK := false
	if detail := privateMode(filepath.Join(dir, "metadata.sqlite")); detail != "" {
		add("backup_metadata", "fail", detail)
	} else {
		add("backup_metadata", "pass", "")
		dbOK = true
	}
	instanceOK := false
	if b, e := os.ReadFile(filepath.Join(dir, "instance.id")); e != nil || len(b) != 48 {
		add("backup_instance_id", "fail", "backup_instance_id_invalid: expected 48 hex characters")
	} else if _, decodeErr := hex.DecodeString(string(b)); decodeErr != nil {
		add("backup_instance_id", "fail", "backup_instance_id_invalid: expected 48 hex characters")
	} else if detail := privateMode(filepath.Join(dir, "instance.id")); detail != "" {
		add("backup_instance_id", "fail", detail)
	} else {
		add("backup_instance_id", "pass", "")
		instanceOK = true
	}
	manifest, manifestErr := service.ReadBackupManifest(dir)
	if manifestErr != nil {
		add("backup_manifest", "fail", "backup_manifest_invalid: "+manifestErr.Error()+" (re-create the backup if the manifest was lost)")
		return checks
	}
	if detail := privateMode(filepath.Join(dir, "manifest.json")); detail != "" {
		add("backup_manifest", "fail", detail)
		return checks
	}
	add("backup_manifest", "pass", fmt.Sprintf("schema v%d, %d entries, created %s", manifest.SchemaVersion, len(manifest.Entries), manifest.CreatedAt.Format(time.RFC3339)))
	if !dbOK || !instanceOK {
		add("backup_crosscheck", "fail", "cannot cross-check; fix the pieces above first")
		return checks
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "instance.id")); string(b) != manifest.InstanceID {
		add("backup_crosscheck", "fail", "manifest_mismatch:instance_id: manifest and instance.id disagree")
		return checks
	}
	if failures := service.CrossCheckManifest(dir, manifest); len(failures) > 0 {
		shown := failures
		if len(shown) > 3 {
			shown = append(append([]string{}, shown[:3]...), fmt.Sprintf("... %d more", len(failures)-3))
		}
		add("backup_crosscheck", "fail", strings.Join(shown, "; "))
		return checks
	}
	add("backup_crosscheck", "pass", fmt.Sprintf("%d version(s) consistent", len(manifest.Entries)))
	return checks
}

// checkInstanceFile reports existence, shape and privacy; hexShape marks
// instance.id as hex (identity), while the key is raw bytes.
func checkInstanceFile(add func(string, string, string), name, path string, size int, dbExists bool) bool {
	b, e := os.ReadFile(path)
	if e != nil {
		if dbExists {
			add(name, "fail", name+"_missing_restore_required: restore from backup; do not regenerate on an existing database")
		} else {
			add(name, "info", "not initialized; the first serve creates it")
		}
		return false
	}
	if len(b) != size {
		add(name, "fail", fmt.Sprintf("%s_invalid: expected %d bytes", name, size))
		return false
	}
	if name == "instance_id" {
		if _, decodeErr := hex.DecodeString(string(b)); decodeErr != nil {
			add(name, "fail", "instance_id_invalid: expected 48 hex characters")
			return false
		}
	}
	if detail := privateMode(path); detail != "" {
		add(name, "fail", detail)
		return false
	}
	add(name, "pass", "")
	return true
}

func privateMode(path string) string {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return "state_file_unavailable: regular file required"
	}
	if info.Mode().Perm()&0077 != 0 {
		return "state_file_not_private: chmod 0600 required"
	}
	return ""
}

func platformStorage(dbPath string) (status string, disabled bool, errCode string) {
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if e != nil {
		return "", false, "metadata_db_unreadable"
	}
	defer db.Close()
	rows, e := db.Query("SELECT kind, disabled, status FROM connections WHERE id = 'platform'")
	if e != nil {
		return "", false, "metadata_db_unreadable"
	}
	defer rows.Close()
	if !rows.Next() {
		return "", false, ""
	}
	var kind string
	if e := rows.Scan(&kind, &disabled, &status); e != nil || kind != "s3" {
		return "", false, "platform_storage_invalid"
	}
	if e := rows.Err(); e != nil {
		return "", false, "metadata_db_unreadable"
	}
	return status, disabled, ""
}

// issuerDiscovery fetches the OIDC discovery document and compares its issuer
// field; no signature, key or account-level validation happens here.
func issuerDiscovery(issuer string) (string, bool) {
	if u, e := url.Parse(issuer); e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return "issuer_invalid: https origin (or loopback http for offline development) required", false
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Get(strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration")
	if e != nil {
		return "issuer_unreachable: check the issuer URL and network path", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("issuer_discovery_failed: http_status=%d", resp.StatusCode), false
	}
	var document struct {
		Issuer string `json:"issuer"`
	}
	if e := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&document); e != nil || document.Issuer == "" {
		return "issuer_discovery_invalid: expected a JSON document with an issuer field", false
	}
	if strings.TrimSuffix(document.Issuer, "/") != strings.TrimSuffix(issuer, "/") {
		return "issuer_mismatch: the discovered issuer differs from the configured one", false
	}
	return "discovery issuer matches", true
}
