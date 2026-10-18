package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

func TestPropertyMigrationHTTPClosedSDKAndRecovery(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "old", DisplayName: "Old", Type: "text"}}}, "definitions")
	if err != nil {
		t.Fatal(err)
	}
	path := "/spaces/" + h.space.ID + "/property-migrations"
	post := func(raw string, keys ...string) int {
		t.Helper()
		req, err := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewBufferString(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		for _, key := range keys {
			req.Header.Add("Idempotency-Key", key)
		}
		response, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	mutate := func(route string, input any, key string) contract.PropertyMigrationReceipt {
		t.Helper()
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1"+route, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		req.Header.Set("Idempotency-Key", key)
		response, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatal(route, response.StatusCode)
		}
		var envelope contract.DataEnvelope[contract.PropertyMigrationReceipt]
		if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	for _, raw := range []string{
		`{}`,
		`{"expected_definition_revision":1,"source_key":"old","target_key":"next","target_type":"text","conversion_policy":"preserve","owner":"spoof"}`,
		`{"expected_definition_revision":1,"source_key":"old","target_key":"next","target_type":"text","conversion_policy":"preserve","source_key":"other"}`,
		`{"expected_definition_revision":1,"source_key":"old","target_key":"next","target_type":"text"}`,
		`{"expected_definition_revision":1,"source_key":"old","target_key":"next","target_type":"text","conversion_policy":null}`,
	} {
		if status := post(raw, "invalid"); status != 400 {
			t.Fatal("invalid request accepted", status)
		}
	}
	q := sdk.PropertyMigrationPreview{ExpectedDefinitionRevision: 1, SourceKey: "old", TargetKey: "next", TargetType: "text", ConversionPolicy: "preserve"}
	raw, _ := json.Marshal(q)
	if status := post(string(raw)); status != 400 {
		t.Fatal("missing key accepted", status)
	}
	if status := post(string(raw), "a", "b"); status != 400 {
		t.Fatal("duplicate key accepted", status)
	}
	// mustSDK authenticates an Agent. Human-only migrations must reject it;
	// the lifecycle below uses the logged-in human session with CSRF protection.
	if _, err = client.PreviewPropertyMigration(t.Context(), h.space.ID, q, "agent-preview"); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent preview", err)
	}
	created := mutate(path, q, "preview")
	planPath := "/property-migrations/" + created.PlanID
	var plan contract.PropertyMigrationView
	if status := h.call(t, "GET", planPath, nil, &plan); status != 200 || !plan.EditingEnabled {
		t.Fatal(plan, status)
	}
	if _, err = client.GetPropertyMigration(t.Context(), created.PlanID); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent read", err)
	}
	confirm := sdk.PropertyMigrationDecision{ExpectedRevision: 1, ImpactDigest: plan.ImpactDigest}
	if _, err = client.ConfirmPropertyMigration(t.Context(), created.PlanID, confirm, "agent-confirm"); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent confirm", err)
	}
	confirmed := mutate(planPath+"/confirm", confirm, "confirm")
	execute := sdk.PropertyMigrationDecision{ExpectedRevision: confirmed.ResultRevision, ImpactDigest: plan.ImpactDigest}
	if _, err = client.ExecutePropertyMigration(t.Context(), created.PlanID, execute, "agent-execute"); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent execute", err)
	}
	first := mutate(planPath+"/execute", execute, "execute")
	repeat := mutate(planPath+"/execute", execute, "execute")
	if repeat != first {
		t.Fatal(repeat, first)
	}
	if _, err = client.LookupPropertyMigration(t.Context(), h.space.ID, sdk.PropertyMigrationLookup{Stage: "preview", IdempotencyKey: "preview"}); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent lookup", err)
	}
	if _, err = client.GetPropertyMigrationCapability(t.Context(), h.space.ID); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent capability", err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("observation wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("migration_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("migration_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("migration_no_write", guard)
	var lookup contract.PropertyMigrationReceipt
	var current contract.PropertyMigrationView
	var capability contract.PropertyMigrationCapability
	lookupStatus := h.call(t, "POST", path+"/lookup", sdk.PropertyMigrationLookup{Stage: "preview", IdempotencyKey: "preview"}, &lookup)
	currentStatus := h.call(t, "GET", planPath, nil, &current)
	capabilityStatus := h.call(t, "GET", path+"/capability", nil, &capability)
	f.Service.DB.Callback().Create().Remove("migration_no_write")
	f.Service.DB.Callback().Update().Remove("migration_no_write")
	f.Service.DB.Callback().Delete().Remove("migration_no_write")
	if writes != 0 || lookupStatus != 200 || currentStatus != 200 || capabilityStatus != 200 || lookup != created || current.Phase != "completed" || capability.MarkdownSupported {
		t.Fatal(writes, lookup, current, lookupStatus, currentStatus, capabilityStatus)
	}
	if status := bob.call(t, "GET", "/property-migrations/"+created.PlanID, nil, nil); status == 200 {
		t.Fatal("cross-user plan leaked")
	}
	f.Service.Config.PropertyEditing = false
	if status := h.call(t, "GET", planPath, nil, &current); status != 200 || current.EditingEnabled {
		t.Fatal(current, status)
	}
	if status := post(string(raw), "off"); status != 422 {
		t.Fatal("human gate bypass", status)
	}
}
