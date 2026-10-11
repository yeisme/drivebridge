package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceTransferPlanProjection(t *testing.T) {
	f := transferRig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	plans := map[string]service.TransferPlanView{}
	for _, mode := range []string{"copy", "migrate"} {
		ref := transferRef(t, f, c, h.space.ID, "transfer/"+mode+".txt", "source-"+mode)
		plan, err := f.Service.CreateTransferPlan(p, service.CreateTransferPlanRequest{Sources: []service.TransferSourceInput{{Ref: ref}}, TargetSpaceID: h.space.ID, TargetPath: mode, Mode: mode}, "task-"+mode)
		if err != nil {
			t.Fatal(err)
		}
		plans[mode] = plan
	}
	get := func(query string) contract.Page[contract.TaskProjection] {
		t.Helper()
		var page contract.Page[contract.TaskProjection]
		if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan&"+query, nil, &page); status != 200 {
			t.Fatalf("transfer task status %d", status)
		}
		return page
	}
	first := get("limit=1")
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("missing bounded page")
	}
	for _, view := range get("").Items {
		if view.OwnerKind != "transfer_plan" || view.UIState != "waiting_confirmation" || view.ActorKind != "unknown" || view.AgentID != nil || view.Progress.Total != nil || len(view.AllowedActions) != 0 {
			t.Fatal("invented task state or attribution")
		}
	}
	if len(get("agent_id=unknown-creator").Items) != 0 {
		t.Fatal("executor promoted to creator")
	}
	var hidden contract.Page[contract.TaskProjection]
	if status := bob.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, &hidden); status != 200 || len(hidden.Items) != 0 {
		t.Fatal("foreign plans visible")
	}
	next := "/workspace/tasks?kind=transfer_plan&limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	if status := h.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatal("stable cursor rejected")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("foreign cursor accepted")
	}
	copyPlan, err := f.Service.ExecuteTransferPlan(t.Context(), p, plans["copy"].PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: plans["copy"].Revision})
	if err != nil || copyPlan.Phase != "copied" {
		t.Fatal("owner copy failed", err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("changed plan cursor accepted")
	}
	copied := get("state=unknown")
	if len(copied.Items) != 1 || copied.Items[0].OwnerID != copyPlan.PlanID || copied.Items[0].OwnerStatus != "copied" {
		t.Fatal("copy claimed proven migration")
	}
	var operations contract.Page[contract.TaskProjection]
	if status := h.call(t, "GET", "/workspace/tasks?kind=operation", nil, &operations); status != 200 {
		t.Fatal(status)
	}
	for _, op := range operations.Items {
		if op.OwnerID == copyPlan.OperationID && op.UIState != "unknown" {
			t.Fatal("anchor completion overclaimed")
		}
	}
	migrate, err := f.Service.ExecuteTransferPlan(t.Context(), p, plans["migrate"].PlanID, service.ExecuteTransferPlanRequest{ExpectedRevision: plans["migrate"].Revision})
	if err != nil || migrate.Phase != "source_cleanup_pending" {
		var item service.TransferItem
		f.Service.DB.Where("plan_id = ?", migrate.PlanID).First(&item)
		t.Fatalf("migration owner failed: phase=%s item_status=%s item_error=%s error=%v", migrate.Phase, item.Status, item.Error, err)
	}
	pending := get("state=waiting_confirmation")
	if len(pending.Items) != 1 || pending.Items[0].BlockedReason != "source_cleanup_pending" {
		t.Fatal("source cleanup conflated with copy")
	}
	completed, err := f.Service.CleanupTransferSource(p, migrate.PlanID, migrate.Revision)
	if err != nil || completed.Phase != "completed" {
		t.Fatal("disposable source cleanup failed", err)
	}
	if len(get("state=unknown").Items) != 2 {
		t.Fatal("cleanup incorrectly upgraded to independent proof")
	}
	if err = f.Service.DB.Model(&service.TransferPlan{}).Where("id = ?", copyPlan.PlanID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 1 {
		t.Fatal("tenant plan visible")
	}
	if err = f.Service.DB.Model(&service.TransferPlan{}).Where("id = ?", copyPlan.PlanID).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err = agentClient.JSON(t.Context(), "GET", "/api/v1/workspace/tasks?kind=transfer_plan", "", nil, nil); err == nil {
		t.Fatal("Agent accessed human projection")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", copyPlan.OperationID).Update("space_id", "wrong").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, nil); status != 503 {
		t.Fatal("anchor drift hidden")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", copyPlan.OperationID).Update("space_id", h.space.ID).Error; err != nil {
		t.Fatal(err)
	}
	var source service.TransferItem
	if err = f.Service.DB.Where("plan_id = ?", copyPlan.PlanID).First(&source).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Delete(&service.Version{}, "id = ?", source.SourceVersionID).Error; err != nil {
		t.Fatal(err)
	}
	remaining := get("").Items
	if len(remaining) != 1 || remaining[0].OwnerID != migrate.PlanID {
		t.Fatal("missing fixed source remained visible or unrelated plan hidden")
	}
	if err = f.Service.DB.Migrator().DropTable(&service.TransferItem{}); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, nil); status != 500 {
		t.Fatal("DB failure hidden")
	}
}

func TestWorkspaceTransferPlanGate(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	if status := h.call(t, "GET", "/workspace/tasks?kind=transfer_plan", nil, nil); status != 400 {
		t.Fatalf("closed deployment gate %d", status)
	}
}
