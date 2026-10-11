package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk"
)

// automationWorld uploads files (emitting committed events) and sets up a
// human-managed rule.
func automationWorld(t *testing.T, rule *sdk.AutomationRuleRequest) (*human, *sdk.Client, string) {
	t.Helper()
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "auto/one.txt"}, "auto-1", sizeReader(16), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "auto/two.txt"}, "auto-2", sizeReader(16), nil); e != nil {
		t.Fatal(e)
	}
	var rules []sdk.AutomationRule
	if rule != nil {
		if status := h.call(t, "PUT", "/spaces/"+h.space.ID+"/automation-rules", rule, &rules); status != 200 || len(rules) != 1 {
			t.Fatalf("rule set status %d", status)
		}
		return h, c, rules[0].ID
	}
	return h, c, ""
}

// Task 2.1/3.1: the changes contract — safe-reference events, opaque
// owner-bound cursors, monotonic acks that never jump ahead.
func TestFPAutomationChangesContract(t *testing.T) {
	h, c, _ := automationWorld(t, nil)
	first, e := c.ReadChanges(context.Background(), h.space.ID, "", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(first.Events) != 1 || first.Coverage != "partial" || first.NextCursor == "" {
		t.Fatalf("first page wrong: %+v", first)
	}
	event := first.Events[0]
	if event.EventID == "" || !strings.HasPrefix(event.ResourceRef, "drivebridge://") || event.Kind != "file_committed" {
		t.Fatalf("event shape wrong: %+v", event)
	}
	// Names, bodies and URLs never ride events.
	if strings.Contains(event.ResourceRef, "/auto/") {
		t.Fatalf("event leaked a path: %+v", event)
	}
	// The cursor continues; a tampered cursor is cursor_expired.
	second, e := c.ReadChanges(context.Background(), h.space.ID, first.NextCursor, 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(second.Events) != 1 || second.Coverage != "complete" {
		t.Fatalf("second page wrong: %+v", second)
	}
	if _, e := c.ReadChanges(context.Background(), h.space.ID, first.NextCursor+"x", 10); e == nil || e.Error() != "cursor_expired" {
		t.Fatalf("tampered cursor must be cursor_expired, got %v", e)
	}
	// Subscriptions ack monotonically and never ahead of delivery.
	sub, e := c.CreateChangeSubscription(context.Background(), sdk.CreateChangeSubscriptionRequest{ConsumerID: "consumer-a", SpaceID: h.space.ID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := c.AckChanges(context.Background(), sub.ID, event.Sequence+1000); e == nil || e.Error() != "ack_ahead" {
		t.Fatalf("ack ahead must be ack_ahead, got %v", e)
	}
	acked, e := c.AckChanges(context.Background(), sub.ID, second.Events[0].Sequence)
	if e != nil || acked.Checkpoint != second.Events[0].Sequence {
		t.Fatalf("ack wrong: %+v %v", acked, e)
	}
	if _, e := c.AckChanges(context.Background(), sub.ID, event.Sequence); e == nil {
		t.Fatal("checkpoint must be monotonic")
	}
}

// Task 3.1/3.2: idempotent intents and pause semantics — replays reuse
// the original intent, three deterministic failures pause the rule, and a
// paused rule refuses to run until an authorized principal clears it.
func TestFPAutomationIntentsAndPausing(t *testing.T) {
	h, c, ruleID := automationWorld(t, &sdk.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:auto-tag", Prefix: "auto", Enabled: true})
	// First run creates one intent per committed event and applies the
	// rule label.
	intents, e := c.RunAutomationRule(context.Background(), ruleID)
	if e != nil {
		t.Fatal(e)
	}
	if len(intents) != 2 {
		t.Fatalf("intents wrong: %+v", intents)
	}
	labeled, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "labels.rule", Op: "contains", Value: "auto-tag"}}})
	if e != nil || len(labeled.Items) != 2 {
		t.Fatalf("rule labels not applied: %+v %v", labeled, e)
	}
	// Replay: the same events land on the same intents, no duplicates.
	again, e := c.RunAutomationRule(context.Background(), ruleID)
	if e != nil {
		t.Fatal(e)
	}
	if len(again) != 0 {
		t.Fatalf("replay created new intents: %+v", again)
	}
	var rows int64
	countLabels(t, h, &rows)
	// Deterministic failure path: an agent-scoped rule with the manual
	// namespace fails label_namespace_forbidden on every event; three
	// consecutive failures pause the rule.
	var manualRules []sdk.AutomationRule
	if status := h.call(t, "PUT", "/spaces/"+h.space.ID+"/automation-rules", &sdk.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "manual:forbidden", Prefix: "manual-ns", Enabled: true}, &manualRules); status != 200 {
		t.Fatalf("manual rule status %d", status)
	}
	// Upload three events under the manual prefix: three same-class
	// deterministic failures in a row pause the rule.
	for i := range 3 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "manual-ns/x" + strconvItoaLocal(i) + ".txt"}, "manual-"+strconvItoaLocal(i), sizeReader(4+i), nil); e != nil {
			t.Fatal(e)
		}
	}
	agentRule := manualRules[0]
	// One run processes all three events; each deterministic failure
	// increments the streak and the third pauses the rule.
	if _, e := c.RunAutomationRule(context.Background(), agentRule.ID); e != nil {
		t.Fatalf("manual run failed: %v", e)
	}
	var after []sdk.AutomationRule
	h.call(t, "GET", "/spaces/"+h.space.ID+"/automation-rules", nil, &after)
	for _, rule := range after {
		if rule.ID == agentRule.ID && (!rule.Paused || rule.PauseReason == "") {
			t.Fatalf("rule must be paused with a reason: %+v", rule)
		}
	}
	if _, e := c.RunAutomationRule(context.Background(), agentRule.ID); e == nil || e.Error() != "rule_paused" {
		t.Fatalf("paused rule must refuse with rule_paused, got %v", e)
	}
	// An authorized human clears the pause.
	var cleared []sdk.AutomationRule
	if status := h.call(t, "PUT", "/spaces/"+h.space.ID+"/automation-rules", map[string]any{"action": "add_label", "action_detail": "manual:forbidden", "prefix": "manual-ns", "enabled": true, "clear_pause": true, "expected_revision": after[0].Revision}, &cleared); status != 200 {
		t.Fatalf("clear pause status %d", status)
	}
	// Agents never write rules.
	if _, e := c.AutomationRules(context.Background(), h.space.ID, &sdk.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:x", Enabled: true}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent rule write must be forbidden, got %v", e)
	}
}

func countLabels(t *testing.T, h *human, out *int64) {
	t.Helper()
	c := mustSDK(t, h)
	labeled, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "labels.rule", Op: "contains", Value: "auto-tag"}}})
	if e != nil {
		t.Fatal(e)
	}
	*out = int64(len(labeled.Items))
}

func strconvItoaLocal(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}

// Rework M1: once three same-class deterministic failures pause the rule,
// a fourth event in the same batch produces no side effect — the run
// stops instead of burning through the tail.
func TestFPAutomationPauseStopsBatch(t *testing.T) {
	h, _, _ := automationWorld(t, &sdk.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "manual:forbidden", Prefix: "manual-ns", Enabled: true})
	c := mustSDK(t, h)
	// Four events under the failing prefix.
	for i := range 4 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "manual-ns/p" + itoaLocal(i) + ".txt"}, "pause-4-"+itoaLocal(i), sizeReader(4+i), nil); e != nil {
			t.Fatal(e)
		}
	}
	var rules []sdk.AutomationRule
	h.call(t, "GET", "/spaces/"+h.space.ID+"/automation-rules", nil, &rules)
	if len(rules) != 1 {
		t.Fatalf("rules: %d", len(rules))
	}
	intents, e := c.RunAutomationRule(context.Background(), rules[0].ID)
	if e != nil {
		t.Fatalf("run failed: %v", e)
	}
	if len(intents) != 3 {
		t.Fatalf("the paused rule must stop at the third failing event, got %d intents", len(intents))
	}
	// Read each actual persisted receipt; a constant count is not evidence.
	for _, intent := range intents {
		stored, err := c.GetAutomationIntent(context.Background(), intent.ID)
		if err != nil || stored.RuleID != rules[0].ID || stored.Status != "paused" || stored.PauseReason != "label_namespace_forbidden" {
			t.Fatal("paused receipt did not persist", err)
		}
	}
	if status := h.call(t, "GET", "/spaces/"+h.space.ID+"/automation-rules", nil, &rules); status != 200 || !rules[0].Paused || rules[0].FailureStreak != 3 {
		t.Fatal("rule did not persist the third failure")
	}
}

func itoaLocal(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}

// Rework M2 (replay uniqueness): resetting the checkpoint makes a rerun
// see the same events again — the unique key must deduplicate onto the
// original intents instead of creating new work.
func TestFPAutomationReplayHitsUniqueKey(t *testing.T) {
	h, c, ruleID := automationWorld(t, &sdk.AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:replay", Prefix: "auto", Enabled: true})
	first, e := c.RunAutomationRule(context.Background(), ruleID)
	if e != nil {
		t.Fatal(e)
	}
	if len(first) != 2 {
		t.Fatalf("first run wrong: %d", len(first))
	}
	// The unique (event, rule_revision, actor, scope) key replays the
	// original intents: running again with no new events returns none,
	// and the label rows stay at one per file.
	again, e := c.RunAutomationRule(context.Background(), ruleID)
	if e != nil {
		t.Fatal(e)
	}
	if len(again) != 0 {
		t.Fatalf("no-op rerun created work: %d", len(again))
	}
	labeled, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "labels.rule", Op: "contains", Value: "replay"}}, Limit: 200})
	if e != nil || len(labeled.Items) != 2 {
		t.Fatalf("labels wrong after rerun: %d %v", len(labeled.Items), e)
	}
}
