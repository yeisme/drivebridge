package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"net/url"
	"testing"
	"time"
)

func TestWorkspaceGrantOrganizationProjection(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	now := time.Now().UTC()
	groupID := "group-projection"
	group := service.WorkspaceAgentGroup{ID: groupID, Owner: alice.user.ID, Name: "Projection", Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := f.Service.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		agent := service.Agent{ID: "projection-agent-" + id, Owner: alice.user.ID, Name: "Agent " + id, SpaceID: alice.space.ID, TokenHash: service.Hash("projection-" + id), RestrictionProfile: "personal_workspace_v1"}
		grant := service.WorkspaceGrant{ID: "projection-" + id, Owner: alice.user.ID, AgentID: agent.ID, SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Protocol: "http", Actions: "files.read", RestrictionProfile: agent.RestrictionProfile, ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
		if err := f.Service.DB.Create(&agent).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.Service.DB.Create(&grant).Error; err != nil {
			t.Fatal(err)
		}
		if id == "c" {
			org := service.WorkspaceAgentOrganization{ID: "projection-org", Owner: alice.user.ID, AgentID: agent.ID, GroupID: &groupID, Pinned: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
			if err := f.Service.DB.Create(&org).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	get := func(query string) contract.Page[contract.GrantView] {
		t.Helper()
		var page contract.Page[contract.GrantView]
		if status := alice.call(t, "GET", "/agent-grants?"+query, nil, &page); status != 200 {
			t.Fatalf("query %s status %d", query, status)
		}
		return page
	}
	legacy := get("limit=1")
	if legacy.Items[0].Organization != nil {
		t.Fatal("default read leaked organization projection")
	}
	if legacy.Items[0].GrantID != "projection-a" {
		t.Fatal("default sort changed")
	}
	first := get("order=pinned&limit=1")
	if first.Items[0].GrantID != "projection-c" || first.NextCursor == "" || first.Total != nil {
		t.Fatal("pin not sorted before pagination")
	}
	org := first.Items[0].Organization
	if org == nil || org.AgentID != first.Items[0].AgentID || org.GroupID == nil || *org.GroupID != groupID || !org.Pinned || org.Revision != 1 {
		t.Fatal("missing atomic organization projection")
	}
	next := "order=pinned&limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	second := get(next)
	if second.Items[0].Organization == nil || second.Items[0].Organization.GroupID != nil || second.Items[0].Organization.Pinned || second.Items[0].Organization.AgentID != second.Items[0].AgentID {
		t.Fatal("virtual ungrouped projection missing")
	}
	if second.Items[0].GrantID != "projection-a" {
		t.Fatal("second page incorrect")
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"group_id=" + groupID, 1}, {"ungrouped=true", 2}, {"ungrouped=false", 1}, {"pinned=true", 1}, {"pinned=false", 2}, {"group_id=foreign", 0}} {
		if len(get(tc.query).Items) != tc.count {
			t.Fatalf("filter failed %s", tc.query)
		}
	}
	if status := bob.call(t, "GET", "/agent-grants?"+next, nil, nil); status != 410 {
		t.Fatalf("foreign cursor status %d", status)
	}
	if status := alice.call(t, "GET", "/agent-grants?"+next+"&pinned=true", nil, nil); status != 410 {
		t.Fatalf("changed query status %d", status)
	}
	for _, query := range []string{"pinned=1", "ungrouped=", "group_id=", "group_id=x&ungrouped=false", "order=unknown", "pinned=true&pinned=false"} {
		if status := alice.call(t, "GET", "/agent-grants?"+query, nil, nil); status != 400 {
			t.Fatalf("invalid query %s status %d", query, status)
		}
	}
	// Foreign organization revisions do not affect this owner's cursor.
	foreign := service.WorkspaceAgentOrganization{ID: "foreign-org", Owner: bob.user.ID, AgentID: "projection-agent-a", Pinned: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := f.Service.DB.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if get(next).Items[0].GrantID != "projection-a" {
		t.Fatal("foreign organization leaked")
	}
	if err := f.Service.DB.Model(&service.WorkspaceAgentOrganization{}).Where("id = ?", "projection-org").Updates(map[string]any{"revision": 2, "pinned": false}).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/agent-grants?"+next, nil, nil); status != 410 {
		t.Fatalf("organization change did not expire cursor: %d", status)
	}
	if len(get("limit=1&cursor="+url.QueryEscape(legacy.NextCursor)).Items) != 1 {
		t.Fatal("organization changed default cursor")
	}
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", "projection-agent-c").Update("archived", true).Error; err != nil {
		t.Fatal(err)
	}
	if len(get("order=pinned").Items) != 2 {
		t.Fatal("archived organization visible")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceAgentOrganization{}).Where("owner = ?", alice.user.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("read created virtual org rows")
	}
}
