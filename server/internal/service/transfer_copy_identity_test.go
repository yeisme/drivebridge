package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestTransferCopyLegacyAllocationRecovery(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "unlinked"
		if linked {
			name = "linked"
		}
		t.Run(name, func(t *testing.T) {
			f := transferRig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			c := mustSDK(t, h)
			ref := transferRef(t, f, c, h.space.ID, "legacy/source.txt", "legacy")
			plan, err := f.Service.CreateTransferPlan(p, service.CreateTransferPlanRequest{Sources: []service.TransferSourceInput{{Ref: ref}}, TargetSpaceID: h.space.ID, TargetPath: "legacy-copy", Mode: "copy"}, "legacy")
			if err != nil {
				t.Fatal(err)
			}
			var item service.TransferItem
			if err = f.Service.DB.Where("plan_id = ?", plan.PlanID).First(&item).Error; err != nil {
				t.Fatal(err)
			}
			// Reproduce an old binary's allocation followed by a lost item writeback.
			old, err := f.Service.ArchiveVersion(t.Context(), p, service.ArchiveRequest{SourceFileID: item.SourceFileID, SourceVersionID: item.SourceVersionID, TargetSpaceID: plan.TargetSpaceID, TargetPath: item.TargetPath}, item.ItemKey)
			if err != nil {
				t.Fatal(err)
			}
			if linked {
				if err = f.Service.DB.Model(&item).Updates(map[string]any{"copy_operation_id": old.ID, "status": "copying"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			done, err := f.Service.ExecuteTransferPlan(t.Context(), p, plan.PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: plan.Revision})
			if err != nil || done.Phase != "copied" {
				t.Fatal("legacy recovery failed", err)
			}
			if err = f.Service.DB.First(&item, "id = ?", item.ID).Error; err != nil {
				t.Fatal(err)
			}
			if item.CopyOperationID != old.ID || item.TargetFileID != old.FileID || item.TargetVersionID != old.VersionID {
				t.Fatal("legacy operation or fixed result replaced")
			}
			var count int64
			if err = f.Service.DB.Model(&service.Operation{}).Where("owner = ? AND kind = ?", p.UserID, "archive").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("legacy allocation duplicated")
			}
			// A linked identity with a mismatched origin must not be replaced
			// by a fresh allocation, even though its old output exists.
			if err = f.Service.DB.Model(&item).Update("status", "copying").Error; err != nil {
				t.Fatal(err)
			}
			if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", old.ID).Update("fingerprint", "wrong").Error; err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.ExecuteTransferPlan(t.Context(), p, plan.PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: done.Revision}); err == nil || err.Error() != "idempotency_conflict" {
				t.Fatal("linked origin mismatch admitted")
			}
			if err = f.Service.DB.Model(&service.Operation{}).Where("owner = ? AND kind = ?", p.UserID, "archive").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("mismatched origin allocated replacement")
			}
		})
	}
}

func TestTransferCopyRepairsHistoricalCrossPlanConflict(t *testing.T) {
	f := transferRig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	ref := transferRef(t, f, c, h.space.ID, "old/source.txt", "old")
	var legacy service.Operation
	for _, target := range []string{"old-first", "old-second"} {
		plan, err := f.Service.CreateTransferPlan(p, service.CreateTransferPlanRequest{Sources: []service.TransferSourceInput{{Ref: ref}}, TargetSpaceID: h.space.ID, TargetPath: target, Mode: "copy"}, target)
		if err != nil {
			t.Fatal(err)
		}
		var item service.TransferItem
		if err = f.Service.DB.Where("plan_id = ?", plan.PlanID).First(&item).Error; err != nil {
			t.Fatal(err)
		}
		if target == "old-first" {
			legacy, err = f.Service.ArchiveVersion(t.Context(), p, service.ArchiveRequest{SourceFileID: item.SourceFileID, SourceVersionID: item.SourceVersionID, TargetSpaceID: plan.TargetSpaceID, TargetPath: item.TargetPath}, item.ItemKey)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		// Persist the exact prior executor outcome, not arbitrary retries of every failure.
		if err = f.Service.DB.Model(&item).Updates(map[string]any{"status": "failed", "error": "idempotency_conflict"}).Error; err != nil {
			t.Fatal(err)
		}
		done, err := f.Service.ExecuteTransferPlan(t.Context(), p, plan.PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: plan.Revision})
		if err != nil || done.Phase != "copied" {
			t.Fatal("historical conflict not repaired", err)
		}
		if err = f.Service.DB.First(&item, "id = ?", item.ID).Error; err != nil || item.CopyOperationID == legacy.ID || item.CopyOperationID == "" {
			t.Fatal("different destination adopted foreign legacy operation")
		}
	}
}
