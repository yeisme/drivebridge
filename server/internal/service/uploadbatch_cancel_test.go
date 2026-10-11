package service_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

// Run with a go test timeout: a recursive mutex regression is bounded by the
// test process, not by leaving an abandoned goroutine in a live fixture.
func TestUploadBatchCancelInflight(t *testing.T) {
	for _, interrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "retry_cleanup"}[interrupt], func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			b, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "cancel-inflight")
			if err != nil {
				t.Fatal(err)
			}
			inputs := []service.UploadBatchItemInput{{RelativePath: "done.txt", Size: 1, SHA256: service.Hash("a")}, {RelativePath: "active.txt", Size: 1, SHA256: service.Hash("b")}, {RelativePath: "pending.txt", Size: 1, SHA256: service.Hash("c")}}
			if _, err = f.Service.AddUploadBatchItems(p, b.BatchID, inputs); err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.SealUploadBatch(p, b.BatchID); err != nil {
				t.Fatal(err)
			}
			create := func(i int, key string) (service.Operation, error) {
				it := inputs[i]
				return f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: it.RelativePath, Size: it.Size, SHA256: it.SHA256, BatchID: b.BatchID, ItemKey: it.RelativePath + "\x00" + it.SHA256}, key)
			}
			done, err := create(0, "done")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.PutPart(p, done.ID, 0, bytes.NewBufferString("a")); err != nil {
				t.Fatal(err)
			}
			done, err = f.Service.Complete(t.Context(), p, done.ID)
			if err != nil {
				t.Fatal(err)
			}
			active, err := create(1, "active")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.PutPart(p, active.ID, 0, bytes.NewBufferString("b")); err != nil {
				t.Fatal(err)
			}
			if interrupt {
				const callback = "fail_child_cancel"
				hit := false
				if err = f.Service.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
					if op, ok := tx.Statement.Dest.(*service.Operation); ok && op.ID == active.ID && op.Status == "cancelled" {
						hit = true
						tx.AddError(errors.New("synthetic cancel interruption"))
					}
				}); err != nil {
					t.Fatal(err)
				}
				_, cancelErr := f.Service.ControlUploadBatch(p, b.BatchID, "cancel")
				if err = f.Service.DB.Callback().Update().Remove(callback); err != nil {
					t.Fatal(err)
				}
				if !hit || cancelErr == nil {
					t.Fatal("cleanup fault not exercised")
				}
				var stored service.UploadBatch
				if err = f.Service.DB.First(&stored, "id = ?", b.BatchID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.Phase != "cancelled" {
					t.Fatal("cancellation admission not durable")
				}
				if _, err = create(2, "after-cancel"); err == nil || err.Error() != "operation_closed" {
					t.Fatal("cancelled batch admitted new scheduling")
				}
			}
			result, err := f.Service.ControlUploadBatch(p, b.BatchID, "cancel")
			if err != nil {
				t.Fatal(err)
			}
			if result.Phase != "cancelled" || result.ItemsCompleted != 1 || result.ItemsWaiting != 0 {
				t.Fatal("cancel lost completed side effect or left active work")
			}
			replay, err := f.Service.ControlUploadBatch(p, b.BatchID, "cancel")
			if err != nil || replay.BatchRevision != result.BatchRevision {
				t.Fatal("repeat cancel rekeyed or revised intent")
			}
			var space service.Space
			var child service.Operation
			if err = f.Service.DB.First(&space, "id = ?", h.space.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err = f.Service.DB.First(&child, "id = ?", active.ID).Error; err != nil {
				t.Fatal(err)
			}
			if space.Reserved != 0 || child.Status != "cancelled" {
				t.Fatal("reservation or child not cancelled")
			}
			if _, err := os.Stat(filepath.Join(f.Service.Config.State, "uploads", active.ID)); !os.IsNotExist(err) {
				t.Fatal("cancelled staging was not removed")
			}
			_, _, content, err := f.Service.Download(t.Context(), p, done.FileID, done.VersionID)
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(content)
			content.Close()
			if readErr != nil || string(body) != "a" {
				t.Fatal("committed content not preserved")
			}
			if _, err = create(2, "blocked"); err == nil || err.Error() != "operation_closed" {
				t.Fatal("new operation scheduled after cancel")
			}
			var file service.File
			if err = f.Service.DB.First(&file, "id = ?", done.FileID).Error; err != nil || file.VersionID != done.VersionID {
				t.Fatal("completed fixed result was changed")
			}
		})
	}
}
