package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestRetentionStopsAfterCommittedPolicyChange(t *testing.T) {
	for _, mode := range []string{"policy", "cancelled", "unknown", "policy_error", "last_cancelled", "completed"} {
		t.Run(mode, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			names := []string{"one", "two"}
			if mode == "last_cancelled" {
				names = names[:1]
			}
			for _, name := range names {
				op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, name+".txt", name, []byte(name))
				if e := f.Service.Trash(p, op.FileID, false); e != nil {
					t.Fatal(e)
				}
				if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; e != nil {
					t.Fatal(e)
				}
			}
			preview, e := f.Service.PreviewRetention(p, "trash")
			if e != nil {
				t.Fatal(e)
			}
			id := preview["plan_id"].(string)
			changed := false
			queries := 0
			// Deterministic between-item change, committed with the first item.
			// No goroutines, sleeps or race instrumentation.
			if e := f.Service.DB.Callback().Update().After("gorm:update").Register("retention_change_after_item", func(tx *gorm.DB) {
				if tx.Statement.Table != "retention_items" || changed || tx.Error != nil {
					return
				}
				changed = true
				db := tx.Session(&gorm.Session{NewDB: true})
				if mode == "policy" {
					tx.AddError(db.Create(&service.RetentionPolicy{ID: service.ID(), Owner: p.UserID, Category: "trash", Revision: 1, KeepDays: 365}).Error)
				}
				if mode == "cancelled" || mode == "unknown" || mode == "completed" || mode == "last_cancelled" {
					state := mode
					if mode == "last_cancelled" {
						state = "cancelled"
					}
					tx.AddError(db.Model(&service.RetentionPlan{}).Where("id = ?", id).Update("status", state).Error)
				}
			}); e != nil {
				t.Fatal(e)
			}
			if e := f.Service.DB.Callback().Query().Before("gorm:query").Register("retention_late_policy_failure", func(tx *gorm.DB) {
				if mode == "policy_error" && changed && tx.Statement.Table == "retention_policies" {
					queries++
					tx.AddError(errors.New("synthetic policy outage"))
				}
			}); e != nil {
				t.Fatal(e)
			}
			out, err := f.Service.ExecuteRetention(context.Background(), p, id)
			if e := f.Service.DB.Callback().Update().Remove("retention_change_after_item"); e != nil {
				t.Fatal(e)
			}
			if e := f.Service.DB.Callback().Query().Remove("retention_late_policy_failure"); e != nil {
				t.Fatal(e)
			}
			if !changed {
				t.Fatal("change not reached")
			}
			want := "retention_plan_stale"
			if mode == "policy_error" {
				want = "database_unavailable"
				if queries == 0 {
					t.Error("late policy query not reached")
				}
			}
			if err == nil || err.Error() != want || out != nil {
				t.Errorf("stale plan kept executing: %v %v", out, err)
			}
			var items []service.RetentionItem
			if e := f.Service.DB.Where("plan_id = ?", id).Order("id").Find(&items).Error; e != nil {
				t.Fatal(e)
			}
			if len(items) != len(names) || items[0].State != "deleted" || (len(names) == 2 && items[1].State != "candidate") {
				t.Fatalf("lost original result or continued: %+v", items)
			}
			var count int64
			if e := f.Service.DB.Model(&service.File{}).Where("owner = ?", p.UserID).Count(&count).Error; e != nil || count != int64(len(names)-1) {
				t.Fatalf("remaining file lost: %d %v", count, e)
			}
			var plan service.RetentionPlan
			if e := f.Service.DB.First(&plan, "id = ?", id).Error; e != nil {
				t.Fatal(e)
			}
			wantState := "preview"
			if mode == "cancelled" || mode == "unknown" || mode == "completed" {
				wantState = mode
			}
			if mode == "last_cancelled" {
				wantState = "cancelled"
			}
			if plan.Status != wantState {
				t.Fatalf("aggregate overwrote stop: %s", plan.Status)
			}
		})
	}
}
