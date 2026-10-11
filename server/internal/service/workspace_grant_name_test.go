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

func TestWorkspaceGrantRenameHTTPPreservesCredentialAndExpiry(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "refs", "read")
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("restriction_profile", "personal_workspace_v1").Error; err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(time.Hour)
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &expiry, IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	var before service.Agent
	if err := f.Service.DB.First(&before, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	call := func(key, name string) (int, contract.GrantView) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"expected_revision": 1, "name": name})
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
	const callback = "rename_grant_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.rename" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, _ := call("key", "Renamed")
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if status != 503 {
		t.Fatalf("audit status %d", status)
	}
	var rolled service.Agent
	if err := f.Service.DB.First(&rolled, "id = ?", a.ID).Error; err != nil || rolled.Name != before.Name {
		t.Fatal("audit failure committed name")
	}
	for i := 0; i < 2; i++ {
		status, view := call("key", "Renamed")
		if status != 200 || view.Name != "Renamed" || view.Revision != 2 || view.ExpiresAt == nil || !view.ExpiresAt.Equal(expiry) || view.CredentialGeneration != 1 {
			t.Fatal("rename/replay changed expiry or generation")
		}
	}
	if status, _ := call("key", "Different"); status != 409 {
		t.Fatalf("changed replay %d", status)
	}
	if status, _ := call("new-key", "Stale"); status != 409 {
		t.Fatalf("stale revision %d", status)
	}
	var after service.Agent
	if err := f.Service.DB.First(&after, "id = ?", a.ID).Error; err != nil || after.TokenHash != before.TokenHash || after.Scopes != before.Scopes || !after.Expires.Equal(before.Expires) {
		t.Fatal("rename changed credential authority")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", g.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("receipt duplicated")
	}
}
