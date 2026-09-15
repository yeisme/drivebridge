package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge/internal/bridge"
)

// seedState creates a state directory with one recorded operation, as a
// previously-running service would have left it.
func seedState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	svc, err := bridge.New(dir, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	op := bridge.Operation{ID: "op-1", SpaceID: "sp", Kind: "upload", Status: "interrupted", Error: "resume_required", Bytes: 10, Transferred: 4, Updated: time.Now().UTC()}
	if err := svc.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	svc.Close()
	if err := os.WriteFile(filepath.Join(dir, "rclone.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// holdLock simulates a registered MCP client holding the state.
func holdLock(t *testing.T, dir string) {
	t.Helper()
	lock, err := os.OpenFile(filepath.Join(dir, "process.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lockFile(lock, lockRelease); lock.Close() })
	if err := lockFile(lock, lockExclusive|lockNonBlocking); err != nil {
		t.Fatal(err)
	}
}

func snapshotCLI(t *testing.T, dir string, mode outputMode) *cli {
	t.Helper()
	c := &cli{mode: mode}
	c.effective.Values = map[string]string{"state": dir, "rclone_config": filepath.Join(dir, "rclone.conf")}
	return c
}

func TestRecentFallsBackToSnapshotUnderLock(t *testing.T) {
	dir := seedState(t)
	holdLock(t, dir)
	cmd := newRecentCommand(snapshotCLI(t, dir, outputMode{JSON: true}))
	out, _ := capture(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Errorf("recent under lock: %v", err)
		}
	})
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Count    int               `json:"count"`
			ReadOnly bool              `json:"read_only"`
			Ops      []json.RawMessage `json:"operations"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("no envelope: %v (%q)", err, out)
	}
	if envelope.Status != "success" || envelope.Data.Count != 1 || !envelope.Data.ReadOnly {
		t.Fatalf("snapshot fallback result: %+v (%s)", envelope.Data, out)
	}
}

func TestOperationFallsBackToSnapshotUnderLock(t *testing.T) {
	dir := seedState(t)
	holdLock(t, dir)
	commands := newTransferCommands(snapshotCLI(t, dir, outputMode{JSON: true}))
	cmd := commands[0]
	for _, candidate := range commands {
		if candidate.Name() == "operation" {
			cmd = candidate
		}
	}
	if err := cmd.Flags().Set("id", "op-1"); err != nil {
		t.Fatal(err)
	}
	out, _ := capture(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Errorf("operation under lock: %v", err)
		}
	})
	if !strings.Contains(out, "interrupted") || !strings.Contains(out, `"status":"partial"`) {
		t.Fatalf("operation snapshot must inspect without the lock: %q", out)
	}
}

func TestFindFallsBackToIndexSnapshotUnderLock(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	svc, err := bridge.New(dir, bridge.NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	seed := bridge.File{Name: "report-q3.pdf", SpaceID: "sp", Dir: "", Ref: "drivebridge://file/1/version/1", MIME: "application/pdf", ObservedAt: time.Now().UTC()}
	if err := svc.DB.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rclone.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	svc.Close()
	holdLock(t, dir)
	commands := newTransferCommands(snapshotCLI(t, dir, outputMode{JSON: true}))
	cmd := commands[0]
	for _, candidate := range commands {
		if candidate.Name() == "find" {
			cmd = candidate
		}
	}
	for k, v := range map[string]string{"space": "sp", "query": "report"} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	out, _ := capture(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Errorf("find under lock: %v", err)
		}
	})
	if !strings.Contains(out, "report-q3.pdf") || !strings.Contains(out, "\"read_only\":true") {
		t.Fatalf("find snapshot must serve the index: %s", out)
	}
}

func TestMutatingCommandsReportTypedLockCode(t *testing.T) {
	dir := seedState(t)
	holdLock(t, dir)
	commands := newTransferCommands(snapshotCLI(t, dir, outputMode{}))
	var target = commands[0]
	for _, candidate := range commands {
		if candidate.Name() == "ls" {
			target = candidate
		}
	}
	if err := target.Flags().Set("space", "sp"); err != nil {
		t.Fatal(err)
	}
	err := target.RunE(target, nil)
	var f *cliFailure
	if !errors.As(err, &f) || f.code != "state_locked" {
		t.Fatalf("ls under lock must fail with the typed code, got %v", err)
	}
	if hint := hintEN("state_locked"); !strings.Contains(hint, "MCP client") {
		t.Fatalf("state_locked hint must name the cause: %q", hint)
	}
}
