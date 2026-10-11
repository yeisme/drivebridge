package service_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

// Test-local database faults only; no production injection surface.
func failBatchItemQuery(t *testing.T, db *gorm.DB, countOnly bool, skip int) func() {
	t.Helper()
	seen := 0
	name := "test_batch_item_query_fault"
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "upload_batch_items" {
			return
		}
		if countOnly {
			if _, ok := tx.Statement.Dest.(*int64); !ok {
				return
			}
		}
		seen++
		if seen > skip {
			tx.AddError(errors.New("synthetic batch query failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := db.Callback().Query().Remove(name); err != nil {
			t.Fatal(err)
		}
		if seen <= skip {
			t.Fatal("fault did not reach target query")
		}
	}
}

func TestUploadBatchQueryFailuresRefuseFalseState(t *testing.T) {
	for _, action := range []string{"read", "resume", "cancel"} {
		t.Run(action, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			b, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "query-failure")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.SealUploadBatch(p, b.BatchID); err != nil {
				t.Fatal(err)
			}
			if action == "resume" {
				if _, err = f.Service.ControlUploadBatch(p, b.BatchID, "pause"); err != nil {
					t.Fatal(err)
				}
			}
			var before service.UploadBatch
			if err = f.Service.DB.First(&before, "id = ?", b.BatchID).Error; err != nil {
				t.Fatal(err)
			}
			remove := failBatchItemQuery(t, f.Service.DB, action != "cancel", 0)
			if action == "read" {
				_, err = f.Service.GetUploadBatch(p, b.BatchID, "", 1)
			} else {
				_, err = f.Service.ControlUploadBatch(p, b.BatchID, action)
			}
			remove()
			if err == nil || err.Error() != "database_unavailable" {
				t.Fatalf("unsafe query outcome: %v", err)
			}
			var after service.UploadBatch
			if err = f.Service.DB.First(&after, "id = ?", b.BatchID).Error; err != nil {
				t.Fatal(err)
			}
			if before.Phase != after.Phase || before.Revision != after.Revision {
				t.Fatal("query failure changed batch state")
			}
		})
	}
}

func TestUploadBatchReadRejectsEachFailedCounter(t *testing.T) {
	for skip := 0; skip < 3; skip++ {
		t.Run(strings.Repeat("next-", skip)+"count", func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			b, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "counter")
			if err != nil {
				t.Fatal(err)
			}
			remove := failBatchItemQuery(t, f.Service.DB, true, skip)
			_, err = f.Service.GetUploadBatch(p, b.BatchID, "", 1)
			remove()
			if err == nil || err.Error() != "database_unavailable" {
				t.Fatal("count failure presented as zero")
			}
		})
	}
}

func TestUploadBatchCompletionCountFailureDoesNotCompleteAnchor(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	b, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "finish-query")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Service.AddUploadBatchItems(p, b.BatchID, []service.UploadBatchItemInput{{RelativePath: "a.txt", Size: 1, SHA256: service.Hash("a")}, {RelativePath: "b.txt", Size: 1, SHA256: service.Hash("b")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SealUploadBatch(p, b.BatchID); err != nil {
		t.Fatal(err)
	}
	op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "a.txt", Size: 1, SHA256: service.Hash("a"), BatchID: b.BatchID, ItemKey: "a.txt\x00" + service.Hash("a")}, "finish-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString("a")); err != nil {
		t.Fatal(err)
	}
	remove := failBatchItemQuery(t, f.Service.DB, true, 0)
	done, err := f.Service.Complete(t.Context(), p, op.ID)
	remove()
	if err != nil || done.Status != "completed" {
		t.Fatal("original file publication was changed")
	}
	var anchor service.Operation
	var batch service.UploadBatch
	var item service.UploadBatchItem
	if err = f.Service.DB.First(&anchor, "id = ?", b.OperationID).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.First(&batch, "id = ?", b.BatchID).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.First(&item, "batch_id = ? AND relative_path = ?", b.BatchID, "a.txt").Error; err != nil {
		t.Fatal(err)
	}
	if anchor.Status == "completed" || batch.Phase == "completed" || item.Status == "completed" {
		t.Fatal("aggregate query failure committed false completion")
	}
	// Publication remains true, but aggregate reconciliation is still pending.
	if done.FileID == "" || done.VersionID == "" {
		t.Fatal("fixed publication identity lost")
	}
}
