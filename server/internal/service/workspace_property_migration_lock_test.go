package service_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestPropertyMigrationAndSidecarUseCatalogBeforeFileLock(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "migration/lock-order.png", "fixture")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
	if err != nil { t.Fatal(err) }

	// Observe typed GORM locking clauses before the SQLite dialect drops FOR
	// UPDATE. This verifies application acquisition order, not PostgreSQL locking.
	locks := []string{}
	const callback = "property_lock_order"
	if err = f.Service.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		locking, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" { return }
		switch tx.Statement.Table {
		case "workspace_property_catalogs", "files":
			locks = append(locks, tx.Statement.Table)
		}
	}); err != nil { t.Fatal(err) }
	t.Cleanup(func() { f.Service.DB.Callback().Query().Remove(callback) })

	_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": json.RawMessage(`"keep"`)}, RemoveKeys: []string{}}, "value")
	if err != nil { t.Fatal(err) }
	want := []string{"workspace_property_catalogs", "files"}
	if !slices.Equal(locks, want) { t.Fatalf("sidecar lock order %v, want %v", locks, want) }

	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}, "preview")
	if err != nil { t.Fatal(err) }
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil { t.Fatal(err) }
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, created.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
	if err != nil { t.Fatal(err) }
	locks = nil
	_, err = f.Service.ExecutePropertyMigration(t.Context(), p, created.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}, "execute")
	if err != nil { t.Fatal(err) }
	if !slices.Equal(locks, want) { t.Fatalf("migration lock order %v, want %v", locks, want) }
}
