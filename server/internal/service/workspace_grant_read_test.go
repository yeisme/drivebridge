package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantReadHTTPIsolation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, _ := agent(t, alice, "refs", "read")
	now := time.Now().UTC()
	grant := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: alice.user.ID, TenantID: "", SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Prefix: a.Prefix, Protocol: "http", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	var raw json.RawMessage
	if status := alice.call(t, "GET", "/agent-grants/"+grant.ID, nil, &raw); status != 200 {
		t.Fatalf("owner status %d", status)
	}
	var view contract.GrantView
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if view.GrantID != grant.ID || view.Protocol != "http" || view.ExpiresAt != nil || view.State != "active" {
		t.Fatalf("incorrect grant projection: %+v", view)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"token_hash", "secret", "credential"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("private field %s", key)
		}
	}
	if status := bob.call(t, "GET", "/agent-grants/"+grant.ID, nil, nil); status != 404 {
		t.Fatalf("cross-user status %d", status)
	}
	if err := f.Service.DB.Model(&grant).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/agent-grants/"+grant.ID, nil, &view); status != 200 || view.State != "disabled" {
		t.Fatal("disabled state not read back")
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "other-tenant").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/agent-grants/"+grant.ID, nil, nil); status != 404 {
		t.Fatalf("cross-tenant status %d", status)
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	_, legacyClient := agent(t, alice, "", "read")
	if err := legacyClient.JSON(t.Context(), "GET", "/api/v1/agent-grants/"+grant.ID, "", nil, nil); err == nil {
		t.Fatal("Agent read human management resource")
	}
}
