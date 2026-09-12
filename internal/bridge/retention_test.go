package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetentionTTLAndManualGC(t *testing.T) {
	s, _, local, _ := setup(t)
	// Default spaces are permanent: gc reports nothing instead of guessing.
	if err := s.SetSpaceTTL("creative", 0); err != nil {
		t.Fatal(err)
	}
	report, err := s.GC(context.Background(), "creative", false)
	if err != nil || report.TTLDays != 0 || len(report.Candidates) != 0 {
		t.Fatalf("permanent-space gc: %+v err=%v", report, err)
	}
	kept := uploadFixture(t, s, local)
	second, err := s.Upload(context.Background(), "creative", "", filepath.Join(local, "reference.png"), "reference.png", "second")
	if err != nil {
		t.Fatal(err)
	}
	second = waitOp(t, s, second.ID)
	if second.Status != "completed" || second.FileRef == kept.FileRef {
		t.Fatalf("second upload: %s %s", second.Status, second.FileRef)
	}
	expiring := second
	if _, err = s.Stat(context.Background(), expiring.FileRef); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pin(kept.FileRef); err != nil {
		t.Fatal(err)
	}
	if err = s.SetSpaceTTL("creative", 7); err != nil {
		t.Fatal(err)
	}
	if days, err := s.TTL("creative"); err != nil || days != 7 {
		t.Fatalf("ttl readback: %d err=%v", days, err)
	}
	// Fresh activity keeps both references out of scope.
	report, err = s.GC(context.Background(), "creative", false)
	if err != nil || len(report.Candidates) != 0 || report.Scanned < 2 {
		t.Fatalf("fresh gc: %+v err=%v", report, err)
	}
	// Backdate the activity of both references past the window.
	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	for _, ref := range []string{kept.FileRef, expiring.FileRef} {
		if err = s.DB.Model(&Event{}).Where("space_id = ? AND ref = ?", "creative", ref).Update("at", old).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Dry run reports the plan without touching anything.
	report, err = s.GC(context.Background(), "creative", true)
	if err != nil || len(report.Candidates) != 1 || report.Candidates[0] != expiring.FileRef || report.Pinned != 1 {
		t.Fatalf("dry-run report: %+v err=%v", report, err)
	}
	var dryProbe File
	if err = s.DB.First(&dryProbe, "ref = ?", expiring.FileRef).Error; err != nil || dryProbe.Trashed != nil {
		t.Fatal("dry run trashed a reference")
	}
	// The real run collects the expired reference into the space trash and
	// keeps the pinned one.
	report, err = s.GC(context.Background(), "creative", false)
	if err != nil || len(report.Collected) != 1 || report.Collected[0] != expiring.FileRef {
		t.Fatalf("gc run: %+v err=%v", report, err)
	}
	var collectedProbe File
	if err = s.DB.First(&collectedProbe, "ref = ?", expiring.FileRef).Error; err != nil || collectedProbe.Trashed == nil {
		t.Fatal("collection did not trash the expired reference")
	}
	var keptProbe File
	if err = s.DB.First(&keptProbe, "ref = ?", kept.FileRef).Error; err != nil || keptProbe.Trashed != nil {
		t.Fatal("pinned reference was collected")
	}
	// Collected references stay restorable.
	if _, err = s.Restore(context.Background(), expiring.FileRef); err != nil {
		t.Fatal(err)
	}
	// Negative windows are rejected; unknown spaces too.
	if err = s.SetSpaceTTL("creative", -1); err == nil {
		t.Fatal("negative ttl accepted")
	}
	if err = s.SetSpaceTTL("missing", 3); err == nil {
		t.Fatal("unknown space accepted")
	}
	if _, err = s.Pin("drivebridge://file/missing"); err == nil || err.Error() != "reference_not_found" {
		t.Fatalf("pin missing ref: %v", err)
	}
}

func TestRetentionTouchKeepsReferencesAlive(t *testing.T) {
	s, _, local, _ := setup(t)
	op := uploadFixture(t, s, local)
	if err := s.SetSpaceTTL("creative", 1); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-72 * time.Hour)
	if err := s.DB.Model(&Event{}).Where("space_id = ? AND ref = ?", "creative", op.FileRef).Update("at", old).Error; err != nil {
		t.Fatal(err)
	}
	// An explicit reuse or link bumps last-reference activity even though the
	// event history is old.
	s.touchRetention("creative", op.FileRef)
	report, err := s.GC(context.Background(), "creative", false)
	if err != nil || len(report.Candidates) != 0 {
		t.Fatalf("touched reference collected: %+v err=%v", report, err)
	}
}

func TestReuseUploadRegistersTouchedActivity(t *testing.T) {
	s, _, local, _ := setup(t)
	original := uploadFixture(t, s, local)
	same, err := os.ReadFile(filepath.Join(local, "reference.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(local, "copy.png"), same, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReuseUpload(context.Background(), "creative", "", filepath.Join(local, "copy.png"), "copy.png", "touch-key"); err != nil {
		t.Fatal(err)
	}
	var row FileRetention
	if err = s.DB.First(&row, "ref = ?", original.FileRef).Error; err != nil {
		t.Fatal("original reference has no retention activity")
	}
	if row.LastRef.IsZero() {
		t.Fatal("retention activity not recorded")
	}
}
