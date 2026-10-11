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

func TestWorkspaceRotationHTTPOneTimeDelivery(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, oldClient := agent(t, alice, "refs", "read")
	deadline := time.Now().UTC().Add(time.Hour)
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: alice.user.ID, SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &deadline, IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	call := func(h *human, key, body string) (int, json.RawMessage) {
		t.Helper()
		req, err := http.NewRequest("POST", h.base+"/api/v1/agent-grants/"+g.ID+"/rotate", bytes.NewBufferString(body))
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
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	if status, _ := call(bob, "request", `{"expected_revision":1}`); status != 404 {
		t.Fatalf("cross-user status %d", status)
	}
	var beforeFailure service.Agent
	if err := f.Service.DB.First(&beforeFailure, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	callback := "workspace_rotation_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.rotate" {
			tx.AddError(errors.New("fixture audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failedStatus, failedBody := call(alice, "failed-request", `{"expected_revision":1}`)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failedStatus != 503 || len(failedBody) != 0 {
		t.Fatal("failed rotation delivered credentials")
	}
	var afterFailure service.Agent
	if err := f.Service.DB.First(&afterFailure, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailure.TokenHash != beforeFailure.TokenHash {
		t.Fatal("audit failure changed active credential")
	}
	var histories int64
	if err := f.Service.DB.Model(&service.WorkspaceCredentialHistory{}).Where("grant_id = ?", g.ID).Count(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if histories != 0 {
		t.Fatal("audit failure left credential history")
	}
	status, raw := call(alice, "request", `{"expected_revision":1}`)
	if status != 200 {
		t.Fatalf("rotation status %d", status)
	}
	var issued contract.GrantIssuedView
	if err := json.Unmarshal(raw, &issued); err != nil {
		t.Fatal(err)
	}
	if issued.Credential.State != "issued" || issued.Credential.Secret == "" || issued.Grant.CredentialGeneration != 2 || !issued.Grant.ExpiresAt.Equal(deadline) {
		t.Fatal("invalid issued response")
	}
	var stored service.Agent
	if err := f.Service.DB.First(&stored, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash != service.Hash(issued.Credential.Secret) {
		t.Fatal("issued secret does not match current hash")
	}
	if err := oldClient.JSON(t.Context(), "GET", "/api/v1/spaces", "", nil, nil); err == nil {
		t.Fatal("old credential still authenticated after rotation")
	}
	status, raw = call(alice, "request", `{"expected_revision":1}`)
	if status != 200 || bytes.Contains(raw, []byte(issued.Credential.Secret)) {
		t.Fatal("replay exposed secret or failed")
	}
	var replay struct {
		Credential map[string]any `json:"credential"`
	}
	if err := json.Unmarshal(raw, &replay); err != nil {
		t.Fatal(err)
	}
	if len(replay.Credential) != 1 || replay.Credential["state"] != "not_replayable" {
		t.Fatal("replay credential fields")
	}
	var lookup contract.RotationReplayView
	if status := alice.call(t, "POST", "/agent-grants/"+g.ID+"/rotate/lookup", map[string]string{"idempotency_key": "request"}, &lookup); status != 200 || lookup.CredentialGeneration != 2 {
		t.Fatal("rotation lookup failed")
	}
	var receipts, audits int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", g.ID).Count(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.Audit{}).Where("resource = ? AND action = ?", g.ID, "workspace.grant.rotate").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if receipts != 1 || audits != 1 {
		t.Fatal("rotation replay duplicated records")
	}
}
