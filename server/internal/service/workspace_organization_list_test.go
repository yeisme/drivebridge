package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceOrganizationListVirtualRowsAndCursor(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, _ := agent(t, h, "refs", "read")
	b, _ := agent(t, h, "refs", "read")
	_, _ = agent(t, bob, "refs", "read")
	groupID := "fixture-group"
	org := service.WorkspaceAgentOrganization{ID: service.ID(), Owner: h.user.ID, AgentID: b.ID, Pinned: true, GroupID: &groupID, Revision: 1}
	if err := f.Service.DB.Create(&org).Error; err != nil {
		t.Fatal(err)
	}
	var first contract.Page[contract.AgentOrganizationView]
	if status := h.call(t, "GET", "/agent-organizations?limit=1", nil, &first); status != 200 || len(first.Items) != 1 || first.Items[0].AgentID != b.ID || !first.Items[0].Pinned || first.NextCursor == "" {
		t.Fatal("pinned ordering failed")
	}
	next := "/agent-organizations?limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	var second contract.Page[contract.AgentOrganizationView]
	if status := h.call(t, "GET", next, nil, &second); status != 200 || len(second.Items) != 1 || second.Items[0].AgentID != a.ID || second.Items[0].Revision != 0 || second.Items[0].GroupID != nil || second.NextCursor != "" {
		t.Fatal("virtual row pagination failed")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatalf("foreign cursor %d", status)
	}
	for _, filter := range []string{"pinned=false", "ungrouped=true"} {
		var page contract.Page[contract.AgentOrganizationView]
		if status := h.call(t, "GET", "/agent-organizations?"+filter, nil, &page); status != 200 || len(page.Items) != 1 || page.Items[0].AgentID != a.ID {
			t.Fatal("virtual row filter failed")
		}
	}
	var grouped contract.Page[contract.AgentOrganizationView]
	if status := h.call(t, "GET", "/agent-organizations?group_id="+groupID, nil, &grouped); status != 200 || len(grouped.Items) != 1 || grouped.Items[0].AgentID != b.ID {
		t.Fatal("group filter failed")
	}
	for _, filter := range []string{"pinned=maybe", "ungrouped=true&group_id=x", "space_id=x&space_id=y"} {
		if status := h.call(t, "GET", "/agent-organizations?"+filter, nil, nil); status < 400 {
			t.Fatal("invalid filter accepted")
		}
	}
	if err := f.Service.DB.Model(&service.WorkspaceAgentOrganization{}).Where("id = ?", org.ID).Updates(map[string]any{"pinned": false, "revision": 2}).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatalf("changed ordering cursor %d", status)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceAgentOrganization{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("GET persisted virtual rows")
	}
}
