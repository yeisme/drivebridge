package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
)

// genericFixture builds a state with an rclone "local" backend remote named
// nas, standing in for any rclone-supported cloud provider offline.
func genericFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	conf := filepath.Join(dir, "rclone.conf")
	// The alias backend pins the remote root to a temp directory, standing in
	// for any cloud provider exactly like a configured remote would.
	body := "[nas]\ntype = alias\nremote = " + filepath.Join(dir, "remote-files") + "\n"
	if e := os.WriteFile(conf, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	configfile.Install()
	if e := config.SetConfigPath(conf); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(dir, "state")
	if e := os.Mkdir(state, 0700); e != nil {
		t.Fatal(e)
	}
	local := filepath.Join(dir, "local")
	remoteRoot := filepath.Join(dir, "remote-files")
	for _, d := range []string{local, remoteRoot} {
		if e := os.Mkdir(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	s, e := New(state, NewOneDrive(), true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s, local, remoteRoot
}

func waitCompleted(t *testing.T, s *Service, id string) Operation {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		op, e := s.Get(id)
		if e != nil {
			t.Fatal(e)
		}
		if op.Status == "completed" {
			return op
		}
		if op.Status == "failed" || op.Status == "cancelled" {
			t.Fatalf("operation %s: %s %s", id, op.Status, op.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("operation did not complete in time")
	return Operation{}
}

func TestGenericSpaceRoundtrip(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", RootID: "", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	content := "generic backend payload 0123456789"
	source := filepath.Join(local, "a.bin")
	if e := os.WriteFile(source, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(context.Background(), "nas", "", source, "a.bin", "k1")
	if e != nil {
		t.Fatal(e)
	}
	op = waitCompleted(t, s, op.ID)
	if op.FileRef == "" || !strings.HasPrefix(op.FileRef, "drivebridge://file/") {
		t.Fatalf("unexpected ref %q", op.FileRef)
	}
	// The remote root received the operation-prefixed object.
	matches, _ := filepath.Glob(filepath.Join(remoteRoot, "*-a.bin"))
	if len(matches) != 1 {
		t.Fatalf("expected uploaded object, got %v", matches)
	}
	// List + stat see the stable reference with a stable version.
	files, e := s.List(context.Background(), "nas", "")
	if e != nil || len(files) != 1 {
		t.Fatalf("list: %v %v", e, files)
	}
	stat, e := s.Stat(context.Background(), op.FileRef)
	if e != nil || stat.SHA256 == "" {
		t.Fatalf("stat: %v %+v", e, stat)
	}
	// Download to a fresh local name and compare content.
	target := filepath.Join(local, "back.bin")
	dop, e := s.Download(context.Background(), op.FileRef, target, "k2")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, dop.ID)
	b, _ := os.ReadFile(target)
	if string(b) != content {
		t.Fatal("downloaded content mismatch")
	}
	// Direct URL access and reference handoff stay OneDrive-only.
	if _, e := s.providerFor(Space{Backend: "generic"}).Access(context.Background(), Space{}, File{}); e == nil || e.Error() != "access_unsupported" {
		t.Fatalf("access should be unsupported: %v", e)
	}
}

func TestGenericSpaceRootPath(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	nested := filepath.Join(remoteRoot, "refs")
	if e := os.MkdirAll(nested, 0700); e != nil {
		t.Fatal(e)
	}
	if e := s.AddSpace(Space{ID: "nas-refs", Remote: "nas", Backend: "generic", RootID: "refs", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	p, _ := s.Space("nas-refs")
	if p.DriveID != "rclone:nas" {
		t.Fatalf("drive label = %q", p.DriveID)
	}
	if e := os.WriteFile(filepath.Join(local, "r.bin"), []byte("nested"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(context.Background(), "nas-refs", "", filepath.Join(local, "r.bin"), "r.bin", "k")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, op.ID)
	if _, e := os.Stat(filepath.Join(nested, op.ID+"-r.bin")); e != nil {
		t.Fatalf("object should land inside the pinned root path: %v", e)
	}
}

func TestSymlinkPolicy(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	outside := filepath.Join(t.TempDir(), "secret.bin")
	if e := os.WriteFile(outside, []byte("outside"), 0600); e != nil {
		t.Fatal(e)
	}
	escaping := filepath.Join(local, "escape.bin")
	if e := os.Symlink(outside, escaping); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Upload(context.Background(), "nas", "", escaping, "escape.bin", "k"); e == nil || e.Error() != "symlink_rejected" {
		t.Fatalf("default must reject symlinks: %v", e)
	}
	// Even with opt-in, a link escaping the allowed root stays blocked by
	// os.Root containment.
	s.DB.Model(&Space{}).Where("id = ?", "nas").Update("follow_symlinks", true)
	if _, e := s.Upload(context.Background(), "nas", "", escaping, "escape.bin", "k-escape"); e == nil {
		t.Fatal("escaping symlink must stay rejected even with opt-in")
	}
	// Opt-in materializes an in-root link as a plain content copy.
	target := filepath.Join(local, "real.bin")
	if e := os.WriteFile(target, []byte("materialized"), 0600); e != nil {
		t.Fatal(e)
	}
	inside := filepath.Join(local, "link.bin")
	if e := os.Symlink("real.bin", inside); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(context.Background(), "nas", "", inside, "link.bin", "k2")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, op.ID)
	b, _ := os.ReadFile(filepath.Join(remoteRoot, op.ID+"-link.bin"))
	if string(b) != "materialized" {
		t.Fatalf("materialized copy mismatch: %q", b)
	}
}

func TestGenericHandoffGated(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20, Project: "p", Actor: "a"}); e != nil {
		t.Fatal(e)
	}
	s.Handoff = true
	if e := os.WriteFile(filepath.Join(local, "x.bin"), []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(context.Background(), "nas", "", filepath.Join(local, "x.bin"), "x.bin", "k")
	if e != nil {
		t.Fatal(e)
	}
	op = waitCompleted(t, s, op.ID)
	_, e = s.Call(context.Background(), "prepare_reference", map[string]any{"file_ref": op.FileRef})
	if e == nil || e.Error() != "handoff_not_available" {
		t.Fatalf("prepare_reference must reject generic spaces: %v", e)
	}
}

func TestAddSpaceBackendValidation(t *testing.T) {
	s, local, _ := genericFixture(t)
	base := Space{ID: "x", Remote: "nas", LocalRoot: local, MaxBytes: 1}
	if e := s.AddSpace(withBackend(base, "weird")); e == nil || e.Error() != "invalid_space" {
		t.Fatalf("unknown backend must be rejected: %v", e)
	}
	escape := base
	escape.Backend = "generic"
	escape.RootID = "../escape"
	if e := s.AddSpace(escape); e == nil || e.Error() != "invalid_space" {
		t.Fatalf("escaping remote path must be rejected: %v", e)
	}
	missingRemote := base
	missingRemote.Backend = "generic"
	missingRemote.Remote = "ghost"
	if _, e := NewGeneric().List(context.Background(), missingRemote, ""); e == nil || e.Error() != "backend_unavailable" {
		t.Fatalf("unconfigured remote must fail with backend_unavailable: %v", e)
	}
}

func withBackend(p Space, backend string) Space { p.Backend = backend; return p }

func TestWorkspaceDirectories(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := s.Mkdir(ctx, "nas", "refs/2026"); e != nil {
		t.Fatal(e)
	}
	// mkdir is idempotent for an existing folder.
	if e := s.Mkdir(ctx, "nas", "refs/2026"); e != nil {
		t.Fatal(e)
	}
	// Upload lands inside the directory and list resolves it by path.
	if e := os.WriteFile(filepath.Join(local, "d.bin"), []byte("dir payload"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(ctx, "nas", "refs/2026", filepath.Join(local, "d.bin"), "d.bin", "k1")
	if e != nil {
		t.Fatal(e)
	}
	op = waitCompleted(t, s, op.ID)
	if _, e := os.Stat(filepath.Join(remoteRoot, "refs", "2026", op.ID+"-d.bin")); e != nil {
		t.Fatalf("object should land inside the directory: %v", e)
	}
	files, e := s.List(ctx, "nas", "refs/2026")
	if e != nil || len(files) != 1 || !strings.HasSuffix(files[0].Name, "-d.bin") {
		t.Fatalf("list by path: %v %+v", e, files)
	}
	root, e := s.List(ctx, "nas", "")
	if e != nil || len(root) != 0 {
		t.Fatalf("root stays directory-free: %v %+v", e, root)
	}
	// Download from the directory reference round-trips.
	target := filepath.Join(local, "back.bin")
	dop, e := s.Download(ctx, op.FileRef, target, "k2")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, dop.ID)
	if b, _ := os.ReadFile(target); string(b) != "dir payload" {
		t.Fatal("directory download mismatch")
	}
	// Traversal and root mkdir are rejected.
	if e := s.Mkdir(ctx, "nas", "../escape"); e == nil || e.Error() != "path_not_allowed" {
		t.Fatalf("traversal must be rejected: %v", e)
	}
	if e := s.Mkdir(ctx, "nas", ""); e == nil || e.Error() != "path_not_allowed" {
		t.Fatalf("root mkdir must be rejected: %v", e)
	}
	if _, e := s.List(ctx, "nas", "/abs"); e == nil || e.Error() != "path_not_allowed" {
		t.Fatalf("absolute path must be rejected: %v", e)
	}
}
