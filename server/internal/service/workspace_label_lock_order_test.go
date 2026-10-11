package service_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLabelMergeStableFileLockOrder(t *testing.T) {
	f := rig(t)
	f.Service.Config.LabelManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	defs := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}, {Key: "target", DisplayName: "Target"}}}
	if _, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, defs, "dictionary"); err != nil { t.Fatal(err) }
	files := make([]service.File, 0, 2)
	for _, name := range []string{"first.txt", "second.txt"} {
		upload := orgUpload(t, mustSDK(t, h), h.space.ID, name, "fixed")
		var file service.File
		if err := f.Service.DB.First(&file, "id = ?", upload.FileID).Error; err != nil { t.Fatal(err) }
		if _, err := f.Service.SetFileLabels(p, file.ID, service.SetFileLabelsRequest{Namespace: "manual", Labels: []string{"source"}}); err != nil { t.Fatal(err) }
		files = append(files, file)
	}
	slices.SortFunc(files, func(a, b service.File) int {
		if a.ID < b.ID { return -1 }
		if a.ID > b.ID { return 1 }
		return 0
	})
	want := []string{files[0].ID, files[1].ID}
	var acquired []string
	// SQLite omits FOR UPDATE from SQL, but the actual service query must still
	// request the lock. Observe executed file queries, not source text or SQL.
	if err := f.Service.DB.Callback().Query().After("gorm:query").Register("label-lock-order", func(tx *gorm.DB) {
		if _, locking := tx.Statement.Clauses["FOR"]; !locking { return }
		if file, ok := tx.Statement.Dest.(*service.File); ok && tx.Error == nil { acquired = append(acquired, file.ID) }
	}); err != nil { t.Fatal(err) }
	defer f.Service.DB.Callback().Query().Remove("label-lock-order")
	for _, reverse := range []bool{false, true} {
		refs := []string{f.Service.FileRef(files[0]), f.Service.FileRef(files[1])}
		key := "ascending"
		if reverse { slices.Reverse(refs); key = "descending" }
		input := contract.LabelMergePreview{Namespace: "manual", SourceKey: "source", TargetKey: "target", ExpectedRevision: 1, SourceRevision: 1, TargetRevision: 1, Refs: refs}
		acquired = nil
		plan, err := f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, input, key)
		if err != nil { t.Fatal(err) }
		if !reflect.DeepEqual(acquired, want) { t.Fatal("preview lock order", key, acquired, want) }
		var persisted service.OrganizationPlan
		if err = f.Service.DB.First(&persisted, "id = ?", plan.PlanID).Error; err != nil { t.Fatal(err) }
		var actions []service.OrganizationAction
		if err = json.Unmarshal([]byte(persisted.Actions), &actions); err != nil || len(actions) != 1 { t.Fatal(actions, err) }
		var original contract.LabelMergePreview
		if err = json.Unmarshal([]byte(actions[0].Value), &original); err != nil || !reflect.DeepEqual(original.Refs, refs) { t.Fatal("caller intent order changed", original, err) }
		acquired = nil
		if _, err = f.Service.ConfirmOrganizationPlan(p, plan.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest}); err != nil { t.Fatal(err) }
		if !reflect.DeepEqual(acquired, want) { t.Fatal("confirmation lock order", key, acquired, want) }
		changed := input
		changed.Refs = slices.Clone(refs)
		slices.Reverse(changed.Refs)
		if _, err = f.Service.CreateLabelMergePlan(t.Context(), p, h.space.ID, changed, key); err == nil || err.Error() != "idempotency_conflict" { t.Fatal("original request digest changed", err) }
	}
}
