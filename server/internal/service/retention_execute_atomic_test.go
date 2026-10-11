package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestRetentionExecutionResultAtomic(t *testing.T) {
	for _, target := range []string{"retention_items", "retention_plans", "spaces", "blobs"} {
		t.Run(target, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "atomic.txt", "atomic", []byte("atomic"))
			if err := f.Service.Trash(p, op.FileID, false); err != nil {
				t.Fatal(err)
			}
			if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; err != nil {
				t.Fatal(err)
			}
			preview, err := f.Service.PreviewRetention(p, "trash")
			if err != nil {
				t.Fatal(err)
			}
			id := preview["plan_id"].(string)
			var beforeSpace service.Space
			if e := f.Service.DB.First(&beforeSpace, "id = ?", h.space.ID).Error; e != nil {
				t.Fatal(e)
			}
			var beforeBlobs []service.Blob
			if e := f.Service.DB.Order("id").Find(&beforeBlobs).Error; e != nil {
				t.Fatal(e)
			}
			hits := 0
			name := "retention_result_fault"
			if err := f.Service.DB.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == target {
					hits++
					tx.AddError(errors.New("synthetic receipt write failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			result, err := f.Service.ExecuteRetention(context.Background(), p, id)
			if e := f.Service.DB.Callback().Update().Remove(name); e != nil {
				t.Fatal(e)
			}
			if hits == 0 {
				t.Fatal("fault not reached")
			}
			effectFailure := target == "spaces" || target == "blobs"
			if effectFailure {
				if err != nil || result["status"] != "failed" || fpNum(result["items_deleted"]) != 0 {
					t.Fatalf("failed effects masked: %v %v", result, err)
				}
			} else if err == nil || result != nil {
				t.Errorf("false success: %v %v", result, err)
			}
			var item service.RetentionItem
			if e := f.Service.DB.Where("plan_id = ?", id).First(&item).Error; e != nil {
				t.Fatal(e)
			}
			var files int64
			if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Count(&files).Error; e != nil {
				t.Fatal(e)
			}
			if target == "retention_items" {
				if files != 1 || item.State != "candidate" {
					t.Fatalf("deletion escaped receipt rollback: %d %s", files, item.State)
				}
			} else if effectFailure {
				if files != 1 || item.State != "failed" {
					t.Fatalf("partial effects committed: %d %s", files, item.State)
				}
			} else {
				if files != 0 || item.State != "deleted" {
					t.Fatalf("original item result lost: %d %s", files, item.State)
				}
			}
			if target != "retention_plans" {
				var space service.Space
				if e := f.Service.DB.First(&space, "id = ?", h.space.ID).Error; e != nil || space.Used != beforeSpace.Used {
					t.Fatalf("usage escaped rollback: %+v %v", space, e)
				}
				for _, before := range beforeBlobs {
					var blob service.Blob
					if e := f.Service.DB.First(&blob, "id = ?", before.ID).Error; e != nil || blob.Refs != before.Refs {
						t.Fatalf("blob escaped rollback: %+v %v", blob, e)
					}
				}
			}
			for i := 0; i < 2; i++ {
				out, e := f.Service.ExecuteRetention(context.Background(), p, id)
				status, deleted, skipped := "completed", int64(1), int64(0)
				if effectFailure {
					status, deleted, skipped = "failed", 0, 1
				}
				if e != nil || out["status"] != status || fpNum(out["items_deleted"]) != deleted || fpNum(out["items_skipped"]) != skipped {
					t.Fatalf("replay count drift: %v %v", out, e)
				}
			}
			var final service.RetentionItem
			if e := f.Service.DB.First(&final, "id = ?", item.ID).Error; e != nil {
				t.Fatal(e)
			}
			if (target == "retention_plans" || effectFailure) && !final.UpdatedAt.Equal(item.UpdatedAt) {
				t.Fatal("replay rewrote original receipt")
			}
		})
	}
}
