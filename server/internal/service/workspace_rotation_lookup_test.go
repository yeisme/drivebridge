package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceRotationLookupHTTPMetadataOnly(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, _ := agent(t, alice, "refs", "read")
	grant := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: alice.user.ID, SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 4, CredentialGeneration: 3}
	if err := f.Service.DB.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	receipt := service.WorkspaceRequest{ID: service.ID(), Owner: alice.user.ID, ActorID: alice.user.ID, Action: "grant.rotate", RequestKey: "original-key", RequestDigest: "fixture-digest", ResourceID: grant.ID, ResultRevision: 2, CredentialGeneration: 2}
	if err := f.Service.DB.Create(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	path := "/agent-grants/" + grant.ID + "/rotate/lookup"
	var raw json.RawMessage
	if status := alice.call(t, "POST", path, map[string]string{"idempotency_key": "original-key"}, &raw); status != 200 {
		t.Fatalf("lookup status %d", status)
	}
	var view contract.RotationReplayView
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if view.CredentialGeneration != 2 || view.GrantID != grant.ID || view.Credential.State != "not_replayable" {
		t.Fatal("lookup used current generation instead of original receipt")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	var credential map[string]json.RawMessage
	if err := json.Unmarshal(fields["credential"], &credential); err != nil {
		t.Fatal(err)
	}
	if len(credential) != 1 || credential["state"] == nil {
		t.Fatal("lookup returned extra credential fields")
	}
	if status := bob.call(t, "POST", path, map[string]string{"idempotency_key": "original-key"}, nil); status != 404 {
		t.Fatalf("cross-owner status %d", status)
	}
	if status := alice.call(t, "POST", path, map[string]string{"idempotency_key": "missing-key"}, nil); status != 404 {
		t.Fatalf("missing key status %d", status)
	}
	_, legacyClient := agent(t, alice, "", "read")
	if err := legacyClient.JSON(t.Context(), "POST", "/api/v1"+path, "", map[string]string{"idempotency_key": "original-key"}, nil); err == nil {
		t.Fatal("Agent accessed human rotation receipt")
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "other-tenant").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "POST", path, map[string]string{"idempotency_key": "original-key"}, nil); status != 404 {
		t.Fatalf("cross-tenant lookup status %d", status)
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	var stored service.WorkspaceGrant
	if err := f.Service.DB.First(&stored, "id = ?", grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 4 || stored.CredentialGeneration != 3 {
		t.Fatal("lookup mutated grant")
	}
}
