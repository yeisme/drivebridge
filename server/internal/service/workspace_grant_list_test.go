package service_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantListFiltersAndCursor(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	now := time.Now().UTC()
	expired := now.Add(-time.Hour)
	for _, tc := range []struct {
		id, name string
		disabled bool
		expiry   *time.Time
	}{{"a", "Research", false, nil}, {"b", "Writer", true, nil}, {"c", "Old", false, &expired}} {
		a := service.Agent{ID: "agent-" + tc.id, Owner: h.user.ID, Name: tc.name, SpaceID: h.space.ID, TokenHash: service.Hash(tc.id), RestrictionProfile: "personal_workspace_v1"}
		g := service.WorkspaceGrant{ID: tc.id, Owner: h.user.ID, AgentID: a.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Protocol: "http", Actions: "files.read", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", Disabled: tc.disabled, IssuedAt: now, Revision: 1, CredentialGeneration: 1}
		if tc.expiry != nil {
			g.ExpiryMode = "duration"
			g.ExpiresAt = tc.expiry
		}
		for _, row := range []any{&a, &g} {
			if err := f.Service.DB.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	var first contract.Page[contract.GrantView]
	if status := h.call(t, "GET", "/agent-grants?limit=1", nil, &first); status != 200 || len(first.Items) != 1 || first.Items[0].GrantID != "a" || first.Items[0].State != "active" || first.NextCursor == "" {
		t.Fatal("first grant page failed")
	}
	next := "/agent-grants?limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	var second contract.Page[contract.GrantView]
	if status := h.call(t, "GET", next, nil, &second); status != 200 || len(second.Items) != 1 || second.Items[0].State != "disabled" {
		t.Fatal("second grant page failed")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatalf("foreign cursor status %d", status)
	}
	for _, tc := range []struct{ query, id string }{{"state=expired", "c"}, {"q=research", "a"}, {"state=disabled", "b"}} {
		var page contract.Page[contract.GrantView]
		if status := h.call(t, "GET", "/agent-grants?"+tc.query, nil, &page); status != 200 || len(page.Items) != 1 || page.Items[0].GrantID != tc.id {
			t.Fatalf("filter failed %s", tc.query)
		}
	}
	var empty contract.Page[contract.GrantView]
	if status := bob.call(t, "GET", "/agent-grants", nil, &empty); status != 200 || len(empty.Items) != 0 {
		t.Fatal("foreign owner list leaked grants")
	}
	for _, query := range []string{"state=unknown", "q=a&q=b", "limit=0", "cursor=bad"} {
		if status := h.call(t, "GET", "/agent-grants?"+query, nil, nil); status < 400 {
			t.Fatal("invalid query accepted")
		}
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", "a").Updates(map[string]any{"revoked": true, "revision": 2}).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatalf("changed grant cursor %d", status)
	}
	_, client := agent(t, h, "refs", "read")
	if err := client.JSON(t.Context(), "GET", "/api/v1/agent-grants", "", nil, nil); err == nil {
		t.Fatal("Agent listed grants")
	}
}
