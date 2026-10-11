package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantPatchEnableDisableHTTP(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "refs", "read")
	expiry := time.Now().UTC().Add(time.Hour)
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &expiry, IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	patch := func(key, body string) (int, contract.GrantView) {
		t.Helper()
		req, err := http.NewRequest("PATCH", h.base+"/api/v1/agent-grants/"+g.ID, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope contract.DataEnvelope[contract.GrantView]
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, envelope.Data
	}
	for _, body := range []string{`{"expected_revision":1,"enabled":null}`, `{"enabled":false}`, `{"expected_revision":1}`} {
		if status, _ := patch("invalid", body); status != 400 {
			t.Fatalf("invalid input status %d", status)
		}
	}
	if status, _ := patch("invalid-action", `{"expected_revision":1,"enabled":false,"actions":["write"]}`); status != 400 {
		t.Fatalf("invalid action status %d", status)
	}
	status, disabled := patch("disable", `{"expected_revision":1,"enabled":false}`)
	if status != 200 || disabled.State != "disabled" || disabled.Revision != 2 || disabled.ExpiresAt == nil || !disabled.ExpiresAt.Equal(expiry) {
		t.Fatal("disable result mismatch")
	}
	if status, replay := patch("disable", `{"expected_revision":1,"enabled":false}`); status != 200 || replay.Revision != 2 {
		t.Fatal("disable replay failed")
	}
	if status, _ := patch("stale", `{"expected_revision":1,"enabled":true}`); status != 409 {
		t.Fatalf("stale status %d", status)
	}
	status, enabled := patch("enable", `{"expected_revision":2,"enabled":true}`)
	if status != 200 || enabled.State != "active" || enabled.Revision != 3 || !enabled.ExpiresAt.Equal(expiry) {
		t.Fatal("enable extended expiry or failed")
	}
	past := time.Now().UTC().Add(-time.Minute)
	if err := f.Service.DB.Model(&g).Updates(map[string]any{"expires_at": past, "disabled": true}).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := patch("expired-enable", `{"expected_revision":3,"enabled":true}`); status != 403 {
		t.Fatalf("expired grant enable status %d", status)
	}
	var afterExpiry service.WorkspaceGrant
	if err := f.Service.DB.First(&afterExpiry, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !afterExpiry.Disabled || afterExpiry.Revision != 3 {
		t.Fatal("expired grant mutated")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", g.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("receipt count %d", count)
	}
}
