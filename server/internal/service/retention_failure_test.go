package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestRetentionProtectionReadFailure(t *testing.T) {
	for _, target := range []string{"holds", "source"} {
		for _, mode := range []string{"purge", "plan", "history", "read"} {
			t.Run(target+"/"+mode, func(t *testing.T) {
				f := rig(t)
				h, _ := login(t, f, "alice")
				p := service.Principal{UserID: h.user.ID}
				c := mustSDK(t, h)
				op := fpUploadBytes(t, c, h.space.ID, "protected.txt", "upload", []byte("protected bytes"))
				hold := fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "hold")
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
				if mode == "history" {
					// A synthetic new head keeps the original version historical;
					// this fixture does not publish or delete any real user object.
					if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("version_id", service.ID()).Error; e != nil {
						t.Fatal(e)
					}
					if e := f.Service.DB.Model(&service.RetentionItem{}).Where("plan_id = ?", preview["plan_id"]).Updates(map[string]any{"kind": "version", "version_id": op.VersionID}).Error; e != nil {
						t.Fatal(e)
					}
				}
				name := "test_retention_query_failure"
				hits := 0
				if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
					match := tx.Statement.Table == "retention_holds"
					if target == "source" {
						_, match = tx.Statement.Dest.(*service.Operation)
					}
					if match {
						hits++
						tx.AddError(errors.New("synthetic retention query failure"))
					}
				}); err != nil {
					t.Fatal(err)
				}
				var purgeErr error
				var result map[string]any
				if mode == "purge" {
					purgeErr = f.Service.Purge(context.Background(), p, op.FileID)
				} else if mode == "read" {
					_, purgeErr = f.Service.GetRetentionHold(p, hold.HoldID)
				} else {
					result, err = f.Service.ExecuteRetention(context.Background(), p, preview["plan_id"].(string))
				}
				if remove := f.Service.DB.Callback().Query().Remove(name); remove != nil {
					t.Fatal(remove)
				}
				if hits == 0 {
					t.Fatal("fault not reached")
				}
				var count int64
				if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Count(&count).Error; e != nil || count != 1 {
					t.Fatalf("file lost under unknown protection: %d %v", count, e)
				}
				if (mode == "purge" || mode == "read") && (purgeErr == nil || purgeErr.Error() != "database_unavailable") {
					t.Fatalf("expected safe purge refusal: %v", purgeErr)
				}
				if mode == "plan" || mode == "history" {
					if err != nil {
						t.Fatal(err)
					}
					if result["status"] != "failed" {
						t.Fatalf("failed protection became success: %+v", result)
					}
					if fpNum(result["items_deleted"]) != 0 {
						t.Fatalf("rollback reported deletion: %+v", result)
					}
					var item service.RetentionItem
					if e := f.Service.DB.Where("plan_id = ? AND file_id = ?", preview["plan_id"], op.FileID).First(&item).Error; e != nil {
						t.Fatal(e)
					}
					if item.State != "failed" || item.Detail != "database_unavailable" {
						t.Fatalf("failure disguised: %+v", item)
					}
				}
				var v service.Version
				if e := f.Service.DB.First(&v, "id = ?", op.VersionID).Error; e != nil {
					t.Fatal("version lost", e)
				}
			})
		}
	}
}
