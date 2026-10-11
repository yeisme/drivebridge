package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestWorkspaceTransferPlanRequestBudget(t *testing.T) {
	f := transferRig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	ref := transferRef(t, f, c, h.space.ID, "budget/source.txt", "source")
	plan, err := f.Service.CreateTransferPlan(p, service.CreateTransferPlanRequest{Sources: []service.TransferSourceInput{{Ref: ref}}, TargetSpaceID: h.space.ID, TargetPath: "budget", Mode: "copy"}, "budget")
	if err != nil {
		t.Fatal(err)
	}
	var seed service.TransferItem
	if err = f.Service.DB.Where("plan_id = ?", plan.PlanID).First(&seed).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]service.TransferItem, 1000)
	for i := range rows {
		rows[i] = seed
		rows[i].ID = service.ID()
		rows[i].ItemKey = service.ID()
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.TransferPlan{}).Where("id = ?", plan.PlanID).Update("items_total", 1001).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan&limit=1", nil, nil); status != 429 {
		t.Fatalf("scan budget %d", status)
	}
	if err = f.Service.DB.Delete(&rows[999]).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, nil); status != 503 {
		t.Fatal("mismatched total accepted")
	}
	if err = f.Service.DB.Model(&service.TransferPlan{}).Where("id = ?", plan.PlanID).Update("items_total", 1000).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, nil); status != 200 {
		t.Fatalf("bounded query %d", status)
	}
}

// Regression for the legacy same-source key collision across distinct plans.
func TestWorkspaceTransferSameSourcePlansKeepDistinctCopies(t *testing.T) {
	f := transferRig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	ref := transferRef(t, f, c, h.space.ID, "collision/source.txt", "same")
	for _, target := range []string{"first", "second"} {
		plan, err := f.Service.CreateTransferPlan(p, service.CreateTransferPlanRequest{Sources: []service.TransferSourceInput{{Ref: ref}}, TargetSpaceID: h.space.ID, TargetPath: target, Mode: "copy"}, target)
		if err != nil {
			t.Fatal(err)
		}
		done, err := f.Service.ExecuteTransferPlan(t.Context(), p, plan.PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: plan.Revision})
		if err != nil {
			t.Fatal(err)
		}
		{
			var item service.TransferItem
			if err = f.Service.DB.Where("plan_id = ?", plan.PlanID).First(&item).Error; err != nil {
				t.Fatal(err)
			}
			if done.Phase != "copied" || item.Error != "" || item.CopyOperationID == "" {
				t.Fatal("distinct plan did not publish its copy")
			}
			var file service.File
			if err = f.Service.DB.First(&file, "id = ?", item.TargetFileID).Error; err != nil || file.Path != target+"/source.txt" {
				t.Fatal("wrong destination")
			}
			replay, err := f.Service.ExecuteTransferPlan(t.Context(), p, plan.PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: done.Revision})
			if err != nil || replay.OperationID != done.OperationID {
				t.Fatal("replay allocated another anchor")
			}
			var count int64
			if err = f.Service.DB.Model(&service.File{}).Where("owner = ? AND path = ?", p.UserID, target+"/source.txt").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("copy duplicated")
			}
			var page contract.Page[contract.TaskProjection]
			if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan&state=recoverable", nil, &page); status != 200 || len(page.Items) != 0 {
				t.Fatal("completed copy still marked recoverable")
			}
		}
	}
}
