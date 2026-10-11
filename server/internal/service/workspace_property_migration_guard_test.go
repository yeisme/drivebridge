package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestPropertyMigrationLegacyPlanRoutesCannotBypassDecision(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
	if err != nil { t.Fatal(err) }
	created, err := f.Service.PreviewPropertyMigration(t.Context(), p, h.space.ID, contract.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}, "preview")
	if err != nil { t.Fatal(err) }
	plan, err := f.Service.GetPropertyMigration(t.Context(), p, created.PlanID)
	if err != nil { t.Fatal(err) }
	if _, err = f.Service.GetOrganizationPlan(p, created.PlanID, "", 200); err == nil { t.Fatal("legacy view exposed property values") }
	if _, err = f.Service.ConfirmOrganizationPlan(p, created.PlanID, service.ConfirmOrganizationPlanRequest{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}); err == nil { t.Fatal("legacy confirmation bypassed key") }
	confirmed, err := f.Service.ConfirmPropertyMigration(t.Context(), p, created.PlanID, contract.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}, "confirm")
	if err != nil { t.Fatal(err) }
	if _, err = f.Service.ExecuteOrganizationPlan(p, created.PlanID, service.ExecuteOrganizationPlanRequest{ExpectedRevision: confirmed.ResultRevision}); err == nil { t.Fatal("legacy execute bypassed digest and key") }
	if _, err = f.Service.CreateCompensationPlan(p, created.PlanID, "compensate"); err == nil { t.Fatal("generic compensation accepted property migration") }
}
