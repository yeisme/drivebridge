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

func TestWorkspaceGrantChangeLookupOriginalReceipt(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	a, _ := agent(t, alice, "refs", "read")
	_, client := agent(t, alice, "", "read") // Remains valid after the target grant is revoked.
	grant := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: alice.user.ID, SpaceID: alice.space.ID, ConnectionID: alice.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	mutate := func(method, suffix, key, body string) {
		t.Helper()
		req, err := http.NewRequest(method, alice.base+"/api/v1/agent-grants/"+grant.ID+suffix, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", alice.base)
		req.Header.Set("X-CSRF-Token", alice.csrf)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		res, err := alice.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("mutation status %d", res.StatusCode)
		}
	}
	mutate("PATCH", "", "disable-key", `{"expected_revision":1,"enabled":false}`)
	mutate("PATCH", "", "enable-key", `{"expected_revision":2,"enabled":true}`)
	mutate("PATCH", "", "expiry-key", `{"expected_revision":3,"expiry_mode":"duration","expires_in_seconds":3600}`)
	mutate("POST", "/revoke", "revoke-key", `{"expected_revision":4}`)
	path := "/agent-grants/" + grant.ID + "/changes/lookup"
	for i, action := range []string{"disable", "enable", "expiry", "revoke"} {
		var raw json.RawMessage
		if status := alice.call(t, "POST", path, map[string]string{"action": action, "idempotency_key": action + "-key"}, &raw); status != 200 {
			t.Fatalf("lookup status %d", status)
		}
		var view contract.GrantChangeReceiptView
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		if view.GrantID != grant.ID || view.Action != action || view.ResultRevision != uint64(i+2) || view.CommittedAt.IsZero() {
			t.Fatal("did not return original committed revision")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 4 {
			t.Fatal("extra receipt fields")
		}
	}
	input := map[string]string{"action": "disable", "idempotency_key": "disable-key"}
	if status := bob.call(t, "POST", path, input, nil); status != 404 {
		t.Fatalf("cross-owner status %d", status)
	}
	if status := alice.call(t, "POST", path, map[string]string{"action": "enable", "idempotency_key": "disable-key"}, nil); status != 404 {
		t.Fatalf("cross-action status %d", status)
	}
	if status := alice.call(t, "POST", path, map[string]string{"action": "disable", "idempotency_key": "missing"}, nil); status != 404 {
		t.Fatalf("missing status %d", status)
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/spaces", "", nil, nil); err != nil {
		t.Fatal("fixture Agent must be authenticated before rejection check")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1"+path, "", input, nil); err == nil {
		t.Fatal("agent accessed lifecycle receipt")
	}
	for _, bad := range []map[string]any{{"action": "rotate", "idempotency_key": "disable-key"}, {"action": "disable", "idempotency_key": " "}, {"action": "disable"}, {"action": "disable", "idempotency_key": "disable-key", "owner": alice.user.ID}} {
		if status := alice.call(t, "POST", path, bad, nil); status != 400 {
			t.Fatalf("invalid input status %d", status)
		}
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "other").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "POST", path, input, nil); status != 404 {
		t.Fatalf("cross-tenant status %d", status)
	}
	if err := f.Service.DB.Model(&grant).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	other := grant
	other.ID = service.ID()
	other.AgentID = service.ID()
	if err := f.Service.DB.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "POST", "/agent-grants/"+other.ID+"/changes/lookup", input, nil); status != 404 {
		t.Fatalf("cross-resource status %d", status)
	}
	callback := "receipt_lookup_failure"
	if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			tx.AddError(errors.New("private fixture database failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status := alice.call(t, "POST", path, input, nil)
	if err := f.Service.DB.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if status != 500 {
		t.Fatalf("DB failure masqueraded as missing receipt: %d", status)
	}
	var stored service.WorkspaceGrant
	if err := f.Service.DB.First(&stored, "id = ?", grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 5 || stored.CredentialGeneration != 1 {
		t.Fatal("lookup mutated grant")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", grant.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatal("lookup mutated receipts")
	}
}
