package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestArchiveVersionPinsSourceAndKeepsConflict(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	digest := sha256.Sum256([]byte("first"))
	up, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "renders/final.txt", Size: 5, SHA256: hex.EncodeToString(digest[:])}, "source-once")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, up.ID, 0, strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	first, err := f.Service.Complete(context.Background(), p, up.ID)
	if err != nil {
		t.Fatal(err)
	}
	op, err := f.Service.ArchiveVersion(context.Background(), p, service.ArchiveRequest{SourceFileID: first.FileID, SourceVersionID: first.VersionID, TargetSpaceID: h.space.ID, TargetPath: "archive/final.txt"}, "archive-once")
	if err != nil || op.Status != "completed" {
		t.Fatalf("archive: %+v %v", op, err)
	}
	second, err := f.Service.ArchiveVersion(context.Background(), p, service.ArchiveRequest{SourceFileID: first.FileID, SourceVersionID: first.VersionID, TargetSpaceID: h.space.ID, TargetPath: "archive/final.txt"}, "archive-twice")
	if err != nil || second.Path == op.Path {
		t.Fatalf("expected conflict-safe path: %+v %v", second, err)
	}
}

func TestArchiveRejectsNonManagedTargetEarly(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	digest := sha256.Sum256([]byte("payload"))
	up, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "src.bin", Size: 7, SHA256: hex.EncodeToString(digest[:])}, "source-once")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, up.ID, 0, strings.NewReader("payload")); err != nil {
		t.Fatal(err)
	}
	first, err := f.Service.Complete(context.Background(), p, up.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A space whose storage is not a live S3 connection cannot receive a
	// managed copy: rejected before staging, reserving, or streaming bytes.
	bare := service.Space{ID: "bare-space", Owner: h.user.ID, Name: "bare", MaxBytes: 1 << 20}
	if e := f.Service.DB.Create(&bare).Error; e != nil {
		t.Fatal(e)
	}
	if _, err = f.Service.ArchiveVersion(context.Background(), p, service.ArchiveRequest{SourceFileID: first.FileID, SourceVersionID: first.VersionID, TargetSpaceID: bare.ID, TargetPath: "copy.bin"}, "archive-bare"); err == nil || err.Error() != "unsupported" {
		t.Fatalf("expected unsupported, got %v", err)
	}
	var count int64
	f.Service.DB.Model(&service.Operation{}).Where("kind = ?", "archive").Count(&count)
	if count != 0 {
		t.Fatalf("no archive operation should be created, got %d", count)
	}
	if bare.Reserved != 0 {
		t.Fatalf("quota must stay untouched, reserved=%d", bare.Reserved)
	}
}
