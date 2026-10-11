package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGroupGetHTTPIsolation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	group := service.WorkspaceAgentGroup{ID: service.ID(), Owner: alice.user.ID, Name: "Research", NormalizedName: "research", Revision: 1}
	if err := f.Service.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	var view contract.AgentGroupView
	if status := alice.call(t, "GET", "/agent-groups/"+group.ID, nil, &view); status != 200 || view.GroupID != group.ID || view.Name != group.Name || view.Revision != 1 {
		t.Fatal("owner group read failed")
	}
	if status := bob.call(t, "GET", "/agent-groups/"+group.ID, nil, nil); status != 404 {
		t.Fatalf("foreign owner status %d", status)
	}
	_, client := agent(t, alice, "refs", "read")
	if err := client.JSON(t.Context(), "GET", "/api/v1/agent-groups/"+group.ID, "", nil, nil); err == nil {
		t.Fatal("Agent read personal group")
	}
	if err := f.Service.DB.Model(&service.WorkspaceAgentGroup{}).Where("id = ?", group.ID).Update("tenant_id", "other-tenant").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/agent-groups/"+group.ID, nil, nil); status != 404 {
		t.Fatalf("foreign tenant status %d", status)
	}
	if err := f.Service.DB.Model(&service.WorkspaceAgentGroup{}).Where("id = ?", group.ID).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", alice.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/agent-groups/"+group.ID, nil, nil); status != 403 {
		t.Fatalf("disabled owner status %d", status)
	}
}
