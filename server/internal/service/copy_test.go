package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func putText(t *testing.T, f *fixtures.Runtime, p service.Principal, space, path, body, key string) service.Operation {
	t.Helper()
	digest := sha256.Sum256([]byte(body))
	up, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: space, Path: path, Size: int64(len(body)), SHA256: hex.EncodeToString(digest[:])}, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, up.ID, 0, strings.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	done, err := f.Service.Complete(context.Background(), p, up.ID)
	if err != nil {
		t.Fatal(err)
	}
	return done
}

func TestCopyFileReusesBlobAndRenames(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	first := putText(t, f, p, h.space.ID, "notes/note.txt", "hello", "src-copy")
	op, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "backup/note.txt", ExpectedVersion: first.VersionID}, "copy-once")
	if err != nil || op.Status != "completed" || op.Kind != "copy" || !op.BlobReused || op.FileID == first.FileID {
		t.Fatalf("copy: %+v %v", op, err)
	}
	_, _, body, err := f.Service.Download(context.Background(), p, op.FileID, op.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(body)
	body.Close()
	if string(got) != "hello" {
		t.Fatalf("copied bytes %q", got)
	}
	again, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "backup/note.txt", ExpectedVersion: first.VersionID}, "copy-twice")
	if err != nil || again.Path == op.Path || again.FileID == op.FileID {
		t.Fatalf("expected rename: %+v %v", again, err)
	}
	_, err = f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: op.Path, ExpectedVersion: first.VersionID, Conflict: "fail"}, "copy-fail")
	if err == nil || err.Error() != "name_conflict" {
		t.Fatalf("expected name_conflict, got %v", err)
	}
	replay, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "backup/note.txt", ExpectedVersion: first.VersionID}, "copy-once")
	if err != nil || replay.ID != op.ID {
		t.Fatalf("idempotent replay %+v %v", replay, err)
	}
}

func TestCopyRejectsTrashedUnmanagedAndStaleVersion(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	first := putText(t, f, p, h.space.ID, "src.bin", "payload", "src-once")
	if err := f.Service.Trash(p, first.FileID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "out.bin", ExpectedVersion: first.VersionID}, "copy-trash"); err == nil || err.Error() != "file_trashed" {
		t.Fatalf("trashed source: %v", err)
	}
	if err := f.Service.Trash(p, first.FileID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "out.bin", ExpectedVersion: "missing"}, "copy-stale"); err == nil || err.Error() != "version_conflict" {
		t.Fatalf("stale version: %v", err)
	}
	bare := service.Space{ID: "bare-copy", Owner: h.user.ID, Name: "bare", MaxBytes: 1 << 20}
	if e := f.Service.DB.Create(&bare).Error; e != nil {
		t.Fatal(e)
	}
	if _, err := f.Service.CopyFile(context.Background(), p, first.FileID, service.CopyRequest{TargetSpaceID: bare.ID, TargetPath: "out.bin", ExpectedVersion: first.VersionID}, "copy-bare"); err == nil || err.Error() != "unsupported" {
		t.Fatalf("unmanaged target: %v", err)
	}
	var count int64
	f.Service.DB.Model(&service.Operation{}).Where("kind = ? AND `key` = ?", "copy", "copy-bare").Count(&count)
	if count != 0 {
		t.Fatalf("no copy operation for unmanaged target, got %d", count)
	}
}

func TestCopyHTTPRequiresIdempotencyKey(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	first := putText(t, f, p, h.space.ID, "web.txt", "from-http", "http-src")
	body, _ := json.Marshal(service.CopyRequest{TargetSpaceID: h.space.ID, TargetPath: "web-copy.txt", ExpectedVersion: first.VersionID})
	req, _ := http.NewRequest("POST", h.base+"/api/v1/files/"+first.FileID+"/copy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	res, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("copy without idempotency key must fail")
	}
	req, _ = http.NewRequest("POST", h.base+"/api/v1/files/"+first.FileID+"/copy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", "http-copy-one")
	res, err = h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var env struct {
		Data service.Operation `json:"data"`
	}
	if err = json.NewDecoder(res.Body).Decode(&env); err != nil || res.StatusCode != 200 || env.Data.Kind != "copy" || env.Data.Status != "completed" {
		t.Fatalf("http copy %d %+v %v", res.StatusCode, env, err)
	}
}

func TestCopyRenamesAroundTrashedOccupancy(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	src := putText(t, f, p, h.space.ID, "notes/doc.txt", "live", "src-live")
	occupied := putText(t, f, p, h.space.ID, "notes/doc (2).txt", "gone", "src-trash")
	if err := f.Service.Trash(p, occupied.FileID, false); err != nil {
		t.Fatal(err)
	}
	op, err := f.Service.CopyFile(context.Background(), p, src.FileID, service.CopyRequest{
		TargetSpaceID: h.space.ID, TargetPath: "notes/doc.txt", ExpectedVersion: src.VersionID,
	}, "copy-over-trash")
	if err != nil {
		t.Fatal(err)
	}
	if op.Path != "notes/doc (3).txt" {
		t.Fatalf("expected skip of live and trashed paths, got %q", op.Path)
	}
}

func TestResumeCopyKeepsOriginalOperation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	src := putText(t, f, p, h.space.ID, "notes/note.txt", "hello", "src-resume")
	op, err := f.Service.CopyFile(context.Background(), p, src.FileID, service.CopyRequest{
		TargetSpaceID: h.space.ID, TargetPath: "backup/note.txt", ExpectedVersion: src.VersionID,
	}, "copy-resume")
	if err != nil || op.Status != "completed" {
		t.Fatalf("copy %+v %v", op, err)
	}
	got, err := f.Service.Resume(context.Background(), p, op.ID)
	if err != nil || got.ID != op.ID || got.Status != "completed" {
		t.Fatalf("resume completed %+v %v", got, err)
	}
	_, err = f.Service.CopyFile(context.Background(), p, src.FileID, service.CopyRequest{
		TargetSpaceID: h.space.ID, TargetPath: op.Path, ExpectedVersion: src.VersionID, Conflict: "fail",
	}, "copy-resume")
	if err == nil || err.Error() != "idempotency_conflict" {
		t.Fatalf("reconstructed fail-mode replay must keep conflicting: %v", err)
	}
}
