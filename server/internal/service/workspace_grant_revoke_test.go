package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantRevokeHTTPTransaction(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, _ := agent(t, alice, "refs", "read")
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: alice.user.ID, SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	call := func(h *human, key, body string, csrf bool) (int, contract.GrantView) {
		t.Helper()
		req, err := http.NewRequest("POST", h.base+"/api/v1/agent-grants/"+g.ID+"/revoke", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		if csrf {
			req.Header.Set("X-CSRF-Token", h.csrf)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.GrantView]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	if status, _ := call(alice, "key", `{"expected_revision":1}`, false); status != 403 {
		t.Fatalf("CSRF status %d", status)
	}
	if status, _ := call(bob, "key", `{"expected_revision":1}`, true); status != 404 {
		t.Fatalf("cross-owner status %d", status)
	}
	if status, _ := call(alice, "", `{"expected_revision":1}`, true); status != 400 {
		t.Fatalf("missing key status %d", status)
	}
	if status, _ := call(alice, "key", `{}`, true); status != 400 {
		t.Fatalf("missing revision status %d", status)
	}
	callbackName := "workspace_fixture_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.revoke" {
			tx.AddError(errors.New("fixture audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, _ := call(alice, "audit-failure", `{"expected_revision":1}`, true)
	if err := f.Service.DB.Callback().Create().Remove(callbackName); err != nil {
		t.Fatal(err)
	}
	if status != 503 {
		t.Fatalf("audit failure status %d", status)
	}
	var afterFailure service.WorkspaceGrant
	if err := f.Service.DB.First(&afterFailure, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailure.Revision != 1 || afterFailure.Revoked {
		t.Fatal("audit failure committed mutation")
	}
	status, first := call(alice, "key", `{"expected_revision":1}`, true)
	if status != 200 || first.State != "revoked" || first.Revision != 2 {
		t.Fatalf("revoke status %d state %s revision %d", status, first.State, first.Revision)
	}
	status, replayed := call(alice, "key", `{"expected_revision":1}`, true)
	if status != 200 || replayed.Revision != 2 {
		t.Fatal("same request did not replay")
	}
	if status, _ := call(alice, "key", `{"expected_revision":2}`, true); status != 409 {
		t.Fatalf("changed digest status %d", status)
	}
	var receiptCount, auditCount int64
	f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", g.ID).Count(&receiptCount)
	f.Service.DB.Model(&service.Audit{}).Where("resource = ? AND action = ?", g.ID, "workspace.grant.revoke").Count(&auditCount)
	if receiptCount != 1 || auditCount != 1 {
		t.Fatalf("receipt=%d audit=%d", receiptCount, auditCount)
	}
	var stored service.WorkspaceGrant
	if err := f.Service.DB.First(&stored, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.Revoked || stored.Revision != 2 || stored.ExpiresAt != nil {
		t.Fatal("durable result mismatch")
	}
}
