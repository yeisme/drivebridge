package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestSoftDeleteLifecycle(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := os.WriteFile(filepath.Join(local, "t.bin"), []byte("trashable"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(ctx, "nas", "", filepath.Join(local, "t.bin"), "t.bin", "k")
	if e != nil {
		t.Fatal(e)
	}
	op = waitCompleted(t, s, op.ID)
	ref := op.FileRef

	// Trash keeps the reference queryable, blocks consumption, keeps bytes.
	if _, e = s.Trash(ctx, ref); e != nil {
		t.Fatal(e)
	}
	stat, e := s.Stat(ctx, ref)
	if e != nil || stat.Trashed == nil {
		t.Fatalf("trashed stat: %v %+v", e, stat)
	}
	if _, e = s.Download(ctx, ref, filepath.Join(local, "nope.bin"), "k2"); e == nil || e.Error() != "file_trashed" {
		t.Fatalf("download must reject trashed: %v", e)
	}
	matches, _ := filepath.Glob(filepath.Join(remoteRoot, ".drivebridge-trash", "*", "*-t.bin"))
	if len(matches) != 1 {
		t.Fatalf("object should sit in dated trash: %v", matches)
	}

	// Purge refuses un-trashed refs and removes trashed ones permanently.
	if e = s.Purge(ctx, "drivebridge://file/none"); e == nil {
		t.Fatal("unknown ref must fail")
	}
	fresh, e := s.Upload(ctx, "nas", "", filepath.Join(local, "t.bin"), "t2.bin", "k2")
	if e != nil {
		t.Fatal(e)
	}
	fresh = waitCompleted(t, s, fresh.ID)
	if e = s.Purge(ctx, fresh.FileRef); e == nil || e.Error() != "file_not_trashed" {
		t.Fatalf("purge is two-step: %v", e)
	}
	if _, e = s.Trash(ctx, fresh.FileRef); e != nil {
		t.Fatal(e)
	}
	if e = s.Purge(ctx, fresh.FileRef); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Stat(ctx, fresh.FileRef); e == nil || e.Error() != "reference_not_found" {
		t.Fatalf("purged ref must vanish: %v", e)
	}

	// Restore returns the original reference without overwriting.
	restored, e := s.Restore(ctx, ref)
	if e != nil || restored.Trashed != nil {
		t.Fatalf("restore: %v %+v", e, restored)
	}
	if b, _ := os.ReadFile(filepath.Join(remoteRoot, restored.Name)); string(b) != "trashable" {
		t.Fatal("restored bytes mismatch")
	}
	target := filepath.Join(local, "back.bin")
	dop, e := s.Download(ctx, ref, target, "k3")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, dop.ID)
}

func TestSearchAndChangesFeed(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	// Zero cursor: current position, no history replay.
	_, cursor, e := s.Changes("nas", 0)
	if e != nil || cursor != 0 {
		t.Fatalf("empty feed: %v %d", e, cursor)
	}
	for _, name := range []string{"hero-a.png", "hero-b.png", "side-c.png"} {
		if e := os.WriteFile(filepath.Join(local, name), []byte(name), 0600); e != nil {
			t.Fatal(e)
		}
		op, e := s.Upload(ctx, "nas", "", filepath.Join(local, name), name, "k-"+name)
		if e != nil {
			t.Fatal(e)
		}
		waitCompleted(t, s, op.ID)
	}
	// Search by case-insensitive substring over the index.
	hits, e := s.Search("nas", "HERO", "")
	if e != nil || len(hits) != 2 {
		t.Fatalf("search: %v %+v", e, hits)
	}
	// Capture the live cursor before further mutations.
	_, live, e := s.Changes("nas", 0)
	if e != nil {
		t.Fatal(e)
	}
	// Trashed files drop out of search.
	trashedRef := hits[0].Ref
	if _, e = s.Trash(ctx, trashedRef); e != nil {
		t.Fatal(e)
	}
	hits, e = s.Search("nas", "hero", "")
	if e != nil || len(hits) != 1 {
		t.Fatalf("search after trash: %v %+v", e, hits)
	}
	// The feed is subscription-style: events strictly after a live cursor.
	if e := os.WriteFile(filepath.Join(local, "late.png"), []byte("late"), 0600); e != nil {
		t.Fatal(e)
	}
	late, e := s.Upload(ctx, "nas", "", filepath.Join(local, "late.png"), "late.png", "k-late")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, late.ID)
	events, cursor2, e := s.Changes("nas", live)
	if e != nil {
		t.Fatal(e)
	}
	kinds := map[string]int{}
	for _, ev := range events {
		kinds[ev.Kind]++
	}
	if kinds["file_added"] != 1 || kinds["file_trashed"] != 1 || kinds["operation_completed"] < 1 {
		t.Fatalf("feed kinds: %v", kinds)
	}
	if _, _, e = s.Changes("nas", cursor2+1); e == nil || e.Error() != "cursor_ahead" {
		t.Fatalf("ahead cursor must fail: %v", e)
	}
	// Restore surfaces as its own event.
	if _, e = s.Restore(ctx, trashedRef); e != nil {
		t.Fatal(e)
	}
	events, _, e = s.Changes("nas", cursor2)
	if e != nil || len(events) != 1 || events[0].Kind != "file_restored" {
		t.Fatalf("restore event: %v %+v", e, events)
	}
}

func TestGenericInboxListsRecentOperationsNewestFirst(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(local, "i.bin")
	if e := os.WriteFile(source, []byte("inbox payload"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(context.Background(), "nas", "", source, "i.bin", "k1")
	if e != nil {
		t.Fatal(e)
	}
	op = waitCompleted(t, s, op.ID)
	dop, e := s.Download(context.Background(), op.FileRef, filepath.Join(local, "i-out.bin"), "k2")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, dop.ID)
	rows, e := s.Call(context.Background(), "inbox", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	ops, ok := rows.([]Operation)
	if !ok || len(ops) < 2 {
		t.Fatalf("inbox rows: %T %+v", rows, rows)
	}
	if ops[0].ID != dop.ID || ops[1].ID != op.ID {
		t.Fatalf("expected newest-first [download, upload], got %s %s", ops[0].ID, ops[1].ID)
	}
}

func TestGenericInboxFailsHonestOnClosedState(t *testing.T) {
	// Bypass genericFixture: the closed-state probe must not touch the shared
	// rclone config singleton that later tests depend on.
	s := &Service{}
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "state.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&Operation{}); e != nil {
		t.Fatal(e)
	}
	s.DB = db
	if sqlDB, e := db.DB(); e == nil {
		sqlDB.Close()
	}
	if _, e := s.Call(context.Background(), "inbox", map[string]any{}); e == nil || e.Error() != "state_unavailable" {
		t.Fatalf("expected state_unavailable, got %v", e)
	}
}

func TestGenericUploadRejectsExisting(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(remoteRoot, "taken.txt"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	p, err := s.Space("nas")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.providerFor(p).Upload(context.Background(), p, "taken.txt", strings.NewReader("new"), 3)
	if err == nil || err.Error() != "target_conflict" {
		t.Fatalf("overwrite: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(remoteRoot, "taken.txt"))
	if err != nil || string(got) != "old" {
		t.Fatalf("existing file changed: %q %v", got, err)
	}
}
