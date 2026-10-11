package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGroupListScopedPagination(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	for _, group := range []service.WorkspaceAgentGroup{
		{ID: "a", Owner: alice.user.ID, Name: "First", NormalizedName: "first", Revision: 1},
		{ID: "b", Owner: alice.user.ID, Name: "Second", NormalizedName: "second", Revision: 1},
		{ID: "c", Owner: alice.user.ID, TenantID: "foreign", Name: "Hidden", NormalizedName: "hidden", Revision: 1},
		{ID: "d", Owner: bob.user.ID, Name: "Other", NormalizedName: "other", Revision: 1},
	} {
		if err := f.Service.DB.Create(&group).Error; err != nil {
			t.Fatal(err)
		}
	}
	var first contract.Page[contract.AgentGroupView]
	if status := alice.call(t, "GET", "/agent-groups?limit=1", nil, &first); status != 200 || len(first.Items) != 1 || first.Items[0].GroupID != "a" || first.NextCursor == "" || first.Total != nil || first.TotalKind != "unknown" {
		t.Fatal("invalid first page")
	}
	next := "/agent-groups?limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	var second contract.Page[contract.AgentGroupView]
	if status := alice.call(t, "GET", next, nil, &second); status != 200 || len(second.Items) != 1 || second.Items[0].GroupID != "b" || second.NextCursor != "" {
		t.Fatal("invalid second page")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatalf("foreign cursor status %d", status)
	}
	if status := alice.call(t, "GET", "/agent-groups?limit=2&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatalf("changed page limit status %d", status)
	}
	for _, query := range []string{"limit=0", "limit=201", "limit=1&limit=2", "cursor=bad"} {
		if status := alice.call(t, "GET", "/agent-groups?"+query, nil, nil); status < 400 {
			t.Fatalf("invalid query accepted: %s", query)
		}
	}
	// Foreign changes must not invalidate this owner's generation.
	if err := f.Service.DB.Model(&service.WorkspaceAgentGroup{}).Where("id = ?", "d").Update("revision", 2).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatalf("foreign mutation invalidated page: %d", status)
	}
	for _, change := range []string{"rename", "create", "delete"} {
		var baseline contract.Page[contract.AgentGroupView]
		if status := alice.call(t, "GET", "/agent-groups?limit=1", nil, &baseline); status != 200 || baseline.NextCursor == "" {
			t.Fatal("missing baseline cursor")
		}
		switch change {
		case "rename":
			if err := f.Service.DB.Model(&service.WorkspaceAgentGroup{}).Where("id = ?", "b").Updates(map[string]any{"name": "Renamed", "revision": 2}).Error; err != nil {
				t.Fatal(err)
			}
		case "create":
			if err := f.Service.DB.Create(&service.WorkspaceAgentGroup{ID: "e", Owner: alice.user.ID, Name: "New", NormalizedName: "new", Revision: 1}).Error; err != nil {
				t.Fatal(err)
			}
		case "delete":
			if err := f.Service.DB.Where("id = ?", "e").Delete(&service.WorkspaceAgentGroup{}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if status := alice.call(t, "GET", "/agent-groups?limit=1&cursor="+url.QueryEscape(baseline.NextCursor), nil, nil); status != 410 {
			t.Fatalf("%s retained stale cursor: %d", change, status)
		}
	}
	_, client := agent(t, alice, "refs", "read")
	if err := client.JSON(t.Context(), "GET", "/api/v1/agent-groups", "", nil, nil); err == nil {
		t.Fatal("Agent listed personal groups")
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", alice.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", next, nil, nil); status != 403 {
		t.Fatalf("disabled pagination status %d", status)
	}
}
