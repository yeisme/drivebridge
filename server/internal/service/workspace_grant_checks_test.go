package service_test

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantChecksLocalOnlyAndRevisionBound(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, client := agent(t, h, "refs", "read")
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Updates(map[string]any{"restriction_profile": "personal_workspace_v1", "connection_id": h.space.ConnectionID}).Error; err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(time.Hour)
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: a.Prefix, Protocol: "http", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &expiry, IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	path := "/agent-grants/" + g.ID + "/checks"
	var view contract.GrantChecksView
	if status := h.call(t, "POST", path, contract.RevisionRequest{ExpectedRevision: 1}, &view); status != 200 || view.ConnectionState != "unknown" || len(view.Checks) != 4 {
		t.Fatal("local check result incorrect")
	}
	for _, check := range view.Checks {
		if check.Status != "observed_ok" {
			t.Fatalf("local check failed %s", check.Name)
		}
	}
	if status := h.call(t, "POST", path, contract.RevisionRequest{ExpectedRevision: 0}, nil); status != 409 {
		t.Fatalf("stale check status %d", status)
	}
	if status := bob.call(t, "POST", path, contract.RevisionRequest{ExpectedRevision: 1}, nil); status != 404 {
		t.Fatalf("foreign check status %d", status)
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1"+path, "", contract.RevisionRequest{ExpectedRevision: 1}, nil); err == nil {
		t.Fatal("Agent inspected management checks")
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", g.ID).Updates(map[string]any{"disabled": true, "revision": 2}).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "POST", path, contract.RevisionRequest{ExpectedRevision: 2}, &view); status != 200 || view.Checks[0].Status != "observed_error" || view.ConnectionState != "unknown" {
		t.Fatal("disabled check incorrectly healthy")
	}
	var stored service.WorkspaceGrant
	if err := f.Service.DB.First(&stored, "id = ?", g.ID).Error; err != nil || stored.Revision != 2 || stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(expiry) || stored.CredentialGeneration != 1 {
		t.Fatal("check mutated grant")
	}
	var count int64
	if err := f.Service.DB.Model(&service.Operation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("check created transfer operation")
	}
}
