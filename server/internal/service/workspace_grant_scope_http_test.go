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

func TestWorkspaceGrantScopeHTTPAtomicReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, credentialClient := agent(t, h, "refs", "read")
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Updates(map[string]any{"restriction_profile": "personal_workspace_v1", "connection_id": h.space.ConnectionID}).Error; err != nil {
		t.Fatal(err)
	}
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: a.Prefix, Protocol: "http", Actions: "labels.suggest,inbox.submit", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	submission := contract.CreateSubmissionRequest{Name: "before.txt", MIMEType: "text/plain", SizeBytes: 0, SHA256: service.Hash("")}
	if err := credentialClient.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "before-scope", submission, nil); err != nil {
		t.Fatalf("initial submit authority unavailable: %v", err)
	}
	call := func(key, prefix string, revision uint64) (int, contract.GrantView) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"expected_revision": revision, "prefix": prefix, "actions": []string{"labels.suggest"}})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("PATCH", h.base+"/api/v1/agent-grants/"+g.ID, bytes.NewReader(body))
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
		var env contract.DataEnvelope[contract.GrantView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	const callback = "scope_patch_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.scope" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call("key", "refs/narrow", 1)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit failure %d", failed)
	}
	var unchanged service.Agent
	if err := f.Service.DB.First(&unchanged, "id = ?", a.ID).Error; err != nil || unchanged.Prefix != "refs" {
		t.Fatal("failed audit changed Agent")
	}
	for i := 0; i < 2; i++ {
		status, view := call("key", "refs/narrow", 1)
		if status != 200 || view.Prefix != "refs/narrow" || view.Revision != 2 || view.ExpiresAt != nil || view.CredentialGeneration != 1 || len(view.Actions) != 1 || view.Actions[0] != "labels.suggest" {
			t.Fatal("scope patch/replay failed")
		}
	}
	if err := credentialClient.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "after-scope", submission, nil); err == nil {
		t.Fatal("old credential retained removed submit action")
	}
	var narrowed service.Agent
	if err := f.Service.DB.First(&narrowed, "id = ?", a.ID).Error; err != nil || narrowed.TokenHash != unchanged.TokenHash {
		t.Fatal("scope patch rotated credential")
	}
	if status, _ := call("key", "refs/other", 1); status != 409 {
		t.Fatalf("changed request %d", status)
	}
	if status, _ := call("next", "refs/narrow/deeper", 2); status != 200 {
		t.Fatalf("second mutation %d", status)
	}
	if status, view := call("key", "refs/narrow", 1); status != 200 || view.Prefix != "refs/narrow/deeper" || view.Revision != 3 {
		t.Fatal("old replay restored wider scope")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("action = ? AND resource_id = ?", "grant.scope", g.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("unexpected receipt count")
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := call("key", "refs/narrow", 1); status != 403 {
		t.Fatalf("disabled replay %d", status)
	}
}
