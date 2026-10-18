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

func TestPropertyMigrationConflictAndIncompatibleOutcomes(t *testing.T) {
	for _, scenario := range []string{"incompatible", "target_collision", "same_revision_drift", "revision_drift", "head_drift", "new_sidecar"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.PropertyEditing = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			client := mustSDK(t, h)
			file := orgUpload(t, client, h.space.ID, "migration/conflict.png", "binary fixture")
			_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
			if err != nil {
				t.Fatal(err)
			}
			value := `"12"`
			if scenario == "incompatible" {
				value = `"not a number"`
			}
			_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": json.RawMessage(value)}, RemoveKeys: []string{}}, "value")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "target_collision" {
				if err = f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Where("file_id = ?", file.FileID).Update("values_json", `{"old":"12","next":{"unknown":true}}`).Error; err != nil {
					t.Fatal(err)
				}
			}
			q := contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "number", ConversionPolicy: "parse_text"}
			receipt, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, q, "preview")
			if err != nil {
				t.Fatal(err)
			}
			plan, err := f.Service.GetPropertyMigration(t.Context(), p, receipt.PlanID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "incompatible" && plan.Items[0].Error != "incompatible_value" {
				t.Fatal(plan)
			}
			if scenario == "target_collision" && plan.Items[0].Error != "target_key_exists" {
				t.Fatal(plan)
			}
			changed := q
			changed.TargetKey = "different"
			if _, err = f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, changed, "preview"); err == nil || err.Error() != "idempotency_conflict" {
				t.Fatal(err)
			}
			confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "same_revision_drift":
				err = f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Where("file_id = ?", file.FileID).Update("values_json", `{"old":"13"}`).Error
			case "revision_drift":
				err = f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Where("file_id = ?", file.FileID).Update("revision", 2).Error
			case "head_drift":
				var version service.Version
				if err = f.Service.DB.First(&version, "id = ?", file.VersionID).Error; err != nil {
					t.Fatal(err)
				}
				version.ID = service.ID()
				if err = f.Service.DB.Create(&version).Error; err != nil {
					t.Fatal(err)
				}
				err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.FileID).Update("version_id", version.ID).Error
			case "new_sidecar":
				other := orgUpload(t, client, h.space.ID, "migration/new.png", "new fixture")
				_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, other.FileID, contract.PropertySidecarPatch{BaseVersionID: other.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": json.RawMessage(`"14"`)}, RemoveKeys: []string{}}, "new-sidecar")
			}
			if err != nil {
				t.Fatal(err)
			}
			execute := contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}
			first, err := f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "execute")
			if scenario == "new_sidecar" {
				if err == nil || err.Error() != "revision_conflict" {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				repeat, err := f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, execute, "execute")
				if err != nil || repeat != first {
					t.Fatal(repeat, err)
				}
				current, err := f.Service.GetPropertyMigration(t.Context(), p, plan.PlanID)
				if err != nil || current.Phase != "partial" || current.Items[0].Status == "done" {
					t.Fatal(current, err)
				}
			}
			definitions, err := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
			if err != nil || definitions.Revision != 1 || definitions.Definitions[0].Key != "old" {
				t.Fatal(definitions, err)
			}
		})
	}
}

func TestPropertyMigrationGateRevocationAndReadFailures(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "migration/auth.pdf", "fixture")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
	if err != nil {
		t.Fatal(err)
	}
	q := contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}
	if _, err = f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, q, "preview"); err == nil || err.Error() != "unsupported_action" {
		t.Fatal(err)
	}
	capability, err := f.Service.PropertyMigrationCapability(t.Context(), p, h.space.ID)
	if err != nil || capability.EditingEnabled || capability.MarkdownSupported {
		t.Fatal(capability, err)
	}
	f.Service.Config.PropertyEditing = true
	_, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"old": json.RawMessage(`"keep"`)}, RemoveKeys: []string{}}, "value")
	if err != nil {
		t.Fatal(err)
	}
	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, q, "preview")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.GetPropertyMigration(t.Context(), bad, plan.PlanID); err == nil {
			t.Fatal("unauthorized preview")
		}
		if _, err = f.Service.LookupPropertyMigration(t.Context(), bad, h.space.ID, contract.PropertyMigrationLookup{Stage: "preview", IdempotencyKey: "preview"}); err == nil {
			t.Fatal("unauthorized receipt")
		}
	}
	for _, table := range []string{"organization_plans", "organization_plan_items", "workspace_requests", "workspace_property_sidecars", "workspace_file_property_projections"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("migration_read_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic migration read failure"))
			}
		})
		_, failure := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, q, "fresh-"+table)
		if table == "organization_plans" || table == "organization_plan_items" {
			_, failure = f.Service.GetPropertyMigration(t.Context(), p, plan.PlanID)
		}
		f.Service.DB.Callback().Query().Remove("migration_read_failure")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal(table, hits, failure)
		}
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.Service.ExecutePropertyMigration(t.Context(), p, plan.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}, "execute")
	if err == nil {
		t.Fatal("revoked file migrated")
	}
	if _, err = f.Service.LookupPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationLookup{Stage: "preview", IdempotencyKey: "preview"}); err == nil {
		t.Fatal("revoked preview leaked")
	}
}

func TestPropertyMigrationUnsupportedProjectionAndBudget(t *testing.T) {
	for _, scenario := range []string{"frontmatter", "budget"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.PropertyEditing = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
			if err != nil {
				t.Fatal(err)
			}
			want := "unsupported_action"
			if scenario == "frontmatter" {
				err = f.Service.DB.Create(&service.WorkspaceFilePropertyProjection{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, FileID: service.ID(), VersionID: service.ID(), Key: "old", StorageKind: "frontmatter", DefinitionRevision: 1, Revision: 1, ValueJSON: `"old"`}).Error
			} else {
				want = "query_budget_exceeded"
				rows := make([]service.WorkspacePropertySidecar, 201)
				for i := range rows {
					rows[i] = service.WorkspacePropertySidecar{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, FileID: service.ID(), VersionID: service.ID(), Revision: 1, DefinitionRevision: 1, ValuesJSON: `{}`}
				}
				err = f.Service.DB.Create(&rows).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}, strings.Repeat("a", 20))
			if err == nil || err.Error() != want {
				t.Fatal(err)
			}
			var count int64
			if err = f.Service.DB.Model(&service.OrganizationPlan{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}
