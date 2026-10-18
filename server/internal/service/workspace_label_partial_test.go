package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceLabelMergePartialFrozenOutcomesAndScopedCursor(t *testing.T) {
	f := rig(t)
	f.Service.Config.LabelManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	defs := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}, {Key: "target", DisplayName: "Target"}}}
	if _, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, defs, "dictionary"); err != nil {
		t.Fatal(err)
	}
	refs := []string{}
	files := []service.File{}
	for _, name := range []string{"first.txt", "second.txt"} {
		upload := orgUpload(t, mustSDK(t, h), h.space.ID, name, "fixed")
		var file service.File
		if err := f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source"}}); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
		refs = append(refs, f.Service.FileRef(file))
	}
	input := contract.LabelMergePreview{Namespace: "manual", SourceKey: "source", TargetKey: "target", ExpectedRevision: 1, SourceRevision: 1, TargetRevision: 1, Refs: refs}
	plan, err := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, input, "preview")
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.Service.GetOrganizationPlan(p, plan.PlanID, "", 1)
	if err != nil || len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("missing bounded page", first, err)
	}
	second, err := f.Service.GetOrganizationPlan(p, plan.PlanID, first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 || second.Items[0].FileID == first.Items[0].FileID {
		t.Fatal("unstable page", second, err)
	}
	other, err := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, input, "another-preview")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.GetOrganizationPlan(p, other.PlanID, first.NextCursor, 1); err == nil {
		t.Fatal("cross-plan cursor reused")
	}
	confirmed, err := f.Service.ConfirmOrganizationPlan(p, plan.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.GetOrganizationPlan(p, plan.PlanID, first.NextCursor, 1); err == nil {
		t.Fatal("old revision cursor reused")
	}
	if _, err = f.Service.SetFileLabels(p, files[0].ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source", "new-label"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ConfirmOrganizationPlan(p, other.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: other.Revision, ImpactDigest: other.ImpactDigest}); err == nil {
		t.Fatal("changed assignments confirmed")
	}
	result, err := f.Service.ExecuteOrganizationPlan(p, plan.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: confirmed.Revision})
	if err != nil || result.Phase != "partial" || result.ItemsDone != 1 || result.ItemsStale != 1 {
		t.Fatal("mixed outcomes", result, err)
	}
	for _, item := range result.Items {
		if item.Status == "stale" && item.ResultVersion != "" {
			t.Fatal("uncommitted result version claimed")
		}
	}
	f.Service.Config.LabelManagement = false
	recovered, err := f.Service.LookupLabelMergePlan(t.Context(), p, h.space.ID, "preview")
	if err != nil || recovered.ItemsDone != 1 || recovered.ItemsStale != 1 {
		t.Fatal("disabled recovery", recovered, err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.GetOrganizationPlan(p, plan.PlanID, "", 50); err == nil {
		t.Fatal("revoked plan read")
	}
	if _, err = f.Service.LookupLabelMergePlan(t.Context(), p, h.space.ID, "preview"); err == nil {
		t.Fatal("revoked lookup")
	}
}
