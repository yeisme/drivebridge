package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestRetentionExecuteAdmissionRefusesUnknownPlan(t *testing.T) {
	for _, target := range []string{"retention_plans", "retention_policies", "retention_items", "missing_item", "wrong_total", "cancelled_context"} {
		t.Run(target, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			c := mustSDK(t, h)
			for _, name := range []string{"one", "two"} {
				op := fpUploadBytes(t, c, h.space.ID, name+".txt", name, []byte(name))
				if err := f.Service.Trash(p, op.FileID, false); err != nil {
					t.Fatal(err)
				}
				if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			}
			preview, err := f.Service.PreviewRetention(p, "trash")
			if err != nil {
				t.Fatal(err)
			}
			planID := preview["plan_id"].(string)
			want := "database_unavailable"
			if target == "missing_item" {
				var item service.RetentionItem
				if err := f.Service.DB.Where("plan_id = ?", planID).First(&item).Error; err != nil {
					t.Fatal(err)
				}
				if err := f.Service.DB.Delete(&item).Error; err != nil {
					t.Fatal(err)
				}
				want = "dependency_unavailable"
			}
			if target == "wrong_total" {
				if err := f.Service.DB.Model(&service.RetentionPlan{}).Where("id = ?", planID).Update("items_total", -1).Error; err != nil {
					t.Fatal(err)
				}
				want = "dependency_unavailable"
			}
			hits := 0
			queryName := "retention_execute_read_fault"
			if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(queryName, func(tx *gorm.DB) {
				if tx.Statement.Table == target {
					hits++
					tx.AddError(errors.New("synthetic admission read failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			writes := 0
			updateName := "retention_execute_write_observer"
			if err := f.Service.DB.Callback().Update().Before("gorm:update").Register(updateName, func(tx *gorm.DB) { writes++ }); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if target == "cancelled_context" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			result, err := f.Service.ExecuteRetention(ctx, p, planID)
			if e := f.Service.DB.Callback().Query().Remove(queryName); e != nil {
				t.Fatal(e)
			}
			if e := f.Service.DB.Callback().Update().Remove(updateName); e != nil {
				t.Fatal(e)
			}
			if target == "cancelled_context" {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("cancelled admission: %v", err)
				}
			} else if err == nil || err.Error() != want {
				t.Errorf("expected %s, got %v", want, err)
			}
			if result != nil {
				t.Errorf("false execution result: %+v", result)
			}
			if target == "retention_plans" || target == "retention_policies" || target == "retention_items" {
				if hits == 0 {
					t.Fatal("fault not reached")
				}
			}
			if writes != 0 {
				t.Errorf("failed admission wrote state: %d", writes)
			}
			for _, model := range []any{&service.File{}, &service.Version{}} {
				var count int64
				if e := f.Service.DB.Model(model).Where("owner = ?", p.UserID).Count(&count).Error; e != nil || count != 2 {
					t.Errorf("admission changed source %T: %d %v", model, count, e)
				}
			}
			var plan service.RetentionPlan
			if e := f.Service.DB.First(&plan, "id = ?", planID).Error; e != nil {
				t.Fatal(e)
			}
			if plan.Status != "preview" || plan.ItemsDeleted != 0 || plan.ItemsSkipped != 0 {
				t.Errorf("plan changed: %+v", plan)
			}
		})
	}
}

func TestRetentionExecuteAdmissionEmptyAndMissing(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	if result, err := f.Service.ExecuteRetention(context.Background(), p, "missing"); err == nil || err.Error() != "not_found" || result != nil {
		t.Fatalf("missing plan: %v %v", result, err)
	}
	preview, err := f.Service.PreviewRetention(p, "trash")
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.Service.ExecuteRetention(context.Background(), p, preview["plan_id"].(string))
	if err != nil || result["status"] != "completed" || fpNum(result["items_deleted"]) != 0 || fpNum(result["items_skipped"]) != 0 {
		t.Fatalf("valid empty plan: %v %v", result, err)
	}
}
