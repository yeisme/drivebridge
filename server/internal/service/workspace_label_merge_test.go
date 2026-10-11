package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLabelMergeHTTPLifecycleAndUnknownRecovery(t *testing.T) {
	f := rig(t)
	f.Service.Config.LabelManagement = true
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	humanJSON := func(method, path, key string, input, out any) int {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil { t.Fatal(err) }
		req, err := http.NewRequest(method, h.base+"/api/v1"+path, bytes.NewReader(body))
		if err != nil { t.Fatal(err) }
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		if key != "" { req.Header.Set("Idempotency-Key", key) }
		res, err := h.http.Do(req)
		if err != nil { t.Fatal(err) }
		defer res.Body.Close()
		var envelope struct { Data json.RawMessage `json:"data"` }
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil { t.Fatal(err) }
		if res.StatusCode == http.StatusOK && out != nil {
			if err = json.Unmarshal(envelope.Data, out); err != nil { t.Fatal(err) }
		}
		return res.StatusCode
	}
	dictionaryPath := "/spaces/"+h.space.ID+"/label-definitions"
	mergePath := "/spaces/"+h.space.ID+"/label-merge-plans"
	defs := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}, {Key: "target", DisplayName: "Target"}}}
	if _, err := c.UpdateLabelDefinitions(t.Context(), h.space.ID, defs, "agent-merge-defs"); err == nil { t.Fatal("Agent dictionary mutation accepted") }
	if status := humanJSON("PUT", dictionaryPath, "merge-defs", defs, nil); status != 200 { t.Fatal("human dictionary write", status) }
	refs := []string{}
	files := []service.File{}
	for i, name := range []string{"selected.txt", "untouched.txt"} {
		upload := orgUpload(t, c, h.space.ID, name, "fixed content")
		var file service.File
		if err := f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil { t.Fatal(err) }
		files = append(files, file)
		if _, err := f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source", "legacy label"}}); err != nil { t.Fatal(err) }
		if _, err := f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "suggested", Labels: []string{"source"}}); err != nil { t.Fatal(err) }
		if i == 0 { refs = append(refs, f.Service.FileRef(file)) }
	}
	input := contract.LabelMergePreview{Namespace: "manual", SourceKey: "source", TargetKey: "target", ExpectedRevision: 1, SourceRevision: 1, TargetRevision: 1, Refs: refs}
	var plan contract.LabelMergePlanView
	if status := humanJSON("POST", mergePath, "selected-merge", input, &plan); status != 200 || plan.Phase != "previewed" || len(plan.Items) != 1 || plan.Items[0].Before == "" { t.Fatal(plan, status) }
	if _, err := c.CreateLabelMergePlan(t.Context(), h.space.ID, input, "agent-merge"); err == nil { t.Fatal("Agent merge accepted") }
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("plan observation wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("label-plan-read", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("label-plan-read", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("label-plan-read", guard)
	readStatus := humanJSON("GET", "/organization-plans/"+plan.PlanID+"?limit=50", "", nil, nil)
	lookupStatus := humanJSON("POST", mergePath+"/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "selected-merge"}, nil)
	f.Service.DB.Callback().Create().Remove("label-plan-read")
	f.Service.DB.Callback().Update().Remove("label-plan-read")
	f.Service.DB.Callback().Delete().Remove("label-plan-read")
	if writes != 0 || readStatus != 200 || lookupStatus != 200 { t.Fatal("plan reads wrote", writes, readStatus, lookupStatus) }
	if status := humanJSON("POST", "/organization-plans/"+plan.PlanID+"/execute", "", contract.LabelMergeExecute{ExpectedRevision: plan.Revision}, nil); status == 200 { t.Fatal("unconfirmed human execution") }
	var confirmed service.OrganizationPlanView
	confirmPath := "/organization-plans/"+plan.PlanID+"/confirm"
	for _, bad := range []string{`{}`, `{"expected_revision":1,"impact_digest":"x","impact_digest":"y"}`, `{"expected_revision":1,"impact_digest":"x","owner":"spoof"}`} {
		if status, _ := fpRawCall(t, h, "POST", confirmPath, []byte(bad)); status != 400 { t.Fatal("open confirmation", status) }
	}
	if status := h.call(t, "POST", confirmPath, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: "changed"}, nil); status != 409 { t.Fatal("changed digest", status) }
	if status := h.call(t, "POST", confirmPath, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest}, &confirmed); status != 200 { t.Fatal("confirm", status) }
	var completed contract.LabelMergePlanView
	if status := humanJSON("POST", "/organization-plans/"+plan.PlanID+"/execute", "", contract.LabelMergeExecute{ExpectedRevision: confirmed.Revision}, &completed); status != 200 || completed.Phase != "completed" || completed.ItemsDone != 1 { t.Fatal(completed, status) }
	// Discarding the submit result is recovered from the original request, not a new preview.
	var recovered contract.LabelMergePlanView
	if status := humanJSON("POST", mergePath+"/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "selected-merge"}, &recovered); status != 200 || recovered.PlanID != plan.PlanID || recovered.ItemsDone != 1 { t.Fatal("unknown recovery", recovered, status) }
	var replay contract.LabelMergePlanView
	if status := humanJSON("POST", mergePath, "selected-merge", input, &replay); status != 200 || replay.PlanID != plan.PlanID { t.Fatal("preview replay", replay, status) }
	for i, file := range files {
		view, err := f.Service.LabelAssignmentsFor(p, file.ID)
		if err != nil { t.Fatal(err) }
		labels := map[string]bool{}
		for _, label := range view.Namespaces["manual"] { labels[label] = true }
		if !labels["legacy label"] || len(view.Namespaces["suggested"]) != 1 { t.Fatal("unrelated assignments lost", view) }
		if i == 0 && (labels["source"] || !labels["target"]) { t.Fatal("selected source not merged", view) }
		if i == 1 && (!labels["source"] || labels["target"]) { t.Fatal("unselected file mutated", view) }
	}
	var page contract.LabelDefinitionPage
	if status := humanJSON("GET", dictionaryPath+"?limit=200", "", nil, &page); status != 200 || page.Revision != 1 || len(page.Definitions) != 2 { t.Fatal("dictionary identities deleted", page, status) }
	if status := bob.call(t, "GET", "/organization-plans/"+plan.PlanID, nil, nil); status != 404 { t.Fatal("foreign plan leaked", status) }
	f.Service.Config.LabelManagement = false
	if status := humanJSON("POST", mergePath+"/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "selected-merge"}, nil); status != 200 { t.Fatal("disabled recovery denied", status) }
	if status := humanJSON("POST", mergePath, "disabled", input, nil); status != 422 { t.Fatal("disabled mutation", status) }
}

func TestWorkspaceLabelMergeRollbackDriftAndRevocation(t *testing.T) {
	for _, scenario := range []string{"assignment_drift", "version_drift", "catalog_drift", "revoked", "outcome_failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.LabelManagement = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			c := mustSDK(t, h)
			defs := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}, {Key: "target", DisplayName: "Target"}}}
			if _, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, defs, "definitions"); err != nil { t.Fatal(err) }
			upload := orgUpload(t, c, h.space.ID, "merge.txt", "fixed content")
			var file service.File
			if err := f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil { t.Fatal(err) }
			if _, err := f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source"}}); err != nil { t.Fatal(err) }
			plan, err := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, contract.LabelMergePreview{Namespace: "manual", SourceKey: "source", TargetKey: "target", ExpectedRevision: 1, SourceRevision: 1, TargetRevision: 1, Refs: []string{f.Service.FileRef(file)}}, "preview")
			if err != nil { t.Fatal(err) }
			confirmed, err := f.Service.ConfirmOrganizationPlan(p, plan.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest})
			if err != nil { t.Fatal(err) }
			switch scenario {
			case "assignment_drift":
				if _, err = f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source", "new"}}); err != nil { t.Fatal(err) }
			case "version_drift":
				if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Update("version_id", service.ID()).Error; err != nil { t.Fatal(err) }
			case "catalog_drift":
				defs.ExpectedRevision = 1
				defs.Definitions[0].DisplayName = "Changed"
				if _, err = f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, defs, "changed"); err != nil { t.Fatal(err) }
			case "revoked":
				if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil { t.Fatal(err) }
			case "outcome_failure":
				f.Service.DB.Callback().Update().Before("gorm:update").Register("merge_outcome_fail", func(tx *gorm.DB) { if tx.Statement.Table == "organization_plan_items" { tx.AddError(errors.New("injected outcome failure")) } })
			}
			result, executeErr := f.Service.ExecuteOrganizationPlan(p, plan.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: confirmed.Revision})
			if scenario == "outcome_failure" { f.Service.DB.Callback().Update().Remove("merge_outcome_fail") }
			if executeErr == nil && (result.Phase != "partial" || result.ItemsStale != 1) { t.Fatal("changed facts executed", result, executeErr) }
			var rows []service.FileLabelAssignment
			if err = f.Service.DB.Where("file_id = ? AND namespace = ?", file.ID, "manual").Find(&rows).Error; err != nil { t.Fatal(err) }
			for _, row := range rows { if row.Label == "target" { t.Fatal("failed merge mutated assignments") } }
			if scenario == "outcome_failure" {
				if executeErr == nil { t.Fatal("failure swallowed") }
				current, err := f.Service.GetOrganizationPlan(p, plan.PlanID, "", 50)
				if err != nil || current.Phase != "executing" || current.Items[0].Status != "pending" { t.Fatal("unknown lost original state", current, err) }
				retried, err := f.Service.ExecuteOrganizationPlan(p, plan.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: current.Revision})
				if err != nil || retried.ItemsDone != 1 { t.Fatal("original execution not resumable", retried, err) }
			}
		})
	}
}
