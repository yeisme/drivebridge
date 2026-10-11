package service_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

// fpDriveRebuild walks one rebuild operation through its phases until it
// reaches a terminal state (active or a refusal that repeats for the whole
// budget). It fails the test when the rebuild never settles.
func fpDriveRebuild(t *testing.T, s *service.Service, operationID string, budget time.Duration) (map[string]any, error) {
	t.Helper()
	deadline := time.Now().Add(budget)
	var lastErr error
	for time.Now().Before(deadline) {
		view, e := s.AdvanceProjectionRebuild(operationID, 500)
		if e != nil {
			lastErr = e
			// A refusal may be transient while the live worker catches up.
			if e.Error() == "projection_not_caught_up" {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return view, e
		}
		if phase, _ := view["phase"].(string); phase == "active" {
			return view, nil
		}
	}
	if lastErr == nil {
		lastErr = context.DeadlineExceeded
	}
	return nil, lastErr
}

// Rework MAJOR-1: a space with trash/restore/purge history must still be able
// to switch projection generations. The verifying digest compares the live
// generation (event-materialized revisions) with the rebuild generation
// (backfill from the file table plus replay), so every trash/restore must move
// File.Updated in the same transaction as its event revision, restores must
// clear tombstones, and purged resources must keep their tombstone rows.
func TestFPTeamRuntimeRebuildAfterTrashRestorePurgeHistory(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	fpRuntimeUpload(t, f, h, "keep.txt")
	zombie := fpRuntimeUpload(t, f, h, "zombie.txt")
	victim := fpRuntimeUpload(t, f, h, "victim.txt")
	fpDrainRuntime(t, f)
	// Trash then restore: the projection must end with a live row again.
	if e := f.Service.Trash(p, zombie.FileID, false); e != nil {
		t.Fatal(e)
	}
	fpDrainRuntime(t, f)
	if e := f.Service.Trash(p, zombie.FileID, true); e != nil {
		t.Fatal(e)
	}
	fpDrainRuntime(t, f)
	// Purge: the resource keeps only its tombstone projection row.
	if e := f.Service.Trash(p, victim.FileID, false); e != nil {
		t.Fatal(e)
	}
	fpDrainRuntime(t, f)
	if e := f.Service.Purge(context.Background(), p, victim.FileID); e != nil {
		t.Fatal(e)
	}
	fpDrainRuntime(t, f)
	admin := service.Principal{UserID: h.user.ID, Admin: true}
	start, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "")
	if e != nil {
		t.Fatal(e)
	}
	operationID, _ := start["operation_id"].(string)
	view, e := fpDriveRebuild(t, f.Service, operationID, 20*time.Second)
	if e != nil {
		t.Fatalf("rebuild never switched with trash/restore/purge history: %v", e)
	}
	if view["phase"] != "active" || view["ready"] != true {
		t.Fatalf("switch landed wrong: %+v", view)
	}
	// The restored resource is live in the new generation, not tombstoned.
	var zombieRow service.ProjectionRow
	if e := f.Service.DB.Where("projection = ? AND generation = ? AND resource_id = ?", "file_catalogue", view["generation"], zombie.FileID).First(&zombieRow).Error; e != nil {
		t.Fatal(e)
	}
	if zombieRow.Tombstone {
		t.Fatal("restore did not clear the tombstone in the rebuilt generation")
	}
	// The purged resource keeps its tombstone row in both generations.
	var victimTombs int64
	f.Service.DB.Model(&service.ProjectionRow{}).Where("projection = ? AND resource_id = ? AND tombstone = ?", "file_catalogue", victim.FileID, true).Count(&victimTombs)
	if victimTombs < 2 {
		t.Fatalf("purged resource lost tombstone rows across generations: %d", victimTombs)
	}
	// The live generation must also show the restore (tombstone cleared).
	var liveZombie service.ProjectionRow
	if e := f.Service.DB.Where("projection = ? AND generation = ? AND resource_id = ?", "file_catalogue", "GEN_0", zombie.FileID).First(&liveZombie).Error; e != nil {
		t.Fatal(e)
	}
	if liveZombie.Tombstone {
		t.Fatal("restore left a sticky tombstone in the live generation")
	}
}

// Rework MAJOR-2: two owner scopes with interleaved post-freeze events must
// both be replayed into the rebuild generation. A single mutable generation
// watermark skips the second scope's earlier events forever.
func TestFPTeamRuntimeRebuildDualScopeInterleaved(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	fpRuntimeUpload(t, f, alice, "a1.txt")
	fpRuntimeUpload(t, f, bob, "b1.txt")
	fpRuntimeUpload(t, f, alice, "a2.txt")
	fpRuntimeUpload(t, f, bob, "b2.txt")
	fpDrainRuntime(t, f)
	admin := service.Principal{UserID: alice.user.ID, Admin: true}
	start, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "")
	if e != nil {
		t.Fatal(e)
	}
	operationID, _ := start["operation_id"].(string)
	// Drive freeze -> backfill first, so the interleaved events below land
	// strictly after the file-table snapshot and must come from the replay.
	for _, phase := range []string{"backfill", "catching_up"} {
		view, e := f.Service.AdvanceProjectionRebuild(operationID, 500)
		if e != nil {
			t.Fatal(e)
		}
		if view["phase"] != phase {
			t.Fatalf("phase transition wrong: want %s got %v", phase, view["phase"])
		}
	}
	// Interleaved post-backfill events across both scopes.
	fpRuntimeUpload(t, f, alice, "a3.txt")
	fpRuntimeUpload(t, f, bob, "b3.txt")
	fpRuntimeUpload(t, f, alice, "a4.txt")
	view, e := fpDriveRebuild(t, f.Service, operationID, 20*time.Second)
	if e != nil {
		t.Fatalf("dual-scope rebuild never switched: %v", e)
	}
	if view["phase"] != "active" || view["ready"] != true {
		t.Fatalf("switch landed wrong: %+v", view)
	}
	// Every scope's latest event is reflected in the new generation.
	for _, row := range []struct{ scope, path string }{
		{alice.user.ID, "a4.txt"},
		{bob.user.ID, "b3.txt"},
	} {
		var files []service.File
		if e := f.Service.DB.Where("owner = ? AND path = ?", row.scope, row.path).Find(&files).Error; e != nil || len(files) != 1 {
			t.Fatalf("fixture file missing for %s: %v", row.path, e)
		}
		var genRow service.ProjectionRow
		if e := f.Service.DB.Where("projection = ? AND generation = ? AND resource_id = ?", "file_catalogue", view["generation"], files[0].ID).First(&genRow).Error; e != nil {
			t.Fatalf("scope %s event skipped by rebuild: %v", row.scope, e)
		}
		if genRow.Revision != files[0].Updated.UnixNano() {
			t.Fatalf("scope %s row revision drifted: %d vs %d", row.scope, genRow.Revision, files[0].Updated.UnixNano())
		}
	}
	// Both scopes' checkpoints moved to the new generation.
	for _, scope := range []string{alice.user.ID, bob.user.ID} {
		var checkpoint service.ProjectionCheckpoint
		f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", scope).First(&checkpoint)
		if checkpoint.Generation != view["generation"] {
			t.Fatalf("scope %s checkpoint not switched: %+v", scope, checkpoint)
		}
	}
}

// Rework MAJOR-3: importing a snapshot whose tables are empty (a fresh
// deployment with no uploads yet) must succeed instead of failing on empty
// slice inserts, and the import stays all-or-nothing.
func TestFPTeamRuntimeImportEmptySnapshot(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	// A logged-in space with no uploads: files, versions, operations and
	// grants are all empty in the export.
	var buf bytes.Buffer
	if e := f.Service.ExportMetadata(&buf); e != nil {
		t.Fatal(e)
	}
	snapshot := buf.Bytes()
	target := t.TempDir()
	targetDB, e := gorm.Open(sqlite.Open(filepath.Join(target, "metadata.sqlite")), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e := targetDB.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{}); e != nil {
		t.Fatal(e)
	}
	result, e := f.Service.ImportMetadata(bytes.NewReader(snapshot), targetDB)
	if e != nil {
		t.Fatalf("empty-table snapshot import failed: %v", e)
	}
	digest := result["imported"].(map[string]int64)
	if digest["files"] != 0 || digest["spaces"] == 0 {
		t.Fatalf("empty import digest wrong: %+v", digest)
	}
	verify, e := f.Service.VerifyMetadata(bytes.NewReader(snapshot), bytes.NewReader(snapshot))
	if e != nil || verify["matching"] != true {
		t.Fatalf("verify rejected identical empty snapshots: %v %+v", e, verify)
	}
	_ = h
}

// Rework MAJOR-4: the metadata export must carry the sealed connection
// credentials, the import must restore them, and the verification digest must
// notice their absence instead of reporting a false match.
func TestFPTeamRuntimeImportConnectionCredentials(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	fpRuntimeUpload(t, f, h, "cred.txt")
	fpDrainRuntime(t, f)
	var source service.Connection
	if e := f.Service.DB.First(&source, "id = ?", "platform").Error; e != nil {
		t.Fatal(e)
	}
	if len(source.Secret) == 0 {
		t.Fatal("fixture platform connection has no sealed secret")
	}
	var buf bytes.Buffer
	if e := f.Service.ExportMetadata(&buf); e != nil {
		t.Fatal(e)
	}
	snapshot := buf.Bytes()
	if !bytes.Contains(snapshot, []byte(`"sealed_secret":`)) {
		t.Fatal("export dropped the sealed connection credential")
	}
	target := t.TempDir()
	targetDB, e := gorm.Open(sqlite.Open(filepath.Join(target, "metadata.sqlite")), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e := targetDB.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{}); e != nil {
		t.Fatal(e)
	}
	if _, e := f.Service.ImportMetadata(bytes.NewReader(snapshot), targetDB); e != nil {
		t.Fatal(e)
	}
	var imported service.Connection
	if e := targetDB.First(&imported, "id = ?", "platform").Error; e != nil {
		t.Fatal(e)
	}
	if len(imported.Secret) == 0 {
		t.Fatal("import dropped the sealed connection credential; connections would be unusable")
	}
	if !bytes.Equal(imported.Secret, source.Secret) {
		t.Fatal("imported sealed credential differs from the source bytes")
	}
	verify, e := f.Service.VerifyMetadata(bytes.NewReader(snapshot), bytes.NewReader(snapshot))
	if e != nil || verify["matching"] != true {
		t.Fatalf("verify rejected identical snapshots: %v %+v", e, verify)
	}
	if d, _ := verify["source"].(map[string]int64); d["connection_credentials"] == 0 {
		t.Fatalf("verify digest ignores connection credentials: %+v", verify)
	}
}
