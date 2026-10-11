package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceGrantCombinedPatchAtomicHTTP(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "refs", "read")
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Updates(map[string]any{"restriction_profile": "personal_workspace_v1", "connection_id": h.space.ConnectionID}).Error; err != nil {
		t.Fatal(err)
	}
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: "refs", Protocol: "http", Actions: "inbox.submit", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"expected_revision": 1, "name": "Combined", "prefix": "refs/narrow", "actions": []string{"labels.suggest"}, "enabled": false, "expiry_mode": "duration", "expires_in_seconds": 3600}
	call := func(key string) (int, contract.GrantView) {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("PATCH", h.base+"/api/v1/agent-grants/"+g.ID, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.GrantView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	const callback = "combined_patch_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.patch" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call("key")
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit failure %d", failed)
	}
	var unchanged service.WorkspaceGrant
	var agentBefore service.Agent
	if err := f.Service.DB.First(&unchanged, "id = ?", g.ID).Error; err != nil || unchanged.Revision != 1 || unchanged.Disabled || unchanged.ExpiresAt != nil || unchanged.Prefix != "refs" {
		t.Fatal("grant rollback incomplete")
	}
	if err := f.Service.DB.First(&agentBefore, "id = ?", a.ID).Error; err != nil || agentBefore.Prefix != "refs" || agentBefore.Name == "Combined" {
		t.Fatal("Agent rollback incomplete")
	}
	status, view := call("key")
	if status != 200 || view.Revision != 2 || view.Name != "Combined" || view.Prefix != "refs/narrow" || view.State != "disabled" || view.ExpiryMode != "duration" || view.ExpiresAt == nil || view.ExpiryChangedAt == nil || view.ExpiresAt.Sub(*view.ExpiryChangedAt) != time.Hour || len(view.Actions) != 1 || view.Actions[0] != "labels.suggest" {
		t.Fatalf("combined patch mismatch status=%d", status)
	}
	status, replay := call("key")
	if status != 200 || replay.Revision != 2 || !replay.ExpiresAt.Equal(*view.ExpiresAt) {
		t.Fatal("replay renewed expiry")
	}
	body["name"] = "Changed"
	if status, _ := call("key"); status != 409 {
		t.Fatal("changed replay accepted")
	}
	if status, _ := call("stale"); status != 409 {
		t.Fatal("stale CAS accepted")
	}
	var after service.Agent
	if err := f.Service.DB.First(&after, "id = ?", a.ID).Error; err != nil || after.TokenHash != agentBefore.TokenHash {
		t.Fatal("combined patch rotated credential")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", g.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("multiple receipts for one patch")
	}
	body = map[string]any{"expected_revision": 2, "enabled": true, "expiry_mode": "never"}
	status, permanent := call("never")
	if status != 200 || permanent.Revision != 3 || permanent.State != "active" || permanent.ExpiresAt != nil || permanent.ExpiryMode != "never" || !permanent.IssuedAt.Equal(g.IssuedAt) {
		t.Fatal("duration to never did not clear expiry or changed issuance")
	}
	status, permanentReplay := call("never")
	if status != 200 || permanentReplay.ExpiresAt != nil || permanentReplay.Revision != 3 || permanentReplay.ExpiryChangedAt == nil || !permanentReplay.ExpiryChangedAt.Equal(*permanent.ExpiryChangedAt) {
		t.Fatal("never replay changed expiry timestamp")
	}
	past := time.Now().UTC().Add(-time.Minute)
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", g.ID).Updates(map[string]any{"expiry_mode": "duration", "expires_at": past}).Error; err != nil {
		t.Fatal(err)
	}
	body = map[string]any{"expected_revision": 3, "enabled": true, "expiry_mode": "never", "name": "Revived"}
	if status, _ := call("revive"); status != 403 {
		t.Fatalf("expired combined revival status %d", status)
	}
	var expired service.WorkspaceGrant
	if err := f.Service.DB.First(&expired, "id = ?", g.ID).Error; err != nil || expired.Revision != 3 || expired.ExpiresAt == nil || !expired.ExpiresAt.Equal(past) {
		t.Fatal("expired grant mutated")
	}
	if err := f.Service.DB.First(&after, "id = ?", a.ID).Error; err != nil || after.Name == "Revived" {
		t.Fatal("rejected revival changed name")
	}
}
