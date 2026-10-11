package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantCreateHTTP(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	input := map[string]any{"name": "Reader", "protocol": "http", "space_id": alice.space.ID, "connection_id": alice.space.ConnectionID, "prefix": "refs", "actions": []string{"files.read"}}
	call := func(h *human, key string, body map[string]any) (int, json.RawMessage) {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1/agent-grants", bytes.NewReader(data))
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
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, envelope.Data
	}
	if status, _ := call(bob, "key", input); status != 404 {
		t.Fatalf("foreign space status %d", status)
	}
	callback := "workspace_create_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.create" {
			tx.AddError(errors.New("fixture audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failedStatus, failedBody := call(alice, "failed", input)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failedStatus != 503 || len(failedBody) != 0 {
		t.Fatal("failed creation returned credential data")
	}
	for _, model := range []any{&service.WorkspaceGrant{}, &service.WorkspaceRequest{}, &service.WorkspaceCredentialHistory{}} {
		var count int64
		if err := f.Service.DB.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("audit failure retained %T", model)
		}
	}
	var marked int64
	if err := f.Service.DB.Model(&service.Agent{}).Where("restriction_profile = ?", "personal_workspace_v1").Count(&marked).Error; err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Fatal("audit failure retained issued identity")
	}
	status, raw := call(alice, "key", input)
	if status != 201 {
		t.Fatalf("create status %d", status)
	}
	var issued contract.GrantIssuedView
	if err := json.Unmarshal(raw, &issued); err != nil {
		t.Fatal(err)
	}
	if issued.Grant.ExpiryMode != "never" || issued.Grant.ExpiresAt != nil || issued.Grant.Protocol != "http" || issued.Credential.Secret == "" {
		t.Fatal("create contract mismatch")
	}
	var agent service.Agent
	if err := f.Service.DB.First(&agent, "id = ?", issued.Grant.AgentID).Error; err != nil {
		t.Fatal(err)
	}
	if agent.Scopes != "" || agent.RestrictionProfile != "personal_workspace_v1" || agent.TokenHash != service.Hash(issued.Credential.Secret) {
		t.Fatal("created identity not restricted")
	}
	status, replay := call(alice, "key", input)
	if status != 201 || bytes.Contains(replay, []byte(issued.Credential.Secret)) {
		t.Fatal("replay failed or leaked secret")
	}
	var replayView contract.GrantReplayView
	if err := json.Unmarshal(replay, &replayView); err != nil {
		t.Fatal(err)
	}
	if replayView.Grant.GrantID != issued.Grant.GrantID || replayView.Credential.State != "not_replayable" {
		t.Fatal("replay changed identity")
	}
	var lookup contract.GrantReplayView
	if status := alice.call(t, "POST", "/agent-grants/lookup", map[string]string{"idempotency_key": "key"}, &lookup); status != 200 || lookup.Grant.GrantID != issued.Grant.GrantID || lookup.Credential.State != "not_replayable" {
		t.Fatal("creation lookup failed")
	}
	if status := bob.call(t, "POST", "/agent-grants/lookup", map[string]string{"idempotency_key": "key"}, nil); status != 404 {
		t.Fatalf("cross-owner lookup status %d", status)
	}
	if status := alice.call(t, "POST", "/agent-grants/lookup", map[string]string{"idempotency_key": "missing"}, nil); status != 404 {
		t.Fatalf("missing lookup status %d", status)
	}
	input["name"] = "Different"
	if status, _ := call(alice, "key", input); status != 409 {
		t.Fatalf("conflict status %d", status)
	}
	var view contract.GrantView
	if status := alice.call(t, "GET", "/agent-grants/"+issued.Grant.GrantID, nil, &view); status != 200 || view.Name != "Reader" {
		t.Fatal("created grant could not be read")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("owner = ?", alice.user.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("replay duplicated grant")
	}
}
