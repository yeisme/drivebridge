package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func findCheck(r doctorReport, name string) *doctorCheck {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			return &r.Checks[i]
		}
	}
	return nil
}

func mustPass(t *testing.T, r doctorReport, name string) {
	t.Helper()
	if c := findCheck(r, name); c == nil || c.Status != "pass" {
		t.Fatalf("%s should pass: %+v", name, c)
	}
}
func mustFail(t *testing.T, r doctorReport, name string) *doctorCheck {
	t.Helper()
	c := findCheck(r, name)
	if c == nil || c.Status != "fail" {
		t.Fatalf("%s should fail: %+v", name, c)
	}
	return c
}

// openFixtureState initializes a real state directory through service.Open and
// closes the service immediately, leaving key/identity/database on disk.
func openFixtureState(t *testing.T, dir string) {
	t.Helper()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := service.Open(service.Config{State: dir, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	if e := s.DB.Exec("INSERT INTO connections (id, owner, kind, name, secret, disabled, status) VALUES ('platform', '', 's3', '平台存储', x'00', 0, 'verified')").Error; e != nil {
		t.Fatal(e)
	}
	s.Close()
}

func discoveryServer(t *testing.T, issuer func() string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("unexpected probe path %q", r.URL.Path)
		}
		w.Write([]byte(`{"issuer":"` + issuer() + `"}`))
	}))
}

func TestDoctorReadyOnConsistentState(t *testing.T) {
	dir := t.TempDir()
	openFixtureState(t, dir)
	web := t.TempDir()
	if e := os.WriteFile(filepath.Join(web, "index.html"), []byte("<html></html>"), 0644); e != nil {
		t.Fatal(e)
	}
	var srv *httptest.Server
	srv = discoveryServer(t, func() string { return srv.URL })
	defer srv.Close()

	r := buildDoctorReport(doctorConfig{State: dir, WebDir: web, Issuer: srv.URL})
	for _, c := range r.Checks {
		if c.Status == "fail" {
			t.Fatalf("unexpected fail %s: %s", c.Name, c.Detail)
		}
	}
	mustPass(t, r, "state_consistency")
	mustPass(t, r, "platform_storage")
	mustPass(t, r, "oidc_issuer")
	mustPass(t, r, "web_dir")
}

func TestDoctorFreshStateIsInfo(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	r := buildDoctorReport(doctorConfig{State: dir})
	if c := findCheck(r, "state_consistency"); c == nil || c.Status != "info" {
		t.Fatalf("fresh state should be info: %+v", c)
	}
	if c := findCheck(r, "platform_storage"); c != nil {
		t.Fatalf("platform_storage should be skipped without a database: %+v", c)
	}
}

func TestDoctorFreshMissingDirectory(t *testing.T) {
	r := buildDoctorReport(doctorConfig{State: filepath.Join(t.TempDir(), "absent")})
	mustFail(t, r, "state_dir")
	if c := findCheck(r, "platform_storage"); c != nil {
		t.Fatalf("platform_storage should be skipped: %+v", c)
	}
}

func TestDoctorKeyLossRequiresRestore(t *testing.T) {
	dir := t.TempDir()
	openFixtureState(t, dir)
	if e := os.Remove(filepath.Join(dir, "instance.key")); e != nil {
		t.Fatal(e)
	}
	r := buildDoctorReport(doctorConfig{State: dir})
	mustFail(t, r, "instance_key")
	cs := mustFail(t, r, "state_consistency")
	if cs.Detail != "instance_key_missing_restore_required: restore instance.key from the secret backup; regenerating would orphan encrypted credentials" {
		t.Fatalf("unexpected guidance: %s", cs.Detail)
	}
}

func TestDoctorIdentityCorruption(t *testing.T) {
	dir := t.TempDir()
	openFixtureState(t, dir)
	if e := os.WriteFile(filepath.Join(dir, "instance.id"), []byte("zz"), 0600); e != nil {
		t.Fatal(e)
	}
	r := buildDoctorReport(doctorConfig{State: dir})
	mustFail(t, r, "instance_id")
}

func TestDoctorIssuerChecks(t *testing.T) {
	srv := discoveryServer(t, func() string { return "https://other-issuer.example.com" })
	defer srv.Close()

	// Mismatching discovery issuer fails only its own check.
	c := mustFail(t, buildDoctorReport(doctorConfig{Issuer: srv.URL}), "oidc_issuer")
	if c.Detail != "issuer_mismatch: the discovered issuer differs from the configured one" {
		t.Fatalf("unexpected detail: %s", c.Detail)
	}
	// Invalid shape never performs a network call.
	if c := mustFail(t, buildDoctorReport(doctorConfig{Issuer: "ftp://issuer.example.com"}), "oidc_issuer"); c.Detail == "" {
		t.Fatal("expected shape failure detail")
	}
}

func TestDoctorWebDir(t *testing.T) {
	dir := t.TempDir()
	openFixtureState(t, dir)
	r := buildDoctorReport(doctorConfig{State: dir, WebDir: t.TempDir()})
	mustFail(t, r, "web_dir")
}

// backupFixtureState builds a state directory with one encrypted space and
// closes the service, leaving a restorable state on disk.
func backupFixtureState(t *testing.T, dir string) {
	t.Helper()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := service.Open(service.Config{State: dir, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	space := service.Space{ID: service.ID(), Owner: service.ID(), Name: "Vault", Encrypted: true, WrappedDEK: []byte{1, 2, 3, 4}}
	if e := s.DB.Create(&space).Error; e != nil {
		t.Fatal(e)
	}
	if e := s.CheckpointMetadata(); e != nil {
		t.Fatal(e)
	}
	s.Close()
}

func TestDoctorReportsEncryptionKeyDistribution(t *testing.T) {
	dir := t.TempDir()
	backupFixtureState(t, dir)
	report := buildDoctorReport(doctorConfig{State: dir})
	c := findCheck(report, "content_encryption")
	if c == nil || c.Status != "pass" || !strings.Contains(c.Detail, "v1:1") {
		t.Fatalf("key distribution missing: %+v", c)
	}
	// Without encrypted spaces the check stays silent.
	plain := t.TempDir()
	openFixtureState(t, plain)
	report = buildDoctorReport(doctorConfig{State: plain})
	if c := findCheck(report, "content_encryption"); c != nil {
		t.Fatalf("plain state reported encryption: %+v", c)
	}
}

func TestDoctorBackupCrossChecks(t *testing.T) {
	state := t.TempDir()
	backupFixtureState(t, state)
	dir := filepath.Join(t.TempDir(), "backup")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"metadata.sqlite", "instance.id"} {
		b, e := os.ReadFile(filepath.Join(state, name))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	report := buildDoctorReport(doctorConfig{State: state, Backup: dir})
	mustFail(t, report, "backup_manifest")
	if e := service.WriteBackupManifest(dir); e != nil {
		t.Fatal(e)
	}
	report = buildDoctorReport(doctorConfig{State: state, Backup: dir})
	for _, name := range []string{"backup_dir", "backup_metadata", "backup_instance_id", "backup_manifest", "backup_crosscheck"} {
		mustPass(t, report, name)
	}
	// Drift between manifest and shipped metadata fails the cross-check.
	manifest, e := service.ReadBackupManifest(dir)
	if e != nil {
		t.Fatal(e)
	}
	manifest.Entries = append(manifest.Entries, service.ManifestEntry{VersionID: service.ID(), ObjectKey: "ghost", SHA256: service.Hash("ghost"), Size: 1})
	body, _ := json.Marshal(manifest)
	if e = os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0600); e != nil {
		t.Fatal(e)
	}
	report = buildDoctorReport(doctorConfig{State: state, Backup: dir})
	detail := mustFail(t, report, "backup_crosscheck").Detail
	if !strings.Contains(detail, "manifest_mismatch:version_unknown") {
		t.Fatalf("cross-check detail unexpected: %s", detail)
	}
	// A manifest pinned to another instance is refused.
	manifest.Entries = nil
	manifest.InstanceID = service.ID()
	body, _ = json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0600)
	report = buildDoctorReport(doctorConfig{State: state, Backup: dir})
	mustFail(t, report, "backup_crosscheck")
}

func TestDoctorWarnsWhenNoLoginableIdentity(t *testing.T) {
	dir := t.TempDir()
	openFixtureState(t, dir)

	// No user keys and no OIDC issuer: the deployment would lock everyone out.
	r := buildDoctorReport(doctorConfig{State: dir})
	c := mustFail(t, r, "login_identities")
	if !strings.Contains(c.Detail, "no_loginable_user_keys") {
		t.Fatalf("detail must name the recovery: %s", c.Detail)
	}

	// An issuer alone keeps the report honest instead of failing.
	r = buildDoctorReport(doctorConfig{State: dir, Issuer: "https://issuer.example"})
	found := false
	for _, c := range r.Checks {
		if c.Name == "login_identities" {
			found = true
			if c.Status != "info" {
				t.Fatalf("with issuer expected info, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("login_identities check missing")
	}

	// One live admin key satisfies the gate.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, "metadata.sqlite")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO user_keys (id, owner, name, token_hash, admin, created_at, expires_at, revoked) VALUES ('k1', 'u1', 'ops', 'hash', 1, datetime('now'), '', 0)")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	r = buildDoctorReport(doctorConfig{State: dir})
	mustPass(t, r, "login_identities")
}
