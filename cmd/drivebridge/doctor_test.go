package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func findCheck(r doctorReport, name string) *doctorCheck {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			return &r.Checks[i]
		}
	}
	return nil
}

func TestDoctorRemoteReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected probe path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	token := filepath.Join(dir, "agent.token")
	writeFile(t, token, "scoped-token\n", 0600)
	root := t.TempDir()

	report := buildDoctorReport(doctorConfig{RemoteURL: srv.URL, RemoteTokenFile: token, LocalRoot: root})
	for _, c := range report.Checks {
		if c.Status == "fail" {
			t.Fatalf("unexpected fail %s: %s", c.Name, c.Detail)
		}
	}
	if report.Mode != "remote" {
		t.Fatalf("mode = %q", report.Mode)
	}
	if c := findCheck(report, "remote_reachable"); c == nil || c.Status != "pass" {
		t.Fatalf("remote_reachable missing or not passing: %+v", c)
	}
}

func TestDoctorRemoteFailures(t *testing.T) {
	dir := t.TempDir()
	openToken := filepath.Join(dir, "open.token")
	writeFile(t, openToken, "scoped-token\n", 0644)

	report := buildDoctorReport(doctorConfig{RemoteURL: "https://files.example.com/path", RemoteTokenFile: openToken, LocalRoot: "relative/path"})
	for _, name := range []string{"remote_url", "remote_token_file", "local_root"} {
		if c := findCheck(report, name); c == nil || c.Status != "fail" {
			t.Fatalf("%s should fail: %+v", name, c)
		}
	}
	// Reachability is only probed when the endpoint shape is valid.
	report = buildDoctorReport(doctorConfig{RemoteURL: "https://files.example.com/path", RemoteTokenFile: openToken, LocalRoot: dir})
	if c := findCheck(report, "remote_reachable"); c != nil {
		t.Fatalf("reachability must be skipped for an invalid endpoint shape: %+v", c)
	}
	// A valid shape with no listener fails fast instead of hanging.
	report = buildDoctorReport(doctorConfig{RemoteURL: "https://127.0.0.1:1", RemoteTokenFile: openToken, LocalRoot: dir})
	if c := findCheck(report, "remote_reachable"); c == nil || c.Status != "fail" {
		t.Fatalf("unreachable endpoint must fail: %+v", c)
	}
}

func TestDoctorLocalReport(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	rclone := filepath.Join(dir, "rclone.conf")
	writeFile(t, rclone, "[RCLONE]\n[one]\ntype = onedrive\n", 0600)

	report := buildDoctorReport(doctorConfig{State: state, RcloneConfig: rclone})
	if c := findCheck(report, "state_dir"); c == nil || c.Status != "pass" {
		t.Fatalf("state_dir: %+v", c)
	}
	if c := findCheck(report, "spaces"); c == nil || c.Status != "fail" || c.Detail == "" {
		t.Fatalf("fresh state must fail spaces with guidance: %+v", c)
	}
	if c := findCheck(report, "rclone_config"); c == nil || c.Status != "pass" {
		t.Fatalf("rclone_config: %+v", c)
	}

	// An exposed rclone config must fail without leaking values.
	writeFile(t, rclone, "[one]\ntype = onedrive\n", 0644)
	report = buildDoctorReport(doctorConfig{State: state, RcloneConfig: rclone})
	if c := findCheck(report, "rclone_config"); c == nil || c.Status != "fail" {
		t.Fatalf("open rclone config must fail: %+v", c)
	}
}
