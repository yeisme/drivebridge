package managedstate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofrs/flock"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type migrationFixture struct {
	ID           string `gorm:"primaryKey"`
	OperationKey string
	Status       string
}

func fixture(t *testing.T, kind string) (string, Plan, []byte) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("candidate fence currently has Linux-only admission")
	}
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	name, _ := databaseName(kind)
	db, err := gorm.Open(sqlite.Open(filepath.Join(path, name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&migrationFixture{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&migrationFixture{ID: "original-operation", OperationKey: "original-key", Status: "unknown"}).Error; err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	if err = connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Join(path, name), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(path, name))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Prepare(path, kind, "original-migration")
	if err != nil {
		t.Fatal(err)
	}
	return path, plan, before
}

func verifyFence(t *testing.T, path string, plan Plan, before []byte) State {
	t.Helper()
	state, err := Inspect(path, plan.Kind)
	if err != nil || state.Layout != "fenced" || state.Plan != plan {
		t.Fatalf("unexpected fenced state: %#v %v", state, err)
	}
	copy, err := os.ReadFile(state.DatabasePath)
	if err != nil || !bytes.Equal(before, copy) {
		t.Fatalf("original database bytes changed: %v", err)
	}
	name, _ := databaseName(plan.Kind)
	markerBefore, err := os.ReadFile(filepath.Join(path, name))
	if err != nil {
		t.Fatal(err)
	}
	old, err := gorm.Open(sqlite.Open(filepath.Join(path, name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err == nil {
		// SQLite opens lazily; both legacy services migrate their tables
		// before constructing a service or recovering workers.
		err = old.AutoMigrate(&migrationFixture{})
		connection, _ := old.DB()
		connection.Close()
	}
	if err == nil {
		t.Fatal("legacy SQLite startup accepted the marker")
	}
	markerAfter, err := os.ReadFile(filepath.Join(path, name))
	if err != nil || !bytes.Equal(markerBefore, markerAfter) {
		t.Fatalf("legacy attempt changed marker: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(state.DatabasePath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	defer connection.Close()
	var original migrationFixture
	if err = db.First(&original, "id = ?", "original-operation").Error; err != nil || original.OperationKey != "original-key" || original.Status != "unknown" {
		t.Fatalf("original recovery identity lost: %#v %v", original, err)
	}
	return state
}

func TestFencePreservesOriginalDatabaseAndMigrationIdentity(t *testing.T) {
	for _, kind := range []string{"cli", "server"} {
		t.Run(kind, func(t *testing.T) {
			path, plan, before := fixture(t, kind)
			if _, err := Install(path, plan); err != nil {
				t.Fatal(err)
			}
			state := verifyFence(t, path, plan, before)
			if replay, err := Install(path, plan); err != nil || replay != state {
				t.Fatalf("original migration replay failed: %#v %v", replay, err)
			}
			other := plan
			other.MigrationID = "replacement-key"
			if _, err := Install(path, other); err == nil || err.Error() != "state_migration_conflict" {
				t.Fatalf("replacement migration key accepted: %v", err)
			}
		})
	}
}

func TestFenceRefusesBusyChangedAndUnsafeState(t *testing.T) {
	for _, scenario := range []string{"locked", "wal", "shm", "journal", "source-change", "snapshot-conflict", "snapshot-alias", "source-alias", "lock-alias", "missing-lock"} {
		t.Run(scenario, func(t *testing.T) {
			path, plan, before := fixture(t, "cli")
			want := "state_checkpoint_required"
			switch scenario {
			case "locked":
				lock := flock.New(filepath.Join(path, "process.lock"))
				if err := lock.Lock(); err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
				want = "state_locked"
			case "wal", "shm", "journal":
				if err := os.WriteFile(filepath.Join(path, "state.sqlite-"+scenario), []byte("preserve sidecar"), 0600); err != nil {
					t.Fatal(err)
				}
			case "source-change":
				plan.SourceDigest = "sha256:" + strings.Repeat("0", 64)
				want = "state_changed"
			case "snapshot-conflict":
				if err := os.Mkdir(filepath.Join(path, snapshotDir), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, snapshotDir, "state.sqlite"), []byte("existing recovery artifact"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "state_recovery_required"
			case "snapshot-alias":
				if err := os.Symlink(t.TempDir(), filepath.Join(path, snapshotDir)); err != nil {
					t.Fatal(err)
				}
				want = "state_file_invalid"
			case "source-alias", "lock-alias", "missing-lock":
				name := "process.lock"
				want = "state_lock_unverifiable"
				if scenario == "source-alias" {
					name, want = "state.sqlite", "state_file_invalid"
				}
				if err := os.Rename(filepath.Join(path, name), filepath.Join(path, name+".original")); err != nil {
					t.Fatal(err)
				}
				if scenario != "missing-lock" {
					if err := os.Symlink(name+".original", filepath.Join(path, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := Install(path, plan); err == nil || err.Error() != want {
				t.Fatalf("want %s, got %v", want, err)
			}
			after, err := os.ReadFile(filepath.Join(path, "state.sqlite"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal changed the original database: %v", err)
			}
		})
	}
}

func TestFenceCrashProcess(t *testing.T) {
	if os.Getenv("DRIVEBRIDGE_FENCE_CRASH_HELPER") != "1" {
		t.Skip("subprocess checkpoint helper")
	}
	var plan Plan
	if json.Unmarshal([]byte(os.Getenv("DRIVEBRIDGE_FENCE_PLAN")), &plan) != nil {
		os.Exit(81)
	}
	_, err := install(os.Getenv("DRIVEBRIDGE_FENCE_PATH"), plan, func(phase string) {
		if phase == os.Getenv("DRIVEBRIDGE_FENCE_PHASE") {
			os.Exit(77)
		}
	})
	if err != nil {
		os.Exit(82)
	}
	os.Exit(83)
}

func TestFenceProcessCrashRecoveryNeverLeavesMissingLegacyPath(t *testing.T) {
	for _, phase := range []string{"snapshot_written", "snapshot_durable", "marker_durable", "marker_published", "commit_durable"} {
		t.Run(phase, func(t *testing.T) {
			path, plan, before := fixture(t, "cli")
			payload, _ := json.Marshal(plan)
			cmd := exec.Command(os.Args[0], "-test.run=^TestFenceCrashProcess$")
			cmd.Env = append(os.Environ(), "DRIVEBRIDGE_FENCE_CRASH_HELPER=1", "DRIVEBRIDGE_FENCE_PATH="+path, "DRIVEBRIDGE_FENCE_PHASE="+phase, "DRIVEBRIDGE_FENCE_PLAN="+string(payload))
			output, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 77 {
				t.Fatalf("checkpoint process did not exit at phase: %v %s", err, output)
			}
			info, err := os.Lstat(filepath.Join(path, "state.sqlite"))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("crash left a hole in the legacy database path")
			}
			state, err := Inspect(path, "cli")
			if err != nil {
				t.Fatal(err)
			}
			want := "legacy"
			if phase == "marker_published" || phase == "commit_durable" {
				want = "fenced"
			}
			if state.Layout != want {
				t.Fatalf("after %s: got %s", phase, state.Layout)
			}
			if _, err = Install(path, plan); err != nil {
				t.Fatalf("restart with original plan: %v", err)
			}
			verifyFence(t, path, plan, before)
		})
	}
}

func TestFenceInspectDoesNotRepairMissingSnapshot(t *testing.T) {
	path, plan, _ := fixture(t, "cli")
	state, err := Install(path, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(state.DatabasePath, state.DatabasePath+".preserved"); err != nil {
		t.Fatal(err)
	}
	if _, err = Inspect(path, "cli"); err == nil || err.Error() != "state_recovery_required" {
		t.Fatalf("missing snapshot must not initialize a database: %v", err)
	}
	if _, err = os.Stat(state.DatabasePath); !os.IsNotExist(err) {
		t.Fatal("inspection created a replacement database")
	}
}

func TestFenceRejectsPlanFromAnotherStateWithIdenticalBytes(t *testing.T) {
	first, plan, before := fixture(t, "cli")
	second, _, _ := fixture(t, "cli")
	if err := os.WriteFile(filepath.Join(second, "state.sqlite"), before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(second, plan); err == nil || err.Error() != "state_changed" {
		t.Fatalf("cross-state migration plan accepted: %v", err)
	}
	for _, path := range []string{first, second} {
		state, err := Inspect(path, "cli")
		if err != nil || state.Layout != "legacy" {
			t.Fatalf("cross-state refusal changed source: %#v %v", state, err)
		}
		if _, err = os.Stat(filepath.Join(path, snapshotDir)); !os.IsNotExist(err) {
			t.Fatal("cross-state refusal staged a snapshot")
		}
	}
}

func TestFenceRefusesSourceAndRootChangesBeforePublication(t *testing.T) {
	for _, scenario := range []string{"source", "root"} {
		t.Run(scenario, func(t *testing.T) {
			path, plan, before := fixture(t, "cli")
			preserved := filepath.Join(t.TempDir(), "preserved")
			_, err := install(path, plan, func(phase string) {
				if phase != "marker_durable" {
					return
				}
				if scenario == "root" {
					if err := os.Rename(path, preserved); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(filepath.Join(path, "state.sqlite"), append(before, []byte("changed")...), 0600); err != nil {
					t.Fatal(err)
				}
			})
			if err == nil || err.Error() != "state_changed" {
				t.Fatalf("changed source/root was admitted: %v", err)
			}
			if scenario == "root" {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatal("migration touched the replacement root")
				}
				body, err := os.ReadFile(filepath.Join(preserved, "state.sqlite"))
				if err != nil || !bytes.Equal(body, before) {
					t.Fatal("migration changed the preserved original state")
				}
			} else {
				body, err := os.ReadFile(filepath.Join(path, "state.sqlite"))
				if err != nil || !bytes.Equal(body, append(before, []byte("changed")...)) {
					t.Fatal("migration overwrote an intervening source change")
				}
			}
		})
	}
}

func TestFenceRejectsTamperedMarkerAndSnapshotAlias(t *testing.T) {
	for _, scenario := range []string{"unknown-field", "duplicate-key", "wrong-kind", "snapshot-alias"} {
		t.Run(scenario, func(t *testing.T) {
			path, plan, _ := fixture(t, "cli")
			if _, err := Install(path, plan); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(path, "state.sqlite")
			before, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			body := string(before)
			want := "state_layout_invalid"
			switch scenario {
			case "unknown-field":
				body = strings.Replace(body, "{", `{"path":"outside",`, 1)
			case "duplicate-key":
				body = strings.Replace(body, "{", `{"kind":"cli",`, 1)
			case "wrong-kind":
				body = strings.Replace(body, `"kind":"cli"`, `"kind":"server"`, 1)
			case "snapshot-alias":
				if err := os.Rename(filepath.Join(path, snapshotDir), filepath.Join(path, "preserved")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("preserved", filepath.Join(path, snapshotDir)); err != nil {
					t.Fatal(err)
				}
				want = "state_recovery_required"
			}
			if err = os.WriteFile(marker, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = Inspect(path, "cli"); err == nil || err.Error() != want {
				t.Fatalf("tampered layout accepted: %v", err)
			}
			after, err := os.ReadFile(marker)
			if err != nil || string(after) != body {
				t.Fatal("inspection repaired or replaced the marker")
			}
		})
	}
}
