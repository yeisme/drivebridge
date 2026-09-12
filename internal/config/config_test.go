package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStrictUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "drivebridge.yaml")
	write(t, bad, "state: /tmp/x\nenabled: true\nscaena_rl: \"\"\n")
	if _, err := Load(bad); err == nil {
		t.Fatal("unknown keys must be rejected")
	}
	good := filepath.Join(dir, "drivebridge.json")
	write(t, good, `{"state": "/tmp/x", "local_root": "/tmp/root"}`)
	file, err := Load(good)
	if err != nil {
		t.Fatal(err)
	}
	if file.Values["state"] != "/tmp/x" || file.Values["local_root"] != "/tmp/root" {
		t.Fatalf("values: %v", file.Values)
	}
	if _, ok := file.Values["rclone_config"]; ok {
		t.Fatal("unset key must be absent")
	}
}

func TestDiscoverOrder(t *testing.T) {
	dir := t.TempDir()
	cwd := filepath.Join(dir, "cwd")
	home := filepath.Join(dir, "home")
	for _, d := range []string{cwd, filepath.Join(home, "drivebridge")} {
		if err := os.MkdirAll(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DRIVEBRIDGE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", home)
	homeFile := filepath.Join(home, "drivebridge", "config.yaml")
	write(t, homeFile, "state: from-home\n")

	restore := chdir(t, cwd)
	defer restore()

	if got := Discover(""); got != homeFile {
		t.Fatalf("home fallback expected, got %q", got)
	}
	if got := Discover("/explicit/path"); got != "/explicit/path" {
		t.Fatalf("explicit wins, got %q", got)
	}
	t.Setenv("DRIVEBRIDGE_CONFIG", "/from/env")
	if got := Discover(""); got != "/from/env" {
		t.Fatalf("env wins, got %q", got)
	}
	t.Setenv("DRIVEBRIDGE_CONFIG", "")
	cwdFile := filepath.Join(cwd, "drivebridge.toml")
	write(t, cwdFile, "state = \"from-cwd\"\n")
	if got := Discover(""); got != cwdFile {
		t.Fatalf("cwd beats home, got %q", got)
	}
	// Extension order within the same directory.
	write(t, filepath.Join(cwd, "drivebridge.yaml"), "state: from-yaml\n")
	if got := Discover(""); got != filepath.Join(cwd, "drivebridge.yaml") {
		t.Fatalf("yaml beats toml, got %q", got)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(old) }
}

func TestResolvePrecedence(t *testing.T) {
	file := File{Path: "/c", Values: map[string]string{"state": "from-file", "rclone_config": "from-file-rc"}}
	t.Setenv("DRIVEBRIDGE_STATE", "from-env")

	if v, s := Resolve(file, "state", "from-flag", true, "def"); v != "from-flag" || s != SourceFlag {
		t.Fatalf("flag should win: %v %v", v, s)
	}
	if v, s := Resolve(file, "state", "", false, "def"); v != "from-env" || s != SourceEnv {
		t.Fatalf("env should beat file: %v %v", v, s)
	}
	t.Setenv("DRIVEBRIDGE_STATE", "")
	if v, s := Resolve(file, "state", "", false, "def"); v != "from-file" || s != SourceFile {
		t.Fatalf("file should beat default: %v %v", v, s)
	}
	if v, s := Resolve(file, "rclone_config", "", false, "def"); v != "from-file-rc" || s != SourceFile {
		t.Fatalf("file value: %v %v", v, s)
	}
	if v, s := Resolve(file, "local_root", "", false, "def-root"); v != "def-root" || s != SourceDefault {
		t.Fatalf("default: %v %v", v, s)
	}
	// An explicitly cleared flag never hides a file value.
	if v, s := Resolve(file, "rclone_config", "", true, "def"); v != "from-file-rc" || s != SourceFile {
		t.Fatalf("empty explicit flag falls through: %v %v", v, s)
	}
}

func TestResolveAllAndTemplate(t *testing.T) {
	file := File{Path: "/c", Values: map[string]string{"state": "/s"}}
	e := ResolveAll(file, map[string]Flag{"local_root": {Value: "/flag/root", Set: true}}, map[string]string{"state": "/def"})
	if e.Values["state"] != "/s" || e.Sources["state"] != SourceFile {
		t.Fatalf("state: %v %v", e.Values, e.Sources)
	}
	if e.Values["local_root"] != "/flag/root" || e.Sources["local_root"] != SourceFlag {
		t.Fatalf("local_root: %v %v", e.Values, e.Sources)
	}
	if tpl := Template(); !contains(tpl, "state:") || !contains(tpl, "scaena_token_file:") {
		t.Fatal("template must cover every key")
	}
	for _, key := range Keys {
		if !contains(Template(), key+":") {
			t.Fatalf("template missing %s", key)
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
