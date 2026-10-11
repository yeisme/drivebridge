package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
)

// A restore committing between Purge's pre-check and its transaction must
// win: the purge aborts and the file survives with trashed=false.
func TestPurgeRestoreRaceKeepsRestoredFile(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	for i := 0; i < 40; i++ {
		op := uploadBytes(t, s, p, sp.ID, fmt.Sprintf("race-%d.bin", i), []byte(fmt.Sprintf("body-%d", i)))
		if e := s.Trash(p, op.FileID, false); e != nil {
			t.Fatal(e)
		}
		restored := make(chan error, 1)
		go func(id string) {
			for n := 0; n < 500; n++ {
				e := s.Trash(p, id, true)
				if e == nil {
					restored <- nil
					return
				}
				if e.Error() == "not_found" {
					restored <- e
					return
				}
			}
			restored <- errors.New("restore never settled")
		}(op.FileID)
		purgeErr := s.Purge(context.Background(), p, op.FileID)
		restoreErr := <-restored
		var count int64
		s.DB.Model(&File{}).Where("id = ?", op.FileID).Count(&count)
		if restoreErr == nil && purgeErr == nil {
			t.Fatalf("iteration %d: restore and purge both succeeded", i)
		}
		if purgeErr == nil && count != 0 {
			t.Fatalf("iteration %d: purge succeeded but row remains", i)
		}
		if restoreErr == nil && count == 0 {
			t.Fatalf("iteration %d: restore succeeded but row is gone", i)
		}
	}
}

func TestRestoreAfterPurgeAndIdempotentRestore(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	op := uploadBytes(t, s, p, sp.ID, "gone.bin", []byte("gone"))
	if e := s.Trash(p, op.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Purge(context.Background(), p, op.FileID); e != nil {
		t.Fatal(e)
	}
	if e := s.Trash(p, op.FileID, true); e == nil || e.Error() != "not_found" {
		t.Fatalf("restore after purge: %v", e)
	}
	live := uploadBytes(t, s, p, sp.ID, "live.bin", []byte("live"))
	if e := s.Trash(p, live.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Trash(p, live.FileID, true); e != nil {
		t.Fatal(e)
	}
	if e := s.Trash(p, live.FileID, true); e != nil {
		t.Fatalf("idempotent restore rejected: %v", e)
	}
}

// Directory operations never hold a reservation, so cancelling one must not
// demand or release quota.
func TestCancelDirectoryOperationRequiresNoReservation(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	op := Operation{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), Key: ID(), Fingerprint: ID(), Kind: "directory_copy", SpaceID: sp.ID, Path: "dest", Status: "running", Size: 123, Updated: time.Now().UTC()}
	if e := s.DB.Create(&op).Error; e != nil {
		t.Fatal(e)
	}
	got, e := s.Cancel(p, op.ID)
	if e != nil {
		t.Fatalf("cancel directory_copy: %v", e)
	}
	if got.Status != "cancelled" {
		t.Fatalf("status %s", got.Status)
	}
	s.DB.First(&sp, "id = ?", sp.ID)
	if sp.Reserved != 0 {
		t.Fatalf("reserved changed: %d", sp.Reserved)
	}
}

// A rename-mode copy must not publish a file at a directory's path.
func TestCopyRenameAvoidsDirectoryPath(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	if e := s.DB.Create(&Directory{ID: ID(), Owner: p.dataOwner(), SpaceID: sp.ID, Path: "backup"}).Error; e != nil {
		t.Fatal(e)
	}
	src := uploadBytes(t, s, p, sp.ID, "src.bin", []byte("copy-me"))
	op, e := s.CopyFile(context.Background(), p, src.FileID, CopyRequest{TargetSpaceID: sp.ID, TargetPath: "backup", ExpectedVersion: src.VersionID}, ID())
	if e != nil {
		t.Fatal(e)
	}
	if op.Status != "completed" {
		t.Fatalf("copy status %s", op.Status)
	}
	if op.Path != "backup (2)" {
		t.Fatalf("rename landed at %q", op.Path)
	}
	var f File
	if e := s.DB.First(&f, "space_id = ? AND path = ?", sp.ID, "backup").Error; e == nil {
		t.Fatal("file published at a directory's path")
	}
}

// A rename-mode copy whose target gets taken during streaming re-resolves at
// completion instead of failing the operation outright.
func TestCompleteRenameMovesAroundLatePathTaker(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("late-taker")
	sum := sha256.Sum256(body)
	src := uploadBytes(t, s, p, sp.ID, "src-late.bin", body)
	op := Operation{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), Key: ID(), Fingerprint: ID(), Kind: "copy", RenameOnConflict: true, SpaceID: sp.ID, Path: "taken.bin", Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), VersionID: ID(), ConnectionID: "platform", Status: "awaiting_upload", ArchiveFileID: src.FileID, ArchiveVersionID: src.VersionID, Updated: time.Now().UTC()}
	if e := s.DB.Create(&op).Error; e != nil {
		t.Fatal(e)
	}
	uploadBytes(t, s, p, sp.ID, "taken.bin", []byte("occupier"))
	if _, e := s.putPart(p, op.ID, 0, bytes.NewReader(body), true); e != nil {
		t.Fatal(e)
	}
	done, e := s.Complete(context.Background(), p, op.ID)
	if e != nil {
		t.Fatal(e)
	}
	if done.Status != "completed" || done.Path != "taken (2).bin" {
		t.Fatalf("late-taker copy: %+v", done)
	}
}
