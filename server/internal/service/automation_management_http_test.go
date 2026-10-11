package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestAutomationManagementHTTPClosedPreviewReceiptAndIsolation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	base := "/spaces/" + alice.space.ID
	call := func(h *human, path, key string, input any) (int, json.RawMessage) {
		t.Helper()
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err = json.NewDecoder(response.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, env.Data
	}
	var capability contract.AutomationManagement
	if status := alice.call(t, "GET", base+"/automation-management", nil, &capability); status != 200 || capability.Available {
		t.Fatal("default management capability", status)
	}
	input := contract.AutomationChangeRequest{Action: "add_label", ActionDetail: "rule:http", Prefix: "auto", BudgetItems: 10, Enabled: true}
	if status, _ := call(alice, base+"/automation-rules/changes", "create", input); status != 422 {
		t.Fatal("default write not closed", status)
	}
	f.Service.Config.AutomationManagement = true
	if status, _ := call(bob, base+"/automation-rules/changes", "create", input); status != 404 {
		t.Fatal("foreign rule accepted", status)
	}
	if status, _ := call(alice, base+"/automation-rules/changes", "", input); status != 400 {
		t.Fatal("missing key accepted", status)
	}
	if status, _ := call(alice, base+"/automation-rules/changes", "malformed", map[string]any{"action": "add_label"}); status != 400 {
		t.Fatal("missing fields accepted", status)
	}
	status, raw := call(alice, base+"/automation-rules/changes", "create", input)
	if status != 200 {
		t.Fatal("change failed", status)
	}
	var receipt contract.AutomationReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.RuleID == "" || receipt.SpaceID != alice.space.ID || receipt.ResultRevision != 1 {
		t.Fatal("receipt mismatch")
	}
	c := mustSDK(t, alice)
	orgUpload(t, c, alice.space.ID, "auto/input.txt", "fixture input")
	status, raw = call(alice, "/automation-rules/"+receipt.RuleID+"/preview", "", map[string]any{})
	if status != 200 {
		t.Fatal("preview failed", status)
	}
	var preview contract.AutomationPreview
	if err := json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || !preview.Executable {
		t.Fatal("preview wrong")
	}
	var effects int64
	if err := f.Service.DB.Model(&service.AutomationIntent{}).Count(&effects).Error; err != nil || effects != 0 {
		t.Fatal("preview wrote intents")
	}
	execute := contract.AutomationExecuteRequest{ExpectedRevision: preview.Revision, PreviewDigest: preview.PreviewDigest}
	status, raw = call(alice, "/automation-rules/"+receipt.RuleID+"/execute", "run", execute)
	if status != 200 {
		t.Fatal("execute failed", status)
	}
	var executed contract.AutomationReceipt
	if err := json.Unmarshal(raw, &executed); err != nil {
		t.Fatal(err)
	}
	status, lookup := call(alice, "/automation-rules/"+receipt.RuleID+"/execute/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "run"})
	if status != 200 || !bytes.Equal(raw, lookup) {
		t.Fatal("lookup changed receipt", status)
	}
	if status, _ := call(bob, "/automation-rules/"+receipt.RuleID+"/execute/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "run"}); status != 404 {
		t.Fatal("foreign lookup exposed receipt", status)
	}
	if status, _ := call(alice, "/automation-rules/"+receipt.RuleID+"/execute/lookup?extra=1", "", contract.IdempotencyLookupRequest{IdempotencyKey: "run"}); status != 400 {
		t.Fatal("unexpected query accepted", status)
	}
	if err := c.JSON(t.Context(), "POST", "/api/v1/automation-rules/"+receipt.RuleID+"/preview", "", map[string]any{}, nil); err == nil {
		t.Fatal("Agent used human preview")
	}
	var rules []service.AutomationRule
	if status := alice.call(t, "GET", base+"/automation-rules", nil, &rules); status != 200 || len(rules) != 1 {
		t.Fatal("legacy list changed")
	}
	if err := f.Service.DB.Model(&service.FileLabelAssignment{}).Where("label = ?", "http").Count(&effects).Error; err != nil || effects != 1 {
		t.Fatal("exact effect not persisted")
	}
}
