package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLabelMergePreviewRollbackAndRestart(t *testing.T) {
	f := rig(t)
	f.Service.Config.LabelManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	defs := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}, {Key: "target", DisplayName: "Target"}}}
	receipt, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, defs, "definitions")
	if err != nil { t.Fatal(err) }
	upload := orgUpload(t, mustSDK(t, h), h.space.ID, "merge.txt", "fixed content")
	var file service.File
	if err = f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil { t.Fatal(err) }
	if _, err = f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source", "target", "other"}}); err != nil { t.Fatal(err) }
	input := contract.LabelMergePreview{Namespace: "manual", SourceKey: "source", TargetKey: "target", ExpectedRevision: 1, SourceRevision: 1, TargetRevision: 1, Refs: []string{f.Service.FileRef(file)}}
	for _, table := range []string{"organization_plans", "organization_plan_items", "workspace_requests"} {
		hits := 0
		f.Service.DB.Callback().Create().Before("gorm:create").Register("preview-failure", func(tx *gorm.DB) { if tx.Statement.Table == table { hits++; tx.AddError(errors.New("injected preview persistence failure")) } })
		_, failure := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, input, "preview")
		f.Service.DB.Callback().Create().Remove("preview-failure")
		if hits != 1 || failure == nil { t.Fatal("preview failure ignored", table, failure) }
		var count int64
		if err = f.Service.DB.Model(&service.OrganizationPlan{}).Count(&count).Error; err != nil || count != 0 { t.Fatal("partial preview", count, err) }
		if _, err = f.Service.LookupLabelMergePlan(t.Context(), p, h.space.ID, "preview"); err == nil || err.Error() != "not_found" { t.Fatal("uncommitted receipt", err) }
	}
	invalid := input
	invalid.Refs = append(append([]string{}, input.Refs...), input.Refs...)
	if _, err = f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, invalid, "duplicate"); err == nil { t.Fatal("duplicate files accepted") }
	invalid.Refs = make([]string, 201)
	if _, err = f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, invalid, "unbounded"); err == nil { t.Fatal("unbounded plan accepted") }
	plan, err := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, input, "preview")
	if err != nil { t.Fatal(err) }
	confirmed, err := f.Service.ConfirmOrganizationPlan(p, plan.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest})
	if err != nil { t.Fatal(err) }
	completed, err := f.Service.ExecuteOrganizationPlan(p, plan.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: confirmed.Revision})
	if err != nil || completed.ItemsDone != 1 { t.Fatal(completed, err) }
	cfg := f.Service.Config
	f.App.Close()
	f.Service.Close()
	reopened, err := service.Open(cfg)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	recovered, err := reopened.LookupLabelMergePlan(t.Context(), p, h.space.ID, "preview")
	if err != nil || recovered.PlanID != plan.PlanID || recovered.ItemsDone != 1 || recovered.Phase != "completed" { t.Fatal("restart recovery", recovered, err) }
	original, err := reopened.LookupLabelDefinitionUpdate(t.Context(), p, h.space.ID, "definitions")
	if err != nil || original != receipt { t.Fatal("restart dictionary receipt", original, err) }
	view, err := reopened.LabelAssignmentsFor(p, file.ID)
	if err != nil || len(view.Namespaces["manual"]) != 2 { t.Fatal("target duplicated", view, err) }
	for _, value := range view.Namespaces["manual"] { if value == "source" { t.Fatal("source retained") } }
}
