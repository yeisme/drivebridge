package service

import (
	"context"
	"testing"
	"time"
)

func TestTrashRetentionSweepAndBatch(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	if sp.TrashRetentionDays != defaultTrashRetentionDays {
		t.Fatalf("default retention: %d", sp.TrashRetentionDays)
	}
	first := uploadBytes(t, s, p, sp.ID, "keep.bin", []byte("keep-bytes"))
	old := uploadBytes(t, s, p, sp.ID, "old.bin", []byte("old-bytes"))
	if e := s.Trash(p, first.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Trash(p, old.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.Model(&File{}).Where("id = ?", old.FileID).Update("trashed_at", time.Now().UTC().AddDate(0, 0, -40)).Error; e != nil {
		t.Fatal(e)
	}
	listed, e := s.SearchFiles(p, sp.ID, "", "", "", "", true)
	if e != nil || len(listed) != 2 {
		t.Fatalf("trash list: %v %d", e, len(listed))
	}
	if listed[0].PurgeAfter == nil {
		t.Fatal("purge_after missing")
	}
	if e := s.SweepExpiredTrash(context.Background()); e != nil {
		t.Fatal(e)
	}
	listed, e = s.SearchFiles(p, sp.ID, "", "", "", "", true)
	if e != nil || len(listed) != 1 || listed[0].ID != first.FileID {
		t.Fatalf("sweep should keep only recent trash: %v %+v", e, listed)
	}
	updated, e := s.SetTrashPolicy(p, sp.ID, 7)
	if e != nil || updated.TrashRetentionDays != 7 {
		t.Fatalf("policy: %v %+v", e, updated)
	}
	if _, e := s.SetTrashPolicy(p, sp.ID, 0); e == nil || e.Error() != "invalid_trash_policy" {
		t.Fatalf("zero policy: %v", e)
	}
	restored, failed, e := s.BatchRestore(p, []string{first.FileID})
	if e != nil || len(restored) != 1 || len(failed) != 0 {
		t.Fatalf("batch restore: %v %v %v", e, restored, failed)
	}
	live, e := s.SearchFiles(p, sp.ID, "", "", "", "", false)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range live {
		if f.ID == first.FileID && !f.Trashed {
			found = true
		}
	}
	if !found {
		t.Fatal("restored file missing")
	}
	if e := s.Trash(p, first.FileID, false); e != nil {
		t.Fatal(e)
	}
	purged, failed, e := s.BatchPurge(context.Background(), p, []string{first.FileID})
	if e != nil || len(purged) != 1 || len(failed) != 0 {
		t.Fatalf("batch purge: %v %v %v", e, purged, failed)
	}
	if _, _, _, e := s.Download(context.Background(), p, first.FileID, ""); e == nil {
		t.Fatal("purged file still readable")
	}
	if _, _, e := s.BatchRestore(p, nil); e == nil || e.Error() != "invalid_batch" {
		t.Fatalf("empty batch: %v", e)
	}
	if _, e := s.SetTrashPolicy(Principal{UserID: p.UserID, Agent: &Agent{Scopes: "read"}}, sp.ID, 10); e == nil {
		t.Fatal("agent set policy")
	}
}

func TestTrashBatchPartialFailure(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	op := uploadBytes(t, s, p, sp.ID, "one.bin", []byte("one"))
	if e := s.Trash(p, op.FileID, false); e != nil {
		t.Fatal(e)
	}
	ok, failed, e := s.BatchPurge(context.Background(), p, []string{op.FileID, "missing-file-id"})
	if e != nil || len(ok) != 1 || len(failed) != 1 || failed[0].Code != "not_found" {
		t.Fatalf("partial: %v %v %v", e, ok, failed)
	}
}
