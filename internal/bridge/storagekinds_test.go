package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rconfig "github.com/rclone/rclone/fs/config"
	configfile "github.com/rclone/rclone/fs/config/configfile"
)

func installRcloneConfig(t *testing.T) {
	t.Helper()
	configfile.Install()
	if e := rconfig.SetConfigPath(filepath.Join(t.TempDir(), "rclone.conf")); e != nil {
		t.Fatal(e)
	}
}

func TestStorageCatalogStaysHonest(t *testing.T) {
	for _, k := range StorageKinds() {
		if k.Kind == "" || k.Display == "" || len(k.Requires) == 0 {
			t.Fatalf("incomplete kind entry: %+v", k)
		}
		if k.Kind != "onedrive" {
			if k.Backend != "generic" {
				t.Fatalf("non-onedrive kind %s must be generic", k.Kind)
			}
			if k.Capabilities["direct_url"] || k.Capabilities["reference_handoff"] {
				t.Fatalf("generic kind %s claims a privileged capability", k.Kind)
			}
			// server_side_copy must be absent for generic kinds: it is
			// backend-dependent, and a flat false would be dishonest.
			if _, ok := k.Capabilities["server_side_copy"]; ok {
				t.Fatalf("generic kind %s must not claim server_side_copy", k.Kind)
			}
			if len(k.Capabilities) != 3 {
				t.Fatalf("generic kind %s caps must mirror the truth table exactly: %v", k.Kind, k.Capabilities)
			}
		}
	}
	// Remote-based kinds keep --remote-path optional (empty = remote root);
	// only s3 (bucket semantics) and local (the target dir) require it.
	requiresPath := map[string]bool{"s3": true, "local": true}
	for _, k := range StorageKinds() {
		want := requiresPath[k.Kind]
		usedPath := false
		for _, r := range k.Requires {
			if strings.HasPrefix(r, "--remote-path") {
				usedPath = true
			}
		}
		if usedPath != want {
			t.Fatalf("kind %s --remote-path required=%v, want %v", k.Kind, usedPath, want)
		}
	}
}

func TestEnsureAndProbe(t *testing.T) {
	installRcloneConfig(t)
	dir := t.TempDir()

	if e := EnsureAliasRemote("db-local-x", dir+"/"); e != nil {
		t.Fatal(e)
	}
	// Trailing-slash variants reuse the same alias instead of remote_exists.
	if !AliasRemoteMatches("db-local-x", dir) {
		t.Fatal("cleaned alias comparison must reuse /dir vs /dir/")
	}
	if e := EnsureAliasRemote("db-local-x", dir); e == nil || e.Error() != "remote_exists" {
		t.Fatalf("duplicate alias: %v", e)
	}
	if e := EnsureWebDAVRemote("db-openlist-x", "http://127.0.0.1:5244/dav", "media", "s3cret"); e != nil {
		t.Fatal(e)
	}
	stored := rconfig.GetValue("db-openlist-x", "pass")
	if stored == "" || stored == "s3cret" {
		t.Fatal("webdav password must be stored obscured")
	}
	if rconfig.GetValue("db-openlist-x", "vendor") != "other" || rconfig.GetValue("db-openlist-x", "user") != "media" {
		t.Fatal("webdav section incomplete")
	}
	if e := EnsureWebDAVRemote("db-openlist-x", "http://other/dav", "media", "x"); e == nil || e.Error() != "remote_exists" {
		t.Fatalf("conflicting webdav remote: %v", e)
	}
	// Same endpoint+user but rotated password must NOT silently reuse stale
	// credentials.
	if WebDAVRemoteMatches("db-openlist-x", "http://127.0.0.1:5244/dav", "media", "rotated") {
		t.Fatal("password rotation must not match a stale remote")
	}
	if !WebDAVRemoteMatches("db-openlist-x", "http://127.0.0.1:5244/dav", "media", "s3cret") {
		t.Fatal("matching credentials must be reusable")
	}

	// Rollback removes exactly the auto-created section.
	DeleteRemote("db-openlist-x")
	if rconfig.GetValue("db-openlist-x", "type") != "" {
		t.Fatal("DeleteRemote left the section behind")
	}

	if e := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0600); e != nil {
		t.Fatal(e)
	}
	res, err := ProbeStorage(context.Background(), "local", "db-local-x", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reachable || res.Entries != 1 || len(res.Sample) != 1 || res.Sample[0] != "a.txt" {
		t.Fatalf("probe mismatch: %+v", res)
	}
	if res.Capabilities["direct_url"] || res.Capabilities["reference_handoff"] {
		t.Fatal("probe must report transfer-only capabilities for generic remotes")
	}
	if res.Writability != "unverified (the first upload verifies)" {
		t.Fatal("probe must not claim write access")
	}
	// The onedrive probe keeps the privileged truth table.
	res, err = ProbeStorage(context.Background(), "onedrive", "db-local-x", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Capabilities["direct_url"] || !res.Capabilities["reference_handoff"] || !res.Capabilities["server_side_copy"] {
		t.Fatalf("onedrive probe caps must stay privileged: %v", res.Capabilities)
	}
	if _, err = ProbeStorage(context.Background(), "local", "missing-remote", ""); err == nil || err.Error() != "probe_failed" {
		t.Fatalf("unknown remote: %v", err)
	}
	if _, err = ProbeStorage(context.Background(), "local", "bad:name", ""); err == nil || err.Error() != "invalid_space" {
		t.Fatalf("colon remote: %v", err)
	}
}
