package service_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceAutomationIntentProjection(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	orgUpload(t, c, h.space.ID, "auto/a.txt", "a")
	orgUpload(t, c, h.space.ID, "auto/b.txt", "b")
	rules, err := f.Service.AutomationRules(p, h.space.ID, &service.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:verified", Prefix: "auto", Enabled: true})
	if err != nil || len(rules) != 1 {
		t.Fatal("rule creation failed", err)
	}
	intents, err := f.Service.RunAutomationRule(p, rules[0].ID)
	if err != nil || len(intents) != 2 {
		t.Fatal("real owner intent creation failed", err)
	}
	get := func(query string) contract.Page[contract.TaskProjection] {
		t.Helper()
		var page contract.Page[contract.TaskProjection]
		if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent&"+query, nil, &page); status != 200 {
			t.Fatalf("intent projection %d", status)
		}
		return page
	}
	before := rules[0]
	if err = f.Service.DB.First(&before, "id = ?", before.ID).Error; err != nil {
		t.Fatal(err)
	}
	var labelsBefore int64
	if err = f.Service.DB.Model(&service.FileLabelAssignment{}).Count(&labelsBefore).Error; err != nil {
		t.Fatal(err)
	}
	first := get("limit=1")
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("missing bounded intents")
	}
	for _, view := range get("").Items {
		if view.OwnerKind != "automation_intent" || view.OwnerStatus != "completed" || view.UIState != "unknown" || view.ActorKind != "unknown" || view.AgentID != nil || view.Progress.Total != nil || len(view.AllowedActions) != 0 {
			t.Fatal("invented credential identity/effect proof")
		}
	}
	next := "/workspace/tasks?kind=automation_intent&limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	if status := h.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatal("stable cursor rejected")
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("foreign cursor admitted")
	}
	var hidden contract.Page[contract.TaskProjection]
	if status := bob.call(t, "GET", "/workspace/tasks?kind=automation_intent", nil, &hidden); status != 200 || len(hidden.Items) != 0 {
		t.Fatal("foreign intents visible")
	}
	if len(get("agent_id=unproven").Items) != 0 {
		t.Fatal("user actor promoted to Agent")
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err = agentClient.JSON(t.Context(), "GET", "/api/v1/workspace/tasks?kind=automation_intent", "", nil, nil); err == nil {
		t.Fatal("Agent accessed human projection")
	}
	var after service.AutomationRule
	if err = f.Service.DB.First(&after, "id = ?", before.ID).Error; err != nil || after.Checkpoint != before.Checkpoint || after.Revision != before.Revision {
		t.Fatal("read ran or edited rule")
	}
	var labelsAfter int64
	if err = f.Service.DB.Model(&service.FileLabelAssignment{}).Count(&labelsAfter).Error; err != nil || labelsAfter != labelsBefore {
		t.Fatal("read emitted labels")
	}
	if len(get("space_id=other").Items) != 0 {
		t.Fatal("space query widened")
	}
	if err = f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 0 {
		t.Fatal("foreign tenant intent visible")
	}
	if err = f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("tenant_id", "").Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.AutomationRule{}).Where("id = ?", before.ID).Update("prefix", "mismatched").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent", nil, nil); status != 503 {
		t.Fatal("same revision rule scope drift admitted")
	}
	if err = f.Service.DB.Model(&service.AutomationRule{}).Where("id = ?", before.ID).Update("prefix", before.Prefix).Error; err != nil {
		t.Fatal(err)
	}
	// Later rule edits are not the historical intent's payload or outcome.
	if err = f.Service.DB.Model(&service.AutomationRule{}).Where("id = ?", before.ID).Updates(map[string]any{"revision": before.Revision + 1, "prefix": "changed", "paused": true}).Error; err != nil {
		t.Fatal(err)
	}
	if len(get("").Items) != 2 {
		t.Fatal("later rule revision rewrote historical intents")
	}
	if err = f.Service.DB.Model(&service.AutomationIntent{}).Where("id = ?", intents[0].ID).Updates(map[string]any{"status": "paused", "updated_at": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("changed intent cursor admitted")
	}
	if len(get("state=recoverable").Items) != 1 {
		t.Fatal("paused state not projected")
	}
	if err = f.Service.DB.Model(&service.OutboxEvent{}).Where("event_id = ?", intents[0].EventID).Update("resource_id", "wrong").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent", nil, nil); status != 503 {
		t.Fatal("event binding drift admitted")
	}
}

func TestWorkspaceAutomationIntentBudgetAndMissingDependency(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	rows := make([]service.AutomationIntent, 1001)
	for i := range rows {
		rows[i] = service.AutomationIntent{ID: service.ID(), Owner: h.user.ID, EventID: service.ID(), ActorID: h.user.ID, RuleRevision: 1}
	}
	if err := f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent&limit=1", nil, nil); status != 429 {
		t.Fatalf("budget %d", status)
	}
	if err := f.Service.DB.Delete(&rows[1000]).Error; err != nil {
		t.Fatal(err)
	}
	var page contract.Page[contract.TaskProjection]
	if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent", nil, &page); status != 200 || len(page.Items) != 0 {
		t.Fatal("missing files leaked intents")
	}
	if err := f.Service.DB.Migrator().DropTable(&service.AutomationIntent{}); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=automation_intent", nil, nil); status != 500 {
		t.Fatal("database failure became empty")
	}
}
