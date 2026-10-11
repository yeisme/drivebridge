package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceOrganizationHTTPReplayAndRollback(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, client := agent(t, h, "refs", "read")
	group := service.WorkspaceAgentGroup{ID: service.ID(), Owner: h.user.ID, Name: "Research", NormalizedName: "research", Revision: 1}
	if err := f.Service.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/agent-organizations/" + a.ID
	call := func(user *human, key string, body map[string]any) (int, contract.AgentOrganizationView) {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("PATCH", user.base+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", user.base)
		req.Header.Set("X-CSRF-Token", user.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := user.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.AgentOrganizationView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	var initial contract.AgentOrganizationView
	if status := h.call(t, "GET", "/agent-organizations/"+a.ID, nil, &initial); status != 200 || initial.Revision != 0 || initial.GroupID != nil || initial.Pinned {
		t.Fatal("virtual organization defaults incorrect")
	}
	if status := bob.call(t, "GET", "/agent-organizations/"+a.ID, nil, nil); status != 404 {
		t.Fatal("foreign organization readable")
	}
	input := map[string]any{"expected_revision": 0, "group_id": group.ID, "pinned": true}
	if status, _ := call(bob, "key", input); status != 404 {
		t.Fatalf("foreign status %d", status)
	}
	if err := client.JSON(t.Context(), "PATCH", path, "key", input, nil); err == nil {
		t.Fatal("Agent organized itself")
	}
	const callback = "organization_patch_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.organization.patch" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call(h, "key", input)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit status %d", failed)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceAgentOrganization{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed audit retained organization")
	}
	for i := 0; i < 2; i++ {
		status, view := call(h, "key", input)
		if status != 200 || !view.Pinned || view.GroupID == nil || *view.GroupID != group.ID || view.Revision != 1 {
			t.Fatal("patch or replay failed")
		}
	}
	if status, _ := call(h, "key", map[string]any{"expected_revision": 0, "pinned": false}); status != 409 {
		t.Fatalf("changed replay status %d", status)
	}
	status, unbound := call(h, "unbind", map[string]any{"expected_revision": 1, "group_id": nil})
	if status != 200 || unbound.GroupID != nil || !unbound.Pinned || unbound.Revision != 2 {
		t.Fatal("null unbind lost pin")
	}
	status, replay := call(h, "key", input)
	if status != 200 || replay.GroupID != nil || replay.Revision != 2 {
		t.Fatal("old replay rewrote newer organization")
	}
	var refreshed contract.AgentOrganizationView
	if status := h.call(t, "GET", "/agent-organizations/"+a.ID, nil, &refreshed); status != 200 || refreshed.Revision != 2 || refreshed.GroupID != nil || !refreshed.Pinned {
		t.Fatal("refresh lost persisted organization")
	}
	if status, _ := call(h, "stale", input); status != 409 {
		t.Fatalf("stale CAS status %d", status)
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := call(h, "key", input); status != 403 {
		t.Fatalf("disabled replay status %d", status)
	}
}
