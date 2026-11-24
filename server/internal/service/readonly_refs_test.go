package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestReadFileRefPinsVersionAndAuthority(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, mustSDK(t, h), h.space.ID, "refs/pinned.txt", "first")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	ref := f.Service.FileRef(file)
	gotFile, gotVersion, err := f.Service.ReadFileRef(p, ref)
	if err != nil {
		t.Fatal(err)
	}
	if gotFile.ID != file.ID || gotVersion.ID != file.VersionID || gotFile.VersionID != file.VersionID {
		t.Fatalf("reference drifted: file=%+v version=%+v", gotFile, gotVersion)
	}
	replacement, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: file.Path, FileID: file.ID, ExpectedVersion: file.VersionID, Size: 0, SHA256: service.Hash("")}, "refs-replace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.Complete(t.Context(), p, replacement.ID); err != nil {
		t.Fatal(err)
	}
	pinnedFile, pinnedVersion, err := f.Service.ReadFileRef(p, ref)
	if err != nil || pinnedVersion.ID != file.VersionID || pinnedFile.VersionID != file.VersionID {
		t.Fatalf("pinned reference followed head: err=%v file=%+v version=%+v", err, pinnedFile, pinnedVersion)
	}
	if _, _, err = f.Service.ReadFileRef(service.Principal{UserID: "foreign"}, ref); err == nil {
		t.Fatal("foreign principal resolved reference")
	}
	if _, _, err = f.Service.ReadFileRef(p, "drivebridge://other/file/"+file.ID+"/version/"+file.VersionID); err == nil {
		t.Fatal("foreign instance reference accepted")
	}
}
