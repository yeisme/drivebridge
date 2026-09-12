package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// syncTestWorkdir isolates each fixture's journals from the user cache.
var syncTestWorkdir string

// syncFixture builds a service whose generic space pairs a local directory
// with a persistent alias-backed remote directory.
func syncFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("sync matrix exercises the rclone bisync engine over local fs pairs")
	}
	local := t.TempDir()
	remoteDir := t.TempDir()
	const remote = "sync-fixture"
	config.Data().SetValue(remote, "type", "alias")
	config.Data().SetValue(remote, "remote", remoteDir)
	t.Cleanup(func() { config.Data().DeleteSection(remote) })
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := New(state, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.AddSpace(Space{ID: "paired", Remote: remote, Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	syncTestWorkdir = filepath.Join(state, "bisync")
	return s, local, remoteDir
}

func syncWrite(t *testing.T, dir, name, content string, age time.Duration) {
	t.Helper()
	syncWriteStamp(t, dir, name, content, time.Now().Add(-age))
}

// syncWriteStamp uses one absolute stamp so files mirrored to both sides
// carry identical metadata for the engine's equality comparison.
func syncWriteStamp(t *testing.T, dir, name, content string, stamp time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if !stamp.IsZero() {
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
}

func syncCount(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			out[filepath.Base(path)] = string(mustRead(t, path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func syncWait(t *testing.T, s *Service, space string, opt SyncOptions) (SyncReport, error) {
	t.Helper()
	if opt.Workdir == "" {
		opt.Workdir = syncTestWorkdir
	}
	report, err := s.Sync(context.Background(), space, opt)
	// bisync keeps a lock file per pair; give a following run a moment.
	time.Sleep(50 * time.Millisecond)
	return report, err
}

func TestSyncFirstRunRequiresExplicitResync(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncWrite(t, local, "local-only.txt", "l", 0)
	syncWrite(t, remoteDir, "remote-only.txt", "r", 0)
	_, err := syncWait(t, s, "paired", SyncOptions{})
	if err == nil || err.Error() != "resync_required" {
		t.Fatalf("first run without resync: %v", err)
	}
	if len(syncCount(t, local)) != 1 || len(syncCount(t, remoteDir)) != 1 {
		t.Fatal("refused first run changed either side")
	}
}

// syncMirror writes the same bytes with the same stamp to both sides.
func syncMirror(t *testing.T, local, remoteDir, name, content string, age time.Duration) {
	t.Helper()
	stamp := time.Now().Add(-age)
	syncWriteStamp(t, local, name, content, stamp)
	syncWriteStamp(t, remoteDir, name, content, stamp)
}

func TestSyncResyncEstablishesBaselineAndPropagates(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncMirror(t, local, remoteDir, "shared.txt", "v1", 2*time.Hour)
	syncWrite(t, local, "local-new.txt", "l", 0)
	syncWrite(t, remoteDir, "remote-new.txt", "r", 0)
	report, err := syncWait(t, s, "paired", SyncOptions{Resync: true})
	if err != nil {
		t.Fatalf("resync: %v", err)
	}
	if report.Status != "completed" || !report.Resync {
		t.Fatalf("resync report: %+v", report)
	}
	localFiles, remoteFiles := syncCount(t, local), syncCount(t, remoteDir)
	for _, name := range []string{"shared.txt", "local-new.txt", "remote-new.txt"} {
		if _, ok := localFiles[name]; !ok {
			t.Fatalf("local missing %s after resync", name)
		}
		if _, ok := remoteFiles[name]; !ok {
			t.Fatalf("remote missing %s after resync", name)
		}
	}
	// A subsequent dry run plans without touching either side.
	syncWrite(t, local, "pending.txt", "p", 0)
	report, err = syncWait(t, s, "paired", SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !report.DryRun || !contains(report.Additions1, "pending.txt") {
		t.Fatalf("dry-run plan: %+v", report)
	}
	if _, ok := syncCount(t, remoteDir)["pending.txt"]; ok {
		t.Fatal("dry run copied a file")
	}
	// The real run propagates the addition.
	if _, err = syncWait(t, s, "paired", SyncOptions{}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if _, ok := syncCount(t, remoteDir)["pending.txt"]; !ok {
		t.Fatal("addition not propagated")
	}
	// Remote-side updates flow back to local.
	syncWrite(t, remoteDir, "pending.txt", "p2", 0)
	if _, err = syncWait(t, s, "paired", SyncOptions{}); err != nil {
		t.Fatalf("third run: %v", err)
	}
	if string(mustRead(t, filepath.Join(local, "pending.txt"))) != "p2" {
		t.Fatal("remote update not propagated to local")
	}
}

func TestSyncConflictKeepsBothCopiesNamedByOperation(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncMirror(t, local, remoteDir, "shared.txt", "base", 3*time.Hour)
	// The engine refuses pairs where literally every file changed; a stable
	// anchor keeps the conflict run honest.
	syncMirror(t, local, remoteDir, "anchor.txt", "stable", 3*time.Hour)
	if _, err := syncWait(t, s, "paired", SyncOptions{Resync: true}); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	syncWrite(t, local, "shared.txt", "local-edit", 0)
	syncWrite(t, remoteDir, "shared.txt", "remote-edit", 0)
	report, err := syncWait(t, s, "paired", SyncOptions{})
	if err != nil {
		t.Fatalf("conflict run: %v", err)
	}
	if !contains(report.Conflicts, "shared.txt") || report.OperationID == "" {
		t.Fatalf("conflict report: %+v", report)
	}
	suffixedLocal, suffixedRemote := 0, 0
	for name, content := range syncCount(t, local) {
		if strings.Contains(name, ".sync-"+report.OperationID) && content == "remote-edit" {
			suffixedLocal++
		}
	}
	for name, content := range syncCount(t, remoteDir) {
		if strings.Contains(name, ".sync-"+report.OperationID) && content == "local-edit" {
			suffixedRemote++
		}
	}
	if suffixedLocal == 0 || suffixedRemote == 0 {
		t.Fatalf("keep-both copies missing: local=%d remote=%d", suffixedLocal, suffixedRemote)
	}
}

func TestSyncDeletionCapAbortsAndOverrideCompletes(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncMirror(t, local, remoteDir, "a.txt", "a", 4*time.Hour)
	syncMirror(t, local, remoteDir, "b.txt", "b", 4*time.Hour)
	syncMirror(t, local, remoteDir, "c.txt", "c", 4*time.Hour)
	syncMirror(t, local, remoteDir, "d.txt", "d", 4*time.Hour)
	if _, err := syncWait(t, s, "paired", SyncOptions{Resync: true}); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.Remove(filepath.Join(local, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := syncWait(t, s, "paired", SyncOptions{})
	if err == nil || !strings.Contains(err.Error(), "sync_deletion_cap_exceeded") || !strings.Contains(err.Error(), "75%") || !strings.Contains(err.Error(), "25%") {
		t.Fatalf("deletion cap: %v", err)
	}
	if len(syncCount(t, remoteDir)) != 4 {
		t.Fatal("capped run deleted remote files")
	}
	// An explicit override continues.
	if _, err = syncWait(t, s, "paired", SyncOptions{MaxDelete: 100}); err != nil {
		t.Fatalf("override run: %v", err)
	}
	if remaining := syncCount(t, remoteDir); len(remaining) != 1 {
		t.Fatalf("override did not propagate deletions: %v", remaining)
	}
}

func TestSyncUntrustedJournalDemandsResync(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncMirror(t, local, remoteDir, "shared.txt", "base", 3*time.Hour)
	if _, err := syncWait(t, s, "paired", SyncOptions{Resync: true}); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	// Corrupt the engine journal: the next run must refuse with an explicit
	// resync demand instead of guessing.
	if err := corruptBisyncJournals(t, s, "paired", local, remoteDir); err != nil {
		t.Skipf("journal not found where expected: %v", err)
	}
	_, err := syncWait(t, s, "paired", SyncOptions{})
	if err == nil || err.Error() != "resync_required" {
		t.Fatalf("corrupt journal accepted: %v", err)
	}
}

func TestSyncOperationRecordedAndEventsEmitted(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncWrite(t, local, "x.txt", "x", 0)
	syncWrite(t, remoteDir, "y.txt", "y", 0)
	// Warm the feed so the sync event lands strictly after a nonzero cursor
	// (a zero cursor only reports the current position).
	s.emit("paired", "sync_probe", "", "warmup")
	_, cursorBefore, err := s.Changes("paired", 0)
	if err != nil {
		t.Fatal(err)
	}
	report, err := syncWait(t, s, "paired", SyncOptions{Resync: true})
	if err != nil {
		t.Fatal(err)
	}
	op, err := s.Get(report.OperationID)
	if err != nil || op.Kind != "sync" || op.Status != "completed" {
		t.Fatalf("operation receipt: %+v err=%v", op, err)
	}
	events, _, err := s.Changes("paired", cursorBefore)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.Kind == "operation_completed" && ev.Detail == report.OperationID {
			found = true
		}
	}
	if !found {
		t.Fatal("sync completion missing from the changes feed")
	}
}

func TestSyncRegistersRemoteFilesAsStableReferences(t *testing.T) {
	s, local, remoteDir := syncFixture(t)
	syncMirror(t, local, remoteDir, "kept.txt", "k", 3*time.Hour)
	syncWrite(t, remoteDir, "remote-side.txt", "r", 0)
	syncWrite(t, local, "local-side.txt", "l", 0)
	if _, err := syncWait(t, s, "paired", SyncOptions{Resync: true}); err != nil {
		t.Fatalf("resync: %v", err)
	}
	hits, err := s.Search("paired", "remote-side", "")
	if err != nil || len(hits) != 1 {
		t.Fatalf("search after sync: %+v err=%v", hits, err)
	}
	if _, err = s.Stat(context.Background(), hits[0].Ref); err != nil {
		t.Fatalf("sync-created reference not verifiable: %v", err)
	}
}

func TestSyncMatrixDeletedRemoteReferenceKeepsFixedError(t *testing.T) {
	s, p, local, _ := setup(t)
	op := uploadFixture(t, s, local)
	var row File
	if err := s.DB.First(&row, "ref = ?", op.FileRef).Error; err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	delete(p.files, row.ItemID)
	p.mu.Unlock()
	if _, err := s.Stat(context.Background(), op.FileRef); err == nil || err.Error() != "remote_not_found" {
		t.Fatalf("deleted remote reference: %v", err)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// corruptBisyncJournals breaks the engine's listing files for this pair.
func corruptBisyncJournals(t *testing.T, s *Service, space, local, remoteDir string) error {
	t.Helper()
	ctx := context.Background()
	f1, err := fs.NewFs(ctx, filepath.ToSlash(local))
	if err != nil {
		return err
	}
	p, err := s.Space(space)
	if err != nil {
		return err
	}
	provider, ok := s.providerFor(p).(remoteFsProvider)
	if !ok {
		return errors.New("sync_unsupported")
	}
	f2, err := provider.Fs(ctx, p)
	if err != nil {
		return err
	}
	basePath := bilib.BasePath(ctx, syncTestWorkdir, f1, f2)
	for _, listing := range []string{basePath + ".path1.lst", basePath + ".path2.lst"} {
		if _, err := os.Stat(listing); err != nil {
			return err
		}
		if err := os.WriteFile(listing, []byte("corrupt"), 0600); err != nil {
			return err
		}
	}
	return nil
}
