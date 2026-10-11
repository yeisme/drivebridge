package service_test

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceInvocationStartedOrder(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a := service.Agent{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, RestrictionProfile: "personal_workspace_v1", TokenHash: service.ID()}
	g := service.WorkspaceGrant{ID: service.ID(), Owner: h.user.ID, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", Protocol: "http", Revision: 1, CredentialGeneration: 1, IssuedAt: time.Now().UTC()}
	for _, row := range []any{&a, &g} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	times := []time.Time{base, base.Add(time.Nanosecond), base.In(time.FixedZone("east", 8*3600)), base.Add(-time.Hour).In(time.FixedZone("west", -7*3600))}
	ids := []string{"a-old", "z-new", "b-tie", "c-oldest"}
	for i, id := range ids {
		row := service.WorkspaceInvocation{ID: id, Owner: h.user.ID, SpaceID: h.space.ID, AgentID: a.ID, GrantID: g.ID, Protocol: "http", Action: "files.list", RequestID: service.ID(), Result: "unknown", StartedAt: times[i]}
		if err := f.Service.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	get := func(q url.Values) contract.InvocationPage {
		t.Helper()
		var p contract.InvocationPage
		if status := h.call(t, "GET", "/agent-invocations?"+q.Encode(), nil, &p); status != 200 {
			t.Fatalf("list status %d", status)
		}
		return p
	}
	for _, half := range []bool{false, true} {
		q := url.Values{"order": {"started_desc"}, "limit": {"1"}}
		if half {
			q.Set("started_at_or_after", base.Add(-2*time.Hour).Format(time.RFC3339Nano))
		}
		first := get(q)
		got := []string{first.Items[0].InvocationID}
		cursor := first.NextCursor
		for cursor != "" {
			if len(got) > len(ids) {
				t.Fatal("pagination repeated")
			}
			q.Set("cursor", cursor)
			p := get(q)
			if len(p.Items) != 1 {
				t.Fatal("missing page")
			}
			got = append(got, p.Items[0].InvocationID)
			cursor = p.NextCursor
		}
		if !reflect.DeepEqual(got, []string{"z-new", "a-old", "b-tie", "c-oldest"}) {
			t.Fatalf("wrong instant/tie order: %v", got)
		}
		q.Set("cursor", first.NextCursor)
		q.Del("order")
		if status := h.call(t, "GET", "/agent-invocations?"+q.Encode(), nil, nil); status != 410 {
			t.Fatalf("sorted cursor reused by legacy: %d", status)
		}
	}
	legacy := get(url.Values{"limit": {"1"}})
	if legacy.Items[0].InvocationID != "a-old" {
		t.Fatal("legacy order changed")
	}
	if status := h.call(t, "GET", "/agent-invocations?order=started_desc&limit=1&cursor="+url.QueryEscape(legacy.NextCursor), nil, nil); status != 410 {
		t.Fatal("legacy cursor accepted by sorted query")
	}
	for _, query := range []string{"order=", "order=id", "order=pinned", "order=started_desc&order=started_desc"} {
		if status := h.call(t, "GET", "/agent-invocations?"+query, nil, nil); status != 400 {
			t.Fatalf("bad order %s: %d", query, status)
		}
	}
	first := get(url.Values{"order": {"started_desc"}, "limit": {"1"}})
	if err := f.Service.DB.Model(&service.WorkspaceInvocation{}).Where("id = ?", "c-oldest").Update("started_at", base.Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/agent-invocations?order=started_desc&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("changed order did not invalidate cursor")
	}
}
