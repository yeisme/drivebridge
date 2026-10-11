package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capture swaps stdout around fn and returns what was printed.
func capture(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = old
	body, _ := io.ReadAll(r)
	return string(body), runErr
}

func runCLI(args ...string) error {
	root := newRootCommand(&cli{})
	root.SetArgs(args)
	return root.Execute()
}

func TestConfigInitShowAndUnknownKey(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "server.yaml")
	t.Setenv("DRIVEBRIDGE_SERVER_CONFIG", "")
	t.Setenv("DRIVEBRIDGE_SERVER_STATE", "")

	if _, err := capture(t, func() error { return runCLI("config", "init", "--config", target) }); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("init must write 0600: %v %v", info, err)
	}
	if _, err := capture(t, func() error { return runCLI("config", "init", "--config", target) }); err == nil {
		t.Fatal("init must refuse to overwrite")
	}

	body, err := capture(t, func() error { return runCLI("config", "show", "--config", target, "--json") })
	if err != nil || !strings.Contains(body, `"source.state":"file"`) {
		t.Fatalf("show must report file source: %v %s", err, body)
	}
	if strings.Contains(body, "client_secret") && strings.Contains(body, "password") {
		t.Fatal("no secret values in show output")
	}

	// Unknown keys fail loudly, for every command.
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("state: /s\nenabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err = runCLI("config", "show", "--config", bad)
	if err == nil || !strings.Contains(err.Error(), "unknown keys: enabled") {
		t.Fatalf("unknown key must fail: %v", err)
	}

	// The doctor preflight is read-only and reports the config source.
	state := filepath.Join(dir, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server2.yaml"), []byte("state: "+state+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err = capture(t, func() error { return runCLI("doctor", "--config", filepath.Join(dir, "server2.yaml")) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "config_file") || !strings.Contains(body, "server2.yaml") {
		t.Fatalf("doctor must surface the config file: %s", body)
	}
}
