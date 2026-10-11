package service_test

import (
	"context"
	"io"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestCopyDirectoryDuplicatesTree(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	if got := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "项目/资料"}, nil); got != 200 {
		t.Fatal(got)
	}
	if got := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "项目/资料/empty"}, nil); got != 200 {
		t.Fatal(got)
	}
	src := putText(t, f, p, h.space.ID, "项目/资料/note.txt", "hello", "dir-src")
	op, err := f.Service.CopyDirectory(context.Background(), p, service.DirectoryCopyRequest{
		SpaceID: h.space.ID, Path: "项目/资料", TargetSpaceID: h.space.ID, TargetPath: "备份/资料",
	}, "dir-copy-once")
	if err != nil || op.Status != "completed" || op.Kind != "directory_copy" || op.TotalFiles != 1 {
		t.Fatalf("copy dir %+v %v", op, err)
	}
	var listing struct{ Entries []service.BrowserEntry }
	if got := h.call(t, "GET", "/browser?space_id="+h.space.ID+"&path=备份/资料", nil, &listing); got != 200 {
		t.Fatal(got)
	}
	foundFile, foundEmpty := false, false
	for _, e := range listing.Entries {
		if e.Kind == "file" && e.Path == "备份/资料/note.txt" && e.File != nil && e.File.ID != src.FileID {
			foundFile = true
		}
		if e.Kind == "directory" && e.Path == "备份/资料/empty" {
			foundEmpty = true
		}
	}
	if !foundFile || !foundEmpty {
		t.Fatalf("copied tree %+v", listing.Entries)
	}
	_, _, body, err := f.Service.Download(context.Background(), p, listingFileID(listing, "备份/资料/note.txt"), "")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(body)
	body.Close()
	if string(got) != "hello" {
		t.Fatalf("bytes %q", got)
	}
	again, err := f.Service.CopyDirectory(context.Background(), p, service.DirectoryCopyRequest{
		SpaceID: h.space.ID, Path: "项目/资料", TargetSpaceID: h.space.ID, TargetPath: "备份/资料",
	}, "dir-copy-twice")
	if err != nil || again.Path == op.Path {
		t.Fatalf("rename %+v %v", again, err)
	}
	replay, err := f.Service.CopyDirectory(context.Background(), p, service.DirectoryCopyRequest{
		SpaceID: h.space.ID, Path: "项目/资料", TargetSpaceID: h.space.ID, TargetPath: "备份/资料",
	}, "dir-copy-once")
	if err != nil || replay.ID != op.ID {
		t.Fatalf("idempotent %+v %v", replay, err)
	}
}

func TestBrowseDirectorySupportsStableSortBeforePagination(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	putText(t, f, p, h.space.ID, "z-last.txt", "12345", "sort-z")
	putText(t, f, p, h.space.ID, "a-first.txt", "1", "sort-a")
	var listing struct{ Entries []service.BrowserEntry }
	if got := h.call(t, "GET", "/browser?space_id="+h.space.ID+"&sort=size&order=desc", nil, &listing); got != 200 {
		t.Fatal(got)
	}
	if len(listing.Entries) < 2 || listing.Entries[0].Path != "z-last.txt" {
		t.Fatalf("size sort before pagination: %+v", listing.Entries)
	}
	if got := h.call(t, "GET", "/browser?space_id="+h.space.ID+"&sort=invalid", nil, nil); got != 400 {
		t.Fatalf("invalid sort status %d", got)
	}
}

func TestResumeDirectoryCopyAfterInterrupt(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	if got := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "项目/资料"}, nil); got != 200 {
		t.Fatal(got)
	}
	putText(t, f, p, h.space.ID, "项目/资料/note.txt", "hello", "dir-resume-src")
	op, err := f.Service.CopyDirectory(context.Background(), p, service.DirectoryCopyRequest{
		SpaceID: h.space.ID, Path: "项目/资料", TargetSpaceID: h.space.ID, TargetPath: "备份/资料",
	}, "dir-copy-resume")
	if err != nil || op.Status != "completed" {
		t.Fatalf("copy dir %+v %v", op, err)
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Updates(map[string]any{
		"status": "interrupted", "error": "resume_required",
	}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := f.Service.Resume(context.Background(), p, op.ID)
	if err != nil || got.ID != op.ID || got.Status != "completed" {
		t.Fatalf("resume %+v %v", got, err)
	}
	_, err = f.Service.CopyDirectory(context.Background(), p, service.DirectoryCopyRequest{
		SpaceID: h.space.ID, Path: "项目/资料", TargetSpaceID: h.space.ID, TargetPath: op.Path, Conflict: "fail",
	}, "dir-copy-resume")
	if err == nil || err.Error() != "idempotency_conflict" {
		t.Fatalf("reconstructed fail-mode replay must keep conflicting: %v", err)
	}
}

func listingFileID(listing struct{ Entries []service.BrowserEntry }, path string) string {
	for _, e := range listing.Entries {
		if e.File != nil && e.Path == path {
			return e.File.ID
		}
	}
	return ""
}

func TestMoveDirectoryRewritesPrefix(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "项目/资料"}, nil)
	first := putText(t, f, p, h.space.ID, "项目/资料/keep.txt", "keep", "dir-move-src")
	out, err := f.Service.MoveDirectory(p, service.DirectoryMoveRequest{SpaceID: h.space.ID, Path: "项目/资料", TargetPath: "归档/资料"})
	if err != nil || out["moved"].(int) != 1 {
		t.Fatalf("move %+v %v", out, err)
	}
	var live service.File
	if f.Service.DB.First(&live, "id = ?", first.FileID).Error != nil || live.Path != "归档/资料/keep.txt" {
		t.Fatalf("identity path %+v", live)
	}
	if _, err = f.Service.MoveDirectory(p, service.DirectoryMoveRequest{SpaceID: h.space.ID, Path: "归档", TargetPath: "归档/资料/nested"}); err == nil || err.Error() != "invalid_directory" {
		t.Fatalf("into self %v", err)
	}
}

func TestTrashDirectoryHidesFolder(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "项目/资料"}, nil)
	first := putText(t, f, p, h.space.ID, "项目/资料/gone.txt", "x", "dir-trash-src")
	out, err := f.Service.TrashDirectory(p, service.DirectoryTrashRequest{SpaceID: h.space.ID, Path: "项目/资料"})
	if err != nil || out["trashed"].(int) != 1 {
		t.Fatalf("trash %+v %v", out, err)
	}
	var listing struct{ Entries []service.BrowserEntry }
	h.call(t, "GET", "/browser?space_id="+h.space.ID+"&path=项目", nil, &listing)
	for _, e := range listing.Entries {
		if e.Path == "项目/资料" {
			t.Fatalf("folder still listed %+v", listing.Entries)
		}
	}
	var live service.File
	if f.Service.DB.First(&live, "id = ?", first.FileID).Error != nil || !live.Trashed {
		t.Fatalf("file not trashed %+v", live)
	}
}
