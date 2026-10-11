package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestRetentionPreviewMissingSpaceAndEmpty(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	empty, err := f.Service.PreviewRetention(p, "trash")
	if err != nil || fpNum(empty["items_total"]) != 0 {
		t.Fatalf("valid empty preview: %v %v", empty, err)
	}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "missing-space.txt", "missing-space", []byte("fixture"))
	if err := f.Service.Trash(p, op.FileID, false); err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Updates(map[string]any{"space_id": "missing", "trashed_at": time.Now().Add(-400 * 24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.Service.PreviewRetention(p, "trash")
	if err == nil || err.Error() != "dependency_unavailable" || result != nil {
		t.Fatalf("missing space became default policy: %v %v", result, err)
	}
	var count int64
	if err := f.Service.DB.Model(&service.RetentionPlan{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("failed preview left header: %d %v", count, err)
	}
}

func TestRetentionPreviewFailureIsAtomic(t *testing.T) {
	for _, target := range []string{"retention_policies", "files", "spaces", "versions", "retention_plans", "retention_items"} {
		t.Run(target, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			c := mustSDK(t, h)
			for _, name := range []string{"first", "second"} {
				op := fpUploadBytes(t, c, h.space.ID, name+".txt", name, []byte(name))
				if err := f.Service.Trash(p, op.FileID, false); err != nil {
					t.Fatal(err)
				}
				if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			}
			hits := 0
			callback := "retention_preview_fault"
			fail := func(tx *gorm.DB) {
				if tx.Statement.Table != target {
					return
				}
				hits++
				if target != "retention_items" || hits == 2 {
					tx.AddError(errors.New("synthetic preview fault"))
				}
			}
			write := target == "retention_items" || target == "retention_plans"
			if write {
				if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, fail); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(callback, fail); err != nil {
					t.Fatal(err)
				}
			}
			category := "trash"
			if target == "versions" {
				category = "historical_versions"
			}
			result, err := f.Service.PreviewRetention(p, category)
			if write {
				if e := f.Service.DB.Callback().Create().Remove(callback); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := f.Service.DB.Callback().Query().Remove(callback); e != nil {
					t.Fatal(e)
				}
			}
			if hits == 0 || (target == "retention_items" && hits != 2) {
				t.Fatalf("fault not reached: %d", hits)
			}
			if err == nil || err.Error() != "database_unavailable" || result != nil {
				t.Errorf("query failure became preview: %v %v", result, err)
			}
			for _, model := range []any{&service.RetentionPlan{}, &service.RetentionItem{}} {
				var count int64
				if e := f.Service.DB.Model(model).Count(&count).Error; e != nil {
					t.Fatal(e)
				}
				if count != 0 {
					t.Errorf("partial preview persisted: %T %d", model, count)
				}
			}
			// Same disposable sources remain available; a manual new preview after
			// removing the fault must persist exactly the complete plan and items.
			good, e := f.Service.PreviewRetention(p, "trash")
			if e != nil {
				t.Fatal(e)
			}
			if fpNum(good["items_total"]) != 2 {
				t.Fatalf("missing candidates: %+v", good)
			}
			var items int64
			if e := f.Service.DB.Model(&service.RetentionItem{}).Where("plan_id = ?", good["plan_id"]).Count(&items).Error; e != nil || items != 2 {
				t.Fatalf("incomplete plan: %d %v", items, e)
			}
			var files int64
			if e := f.Service.DB.Model(&service.File{}).Where("owner = ?", p.UserID).Count(&files).Error; e != nil || files != 2 {
				t.Fatalf("preview changed files: %d %v", files, e)
			}
		})
	}
}
