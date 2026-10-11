package service_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestPropertyMigrationAtomicCutoverAndReceipt(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "migration/image.png", "binary fixture")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "rank", DisplayName: "Rank", Type: "text"}}}, "migration-definitions")
	if err != nil { t.Fatal(err) }
	_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"rank": json.RawMessage(`"12.50"`)}, RemoveKeys: []string{}}, "migration-value")
	if err != nil { t.Fatal(err) }
	if err = f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Where("file_id = ?", file.FileID).Update("values_json", `{"rank":"12.50","legacy":{ "nested" : [true, null] }}`).Error; err != nil { t.Fatal(err) }
	q := contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "rank", TargetKey: "score", TargetType: "number", ConversionPolicy: "parse_text"}
	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, q, "migration-preview")
	if err != nil { t.Fatal(err) }
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].After != "12.50" { t.Fatal(plan, err) }
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: plan.Revision, ImpactDigest: plan.ImpactDigest}, "migration-confirm")
	if err != nil { t.Fatal(err) }
	execute := contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}
	// A late receipt failure rolls back sidecars, dictionary and item outcomes.
	hits := 0
	f.Service.DB.Callback().Create().Before("gorm:create").Register("migration_receipt_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" { hits++; tx.AddError(errors.New("synthetic receipt failure")) }
	})
	_, err = f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "migration-execute")
	f.Service.DB.Callback().Create().Remove("migration_receipt_failure")
	if hits != 1 || err == nil || err.Error() != "database_unavailable" { t.Fatal(hits, err) }
	old, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || string(old.Values["rank"]) != `"12.50"` || old.DefinitionRevision != 1 { t.Fatal(old, err) }
	receipt, err := f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "migration-execute")
	if err != nil { t.Fatal(err) }
	replay, err := f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "migration-execute")
	if err != nil || replay != receipt { t.Fatal(replay, err) }
	execute.ImpactDigest = strings.Repeat("0", 64)
	if _, err = f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "migration-execute"); err == nil || err.Error() != "idempotency_conflict" { t.Fatal(err) }
	current, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || current.Revision != 2 || current.DefinitionRevision != 2 || string(current.Values["score"]) != "12.50" || current.Values["rank"] != nil { t.Fatal(current, err) }
	var stored service.WorkspacePropertySidecar
	if err = f.Service.DB.First(&stored, "file_id = ?", file.FileID).Error; err != nil || !strings.Contains(stored.ValuesJSON, `{ "nested" : [true, null] }`) { t.Fatal(stored, err) }
	lookup, err := f.Service.LookupPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationLookup{Stage: "preview", IdempotencyKey: "migration-preview"})
	if err != nil || lookup != created { t.Fatal(lookup, err) }
}
