package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceOrganizationPlanProjection(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	upload := orgUpload(t, c, h.space.ID, "plan/source.txt", "source")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil {
		t.Fatal(err)
	}
	input := service.CreateOrganizationPlanRequest{Refs: []string{f.Service.FileRef(file)}, Actions: []service.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "reviewed"}, {Kind: "move", TargetPrefix: "organized"}}}
	plan, err := f.Service.CreateOrganizationPlan(p, input, "plan-projection")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateOrganizationPlan(p, input, "second-projection"); err != nil {
		t.Fatal(err)
	}
	get := func(query string) contract.Page[contract.TaskProjection] {
		t.Helper()
		var page contract.Page[contract.TaskProjection]
		if status := h.call(t, "GET", "/workspace/tasks?kind=organization_plan&"+query, nil, &page); status != 200 {
			t.Fatalf("plan list %d", status)
		}
		return page
	}
	first := get("limit=1")
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("missing pagination")
	}
	for _, view := range get("").Items {
		if view.OwnerKind != "organization_plan" || view.UIState != "waiting_confirmation" || view.ActorKind != "unknown" || view.AgentID != nil || view.Progress.Total != nil || view.Progress.Succeeded != nil || len(view.AllowedActions) != 0 {
			t.Fatal("invented creator/progress/execution")
		}
	}
	if len(get("agent_id=unproven-agent").Items) != 0 {
		t.Fatal("unrecorded creator attributed to Agent")
	}
	var hidden contract.Page[contract.TaskProjection]
	if status := bob.call(t, "GET", "/workspace/tasks?kind=organization_plan", nil, &hidden); status != 200 || len(hidden.Items) != 0 {
		t.Fatal("foreign plans visible")
	}
	next := "/workspace/tasks?kind=organization_plan&limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	if status := h.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatal("stable cursor failed")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("cross actor cursor admitted")
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=operation&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("cross kind cursor admitted")
	}
	if err = f.Service.DB.Model(&service.OrganizationPlan{}).Where("id = ?", plan.PlanID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 1 {
		t.Fatal("cross tenant plan visible")
	}
	if err = f.Service.DB.Model(&service.OrganizationPlan{}).Where("id = ?", plan.PlanID).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	confirmed, err := f.Service.ConfirmOrganizationPlan(p, plan.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest})
	if err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("changed plan failed to invalidate cursor")
	}
	if len(get("state=running").Items) != 1 {
		t.Fatal("confirmed plan missing")
	}
	completed, err := f.Service.ExecuteOrganizationPlan(p, plan.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: confirmed.Revision})
	if err != nil || completed.Phase != "completed" {
		t.Fatal("real owner execution failed", err)
	}
	views := get("state=unknown").Items
	if len(views) != 1 || views[0].OwnerID != plan.PlanID || views[0].OwnerStatus != "completed" || views[0].BlockedReason != "organization_result_unproven" {
		t.Fatal("owner completed promoted to proven action success")
	}
	if err = f.Service.DB.First(&file, "id = ?", file.ID).Error; err != nil || file.Path != "organized/source.txt" {
		t.Fatal("owner move did not execute")
	}
	var labels int64
	if err = f.Service.DB.Model(&service.FileLabelAssignment{}).Where("file_id = ? AND label = ?", file.ID, "reviewed").Count(&labels).Error; err != nil || labels != 1 {
		t.Fatal("owner label did not execute")
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err = agentClient.JSON(t.Context(), "GET", "/api/v1/workspace/tasks?kind=organization_plan", "", nil, nil); err == nil {
		t.Fatal("Agent read human plan list")
	}
	if err = f.Service.DB.Delete(&service.File{}, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 0 {
		t.Fatal("source visibility loss did not hide whole plans")
	}
	if err = f.Service.DB.Migrator().DropTable(&service.OrganizationPlanItem{}); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=organization_plan", nil, nil); status != 500 {
		t.Fatal("item database failure became empty")
	}
}

func TestWorkspaceOrganizationPlanRequestBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	plans := []service.OrganizationPlan{{ID: "a", Owner: h.user.ID, SpaceID: h.space.ID, Key: "a", Phase: "previewed", Revision: 1}, {ID: "b", Owner: h.user.ID, SpaceID: h.space.ID, Key: "b", Phase: "previewed", Revision: 1}}
	if err := f.Service.DB.Create(&plans).Error; err != nil {
		t.Fatal(err)
	}
	file := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: "fixture.txt"}
	if err := f.Service.DB.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]service.OrganizationPlanItem, 1001)
	for i := range rows {
		plan := "a"
		if i >= 500 {
			plan = "b"
		}
		rows[i] = service.OrganizationPlanItem{ID: service.ID(), PlanID: plan, Owner: h.user.ID, ItemKey: service.ID(), FileID: file.ID, Path: file.Path, Status: "pending"}
	}
	if err := f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=organization_plan&limit=1", nil, nil); status != 429 {
		t.Fatalf("whole request budget %d", status)
	}
	if err := f.Service.DB.Delete(&rows[1000]).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=organization_plan&limit=1", nil, nil); status != 200 {
		t.Fatalf("bounded query %d", status)
	}
	if err := f.Service.DB.Model(&service.OrganizationPlanItem{}).Where("id = ?", rows[0].ID).Update("owner", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=organization_plan", nil, nil); status != 503 {
		t.Fatal("item binding drift hidden")
	}
}
