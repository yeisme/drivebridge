package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestPropertyMigrationHistoricalVersionAndWriteBoundaries(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "migration/history.png", "fixture")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
	if err != nil { t.Fatal(err) }
	_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": json.RawMessage(`"old version"`)}, RemoveKeys: []string{}}, "value")
	if err != nil { t.Fatal(err) }
	var version service.Version
	if err = f.Service.DB.First(&version, "id = ?", file.VersionID).Error; err != nil { t.Fatal(err) }
	version.ID = service.ID()
	if err = f.Service.DB.Create(&version).Error; err != nil { t.Fatal(err) }
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.FileID).Update("version_id", version.ID).Error; err != nil { t.Fatal(err) }
	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}, "preview")
	if err != nil { t.Fatal(err) }
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil { t.Fatal(err) }
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
	if err != nil { t.Fatal(err) }
	q := contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}
	for _, table := range []string{"workspace_property_catalogs", "workspace_property_definitions", "workspace_property_sidecars", "organization_plan_items", "organization_plans"} {
		hits := 0
		f.Service.DB.Callback().Update().Before("gorm:update").Register("migration_update_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table { hits++; tx.AddError(errors.New("synthetic update failure")) }
		})
		_, failure := f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, q, "execute")
		f.Service.DB.Callback().Update().Remove("migration_update_failure")
		if hits != 1 || failure == nil || failure.Error() != "database_unavailable" { t.Fatal(table, hits, failure) }
		values, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
		if err != nil || values.Revision != 1 || values.DefinitionRevision != 1 || string(values.Values["old"]) != `"old version"` { t.Fatal(values, err) }
	}
	if _, err = f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, q, "execute"); err != nil { t.Fatal(err) }
	values, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || values.VersionID != file.VersionID || string(values.Values["next"]) != `"old version"` { t.Fatal(values, err) }
	fresh, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, version.ID)
	if err != nil || fresh.Revision != 0 || len(fresh.Values) != 0 { t.Fatal(fresh, err) }
	var head service.File
	if err = f.Service.DB.First(&head, "id = ?", file.FileID).Error; err != nil || head.VersionID != version.ID { t.Fatal(head, err) }
}

func TestPropertyMigrationReferenceRevocation(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	file := orgUpload(t, client, h.space.ID, "migration/ref.png", "fixture")
	target := orgUpload(t, client, h.space.ID, "migration/target.txt", "reference target")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "reference"}}}, "definitions")
	if err != nil { t.Fatal(err) }
	ref, _ := json.Marshal(f.Service.FileRef(service.File{ID: target.FileID, VersionID: target.VersionID}))
	_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": ref}, RemoveKeys: []string{}}, "value")
	if err != nil { t.Fatal(err) }
	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "reference", ConversionPolicy: "preserve"}, "preview")
	if err != nil { t.Fatal(err) }
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil { t.Fatal(err) }
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
	if err != nil { t.Fatal(err) }
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", target.FileID).Update("trashed", true).Error; err != nil { t.Fatal(err) }
	if _, err = f.Service.GetPropertyMigration(t.Context(), p, plan.PlanID); err == nil { t.Fatal("reference disclosed after revocation") }
	if _, err = f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}, "execute"); err == nil { t.Fatal("reference migrated after revocation") }
}
