package service_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceInvocationListScopeTimeAndCursor(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	// Synthetic boundary rows supplement the separate real-transport test.
	a := service.Agent{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, RestrictionProfile: "personal_workspace_v1", TokenHash: service.ID()}
	grant := service.WorkspaceGrant{ID: service.ID(), Owner: h.user.ID, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", Protocol: "http", Revision: 1, CredentialGeneration: 1, IssuedAt: time.Now().UTC()}
	for _, row := range []any{&a, &grant} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 10, 5, 12, 0, 0, 0, time.FixedZone("offset", 8*3600))
	rows := []service.WorkspaceInvocation{}
	for i := 0; i < 3; i++ {
		rows = append(rows, service.WorkspaceInvocation{ID: fmt.Sprintf("call-%d", i), Owner: h.user.ID, SpaceID: h.space.ID, AgentID: a.ID, GrantID: grant.ID, Protocol: "http", Action: "files.list", RequestID: service.ID(), Result: "unknown", StartedAt: base.Add(time.Duration(i) * time.Second)})
	}
	if err := f.Service.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	get := func(query string) contract.InvocationPage {
		t.Helper()
		var page contract.InvocationPage
		if status := h.call(t, "GET", "/agent-invocations?"+query, nil, &page); status != 200 {
			t.Fatalf("list status %d", status)
		}
		return page
	}
	first := get("limit=1")
	if first.Total != nil || first.Coverage != "best_effort" || first.NextCursor == "" || first.Items[0].FinishedAt != nil {
		t.Fatal("invented coverage or finish")
	}
	next := "/agent-invocations?limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	if status := h.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatal("stable cursor failed")
	}
	if status := h.call(t, "GET", next+"&result=unknown", nil, nil); status != 410 {
		t.Fatal("changed query reused a still-valid cursor")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("cross owner cursor admitted")
	}
	var empty contract.InvocationPage
	if status := bob.call(t, "GET", "/agent-invocations", nil, &empty); status != 200 || len(empty.Items) != 0 {
		t.Fatal("foreign calls visible")
	}
	window := url.Values{"started_after": {base.UTC().Format(time.RFC3339)}, "started_before": {base.Add(2 * time.Second).Format(time.RFC3339)}}
	filtered := get(window.Encode())
	if len(filtered.Items) != 1 || filtered.Items[0].InvocationID != "call-1" {
		t.Fatal("timestamp offsets or exclusive boundaries mishandled")
	}
	half := url.Values{"started_at_or_after": {base.UTC().Format(time.RFC3339Nano)}, "started_before": {base.Add(2 * time.Second).Format(time.RFC3339Nano)}, "limit": {"1"}}
	halfFirst := get(half.Encode())
	if len(halfFirst.Items) != 1 || halfFirst.Items[0].InvocationID != "call-0" || halfFirst.NextCursor == "" {
		t.Fatal("half-open lower endpoint was excluded")
	}
	half.Set("cursor", halfFirst.NextCursor)
	// Equivalent explicit zones represent the same query identity.
	half.Set("started_at_or_after", base.Format(time.RFC3339Nano))
	halfNext := get(half.Encode())
	if len(halfNext.Items) != 1 || halfNext.Items[0].InvocationID != "call-1" || halfNext.NextCursor != "" {
		t.Fatal("half-open upper endpoint included or cursor lost")
	}
	window.Set("limit", "1")
	window.Set("cursor", halfFirst.NextCursor)
	if status := h.call(t, "GET", "/agent-invocations?"+window.Encode(), nil, nil); status != 410 {
		t.Fatal("inclusive cursor accepted for exclusive query")
	}
	if status := h.call(t, "GET", next+"&started_at_or_after="+url.QueryEscape(base.Format(time.RFC3339Nano)), nil, nil); status != 410 {
		t.Fatal("legacy cursor accepted for half-open query")
	}
	// Exact nanoseconds: no subtract-one-millisecond approximation.
	precise := url.Values{"started_at_or_after": {base.Add(time.Second).UTC().Format(time.RFC3339Nano)}, "started_before": {base.Add(time.Second + time.Nanosecond).Format(time.RFC3339Nano)}}
	if got := get(precise.Encode()); len(got.Items) != 1 || got.Items[0].InvocationID != "call-1" {
		t.Fatal("nanosecond half-open range changed")
	}
	precise.Set("started_at_or_after", base.Add(time.Second+time.Nanosecond).Format(time.RFC3339Nano))
	precise.Set("started_before", base.Add(2*time.Second).Format(time.RFC3339Nano))
	if len(get(precise.Encode()).Items) != 0 {
		t.Fatal("rounded inclusive boundary")
	}
	for _, query := range []string{
		"started_at_or_after=bad", "started_at_or_after=", "started_at_or_after=2026-10-05T00:00:00Z&started_after=2026-10-05T00:00:00Z",
		"started_at_or_after=2026-10-05T00:00:00Z&started_before=2026-10-05T00:00:00Z",
		"started_at_or_after=2026-10-06T00:00:00Z&started_before=2026-10-05T00:00:00Z",
		"started_at_or_after=2026-10-05T00:00:00Z&started_at_or_after=2026-10-05T00:00:00Z",
	} {
		if status := h.call(t, "GET", "/agent-invocations?"+query, nil, nil); status != 400 {
			t.Fatalf("invalid half-open query %s: %d", query, status)
		}
	}
	if len(get("started_at_or_after=0001-01-01T00:00:00Z").Items) != 3 {
		t.Fatal("zero-time inclusive bound failed")
	}
	if len(get("started_before=0001-01-01T00:00:00Z").Items) != 0 {
		t.Fatal("explicit zero-time bound treated as absent")
	}
	if len(get("space_id=other").Items) != 0 || len(get("agent_id=other").Items) != 0 || len(get("result=succeeded").Items) != 0 {
		t.Fatal("filter invented observations")
	}
	for _, query := range []string{"started_after=bad", "started_before=", "started_after=2026-10-05T00:00:00Z&started_before=2026-10-04T00:00:00Z", "result=bad", "limit=201", "limit=1&limit=2", "unknown=1", "started_after=%ZZ", "agent_id=other;result=unknown"} {
		if status := h.call(t, "GET", "/agent-invocations?"+query, nil, nil); status != 400 {
			t.Fatalf("invalid query %s: %d", query, status)
		}
	}
	finish := base.Add(time.Minute)
	if err := f.Service.DB.Model(&service.WorkspaceInvocation{}).Where("id = ?", rows[0].ID).Update("finished_at", finish).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("finished record failed to invalidate cursor")
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 3 {
		t.Fatal("grant revocation erased owner history")
	}
	if err := f.Service.DB.Model(&service.WorkspaceInvocation{}).Where("owner = ?", h.user.ID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 0 {
		t.Fatal("foreign tenant records visible")
	}
	if err := f.Service.DB.Model(&service.WorkspaceInvocation{}).Where("owner = ?", h.user.ID).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/agent-invocations", nil, nil); status != 403 {
		t.Fatal("disabled owner read history")
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", grant.ID).Update("agent_id", "wrong").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/agent-invocations", nil, nil); status != 503 {
		t.Fatal("binding drift became empty")
	}
	if err := f.Service.DB.Migrator().DropTable(&service.WorkspaceInvocation{}); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/agent-invocations", nil, nil); status != 500 {
		t.Fatal("DB failure became empty")
	}
}

func TestWorkspaceInvocationListBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	rows := make([]service.WorkspaceInvocation, 1001)
	for i := range rows {
		rows[i] = service.WorkspaceInvocation{ID: service.ID(), Owner: h.user.ID, AgentID: "agent", SpaceID: h.space.ID, GrantID: "grant", RequestID: service.ID(), Protocol: "http", Action: "files.list", Result: "unknown", StartedAt: time.Now().UTC()}
	}
	if err := f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"limit=1", "limit=1&order=started_desc", "limit=1&order=started_desc&started_at_or_after=9999-01-01T00:00:00Z"} {
		if status := h.call(t, "GET", "/agent-invocations?"+query, nil, nil); status != 429 {
			t.Fatalf("budget %s: %d", query, status)
		}
	}
}
