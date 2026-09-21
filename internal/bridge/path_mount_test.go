package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rconfig "github.com/rclone/rclone/fs/config"
	configfile "github.com/rclone/rclone/fs/config/configfile"
)

func loadRcloneConf(t *testing.T, body string) {
	t.Helper()
	installRcloneConfig(t)
	path := rconfig.GetConfigPath()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	configfile.Install()
	if err := rconfig.SetConfigPath(path); err != nil {
		t.Fatal(err)
	}
}

func TestRcloneMountSource(t *testing.T) {
	got, err := RcloneMountSource("minio", "bucket/pinax")
	if err != nil || got != "minio:bucket/pinax" {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = RcloneMountSource("minio", "/bucket/pinax/")
	if err != nil || got != "minio:bucket/pinax" {
		t.Fatalf("trim got %q %v", got, err)
	}
	if _, err := RcloneMountSource("minio", "../secret"); err == nil || err.Error() != "invalid_space" {
		t.Fatalf("traversal err=%v", err)
	}
	if _, err := RcloneMountSource("bad:name", "bucket"); err == nil {
		t.Fatal("expected invalid remote")
	}
}

func TestPrepareSpaceContentRootPrefersAlias(t *testing.T) {
	dir := t.TempDir()
	loadRcloneConf(t, "[docs]\ntype = alias\nremote = "+dir+"\n")
	root, err := PrepareSpaceContentRoot(Space{ID: "docs", Remote: "docs", Backend: "generic"}, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if root.Adapter != "pinax-vault" || root.Path != filepath.Clean(dir) {
		t.Fatalf("%+v", root)
	}
	root.Close()
}

func TestPrepareSpaceContentRootS3UsesRcloneMount(t *testing.T) {
	loadRcloneConf(t, "[minio]\ntype = s3\nprovider = Minio\n")
	old := rcloneExec
	t.Cleanup(func() { rcloneExec = old })
	var mountSrc, mountTarget string
	var umounted bool
	rcloneExec = func(args ...string) error {
		switch args[0] {
		case "mount":
			mountSrc, mountTarget = args[1], args[2]
			if err := os.MkdirAll(filepath.Join(args[2], "notes"), 0o755); err != nil {
				return err
			}
			return nil
		case "umount":
			umounted = true
			return nil
		default:
			return errors.New("unexpected rclone " + args[0])
		}
	}
	state := t.TempDir()
	root, err := PrepareSpaceContentRoot(Space{ID: "pinax-vault", Remote: "minio", Backend: "generic", RootID: "bucket/pinax"}, state, "/tmp/rclone.conf")
	if err != nil {
		t.Fatal(err)
	}
	if root.Adapter != "pinax-vault-rclone" {
		t.Fatalf("adapter=%s", root.Adapter)
	}
	if mountSrc != "minio:bucket/pinax" {
		t.Fatalf("source=%s", mountSrc)
	}
	want := filepath.Join(state, "mnt", "pinax-vault")
	if mountTarget != want || root.Path != want {
		t.Fatalf("target=%s path=%s", mountTarget, root.Path)
	}
	root.Close()
	if !umounted {
		t.Fatal("expected umount")
	}
}

func TestPrepareSpaceContentRootMissingRclone(t *testing.T) {
	loadRcloneConf(t, "[minio]\ntype = s3\n")
	old := rcloneExec
	t.Cleanup(func() { rcloneExec = old })
	rcloneExec = func(args ...string) error {
		return errors.New("mount_path_adapter_missing")
	}
	_, err := PrepareSpaceContentRoot(Space{ID: "pinax-vault", Remote: "minio", Backend: "generic", RootID: "b/p"}, t.TempDir(), "")
	if err == nil || err.Error() != "mount_path_adapter_missing" {
		t.Fatalf("err=%v", err)
	}
}

func TestPrepareSpaceContentRootRejectsNonS3Generic(t *testing.T) {
	loadRcloneConf(t, "[gd]\ntype = drive\n")
	called := false
	old := rcloneExec
	t.Cleanup(func() { rcloneExec = old })
	rcloneExec = func(args ...string) error {
		called = true
		return nil
	}
	_, err := PrepareSpaceContentRoot(Space{ID: "g", Remote: "gd", Backend: "generic"}, t.TempDir(), "")
	if err == nil || err.Error() != "mount_path_adapter_missing" || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestPrepareSpaceContentRootS3PassesConfigAndDoesNotLogSecrets(t *testing.T) {
	loadRcloneConf(t, "[minio]\ntype = s3\n")
	old := rcloneExec
	t.Cleanup(func() { rcloneExec = old })
	var args []string
	rcloneExec = func(a ...string) error {
		args = append([]string{}, a...)
		if a[0] == "mount" {
			return os.MkdirAll(a[2], 0o700)
		}
		return nil
	}
	cfg := filepath.Join(t.TempDir(), "rclone.conf")
	root, err := PrepareSpaceContentRoot(Space{ID: "s3notes", Remote: "minio", Backend: "generic", RootID: "bucket/p"}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--config "+cfg) || !strings.Contains(joined, "--vfs-cache-mode writes") || !strings.Contains(joined, "--daemon") {
		t.Fatalf("args=%v", args)
	}
	if strings.Contains(joined, "secret") || strings.Contains(joined, "access_key") {
		t.Fatalf("secrets in args: %v", args)
	}
	root.Close()
}
