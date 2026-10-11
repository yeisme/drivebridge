package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func automationSafetyRig(t *testing.T) (*Service, Principal, Space, Operation) {
	t.Helper()
	s, _, p := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	op := uploadBytes(t, s, p, sp.ID, "auto/input.txt", []byte("automation scope proof"))
	return s, p, sp, op
}
func safetyRule(t *testing.T, s *Service, p Principal, sp Space, label string) AutomationRule {
	t.Helper()
	rows, err := s.AutomationRules(p, sp.ID, &AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:" + label, Prefix: "auto", BudgetItems: 10, Enabled: true})
	if err != nil || len(rows) != 1 {
		t.Fatal("rule fixture failed", err)
	}
	return rows[0]
}
func TestAutomationSafetyDistinctRulesRetainIndependentIntents(t *testing.T) {
	s, p, sp, op := automationSafetyRig(t)
	a, b := safetyRule(t, s, p, sp, "first"), safetyRule(t, s, p, sp, "second")
	for _, rule := range []AutomationRule{a, b} {
		intents, err := s.RunAutomationRule(p, rule.ID)
		if err != nil || len(intents) != 1 || intents[0].RuleID != rule.ID {
			t.Fatal("distinct rule reused another intent", err)
		}
		if intents[0].Status != "completed" {
			t.Fatal("run returned stale pending intent after commit")
		}
	}
	var labels int64
	if err := s.DB.Model(&FileLabelAssignment{}).Where("file_id = ?", op.FileID).Count(&labels).Error; err != nil || labels != 2 {
		t.Fatal("two independent effects not preserved")
	}
}
func TestAutomationSafetyRechecksAgentAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, scopes string
		revoked              bool
	}{{"readonly", "auto", "read", false}, {"other-prefix", "elsewhere", "read,write", false}, {"revoked", "auto", "read,write", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, sp, _ := automationSafetyRig(t)
			rule := safetyRule(t, s, p, sp, "scoped")
			a, _, err := s.createAgent(p, "caller", sp.ID, tc.prefix, tc.scopes, "", 1)
			if err != nil {
				t.Fatal(err)
			}
			if tc.revoked {
				if err = s.DB.Model(&Agent{}).Where("id = ?", a.ID).Update("revoked", true).Error; err != nil {
					t.Fatal(err)
				}
			}
			actor := Principal{UserID: p.UserID, Agent: &a}
			if _, err = s.RunAutomationRule(actor, rule.ID); err == nil {
				t.Fatal("unauthorized rule execution accepted")
			}
			var count int64
			if err = s.DB.Model(&FileLabelAssignment{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("denied execution changed labels")
			}
			if err = s.DB.Model(&AutomationIntent{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("denied execution allocated intent")
			}
			var after AutomationRule
			if err := s.DB.First(&after, "id = ?", rule.ID).Error; err != nil {
				t.Fatal(err)
			}
			if after.Checkpoint != rule.Checkpoint {
				t.Fatal("denied execution advanced checkpoint")
			}
		})
	}
}
func TestAutomationSafetyEffectReceiptAndCheckpointRollbackTogether(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "atomic")
	const fault = "automation_receipt_failure"
	if err := s.DB.Callback().Update().Before("gorm:update").Register(fault, func(tx *gorm.DB) {
		if tx.Statement.Table == "automation_intents" {
			tx.AddError(errors.New("synthetic receipt write failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := s.RunAutomationRule(p, rule.ID)
	s.DB.Callback().Update().Remove(fault)
	if err == nil {
		t.Fatal("failed receipt returned success")
	}
	for _, model := range []any{&FileLabelAssignment{}, &AutomationIntent{}} {
		var n int64
		if e := s.DB.Model(model).Count(&n).Error; e != nil || n != 0 {
			t.Fatal("failed receipt retained partial effect")
		}
	}
	var after AutomationRule
	if err := s.DB.First(&after, "id = ?", rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Checkpoint != 0 {
		t.Fatal("failed receipt consumed event")
	}
	result, err := s.RunAutomationRule(p, rule.ID)
	if err != nil || len(result) != 1 || result[0].Status != "completed" {
		t.Fatal("original event could not resume", err)
	}
}
func TestAutomationSafetyReadFailuresAreNotEmptySuccess(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "query")
	const fault = "automation_event_read_failure"
	if err := s.DB.Callback().Query().Before("gorm:query").Register(fault, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]OutboxEvent); ok {
			tx.AddError(errors.New("synthetic outbox read failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := s.RunAutomationRule(p, rule.ID)
	s.DB.Callback().Query().Remove(fault)
	if err == nil {
		t.Fatal("outbox failure returned empty success")
	}
}
func TestAutomationSafetyDisabledHumanCannotManageOrRun(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "user")
	if err := s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.AutomationRules(p, sp.ID, &AutomationRuleRequest{ActionKind: "add_label", ActionDetail: "rule:user", Prefix: "auto", ExpectedRevision: rule.Revision, Enabled: false}); err == nil {
		t.Fatal("disabled human changed rule")
	}
	if _, err := s.RunAutomationRule(p, rule.ID); err == nil {
		t.Fatal("disabled human ran rule")
	}
}
func TestAutomationSafetyConcurrentRuleCAS(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "cas")
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, enabled := range []bool{false, true} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.AutomationRules(p, sp.ID, &AutomationRuleRequest{ActionKind: "add_label", ActionDetail: rule.ActionDetail, Prefix: rule.Prefix, BudgetItems: rule.BudgetItems, ExpectedRevision: rule.Revision, Enabled: enabled})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	passed := 0
	for err := range results {
		if err == nil {
			passed++
		}
	}
	if passed != 1 {
		t.Fatalf("stale CAS writers accepted: %d", passed)
	}
}
func TestAutomationSafetyNoRunOnTrashedFile(t *testing.T) {
	s, p, sp, op := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "trash")
	if err := s.DB.Model(&File{}).Where("id = ?", op.FileID).Updates(map[string]any{"trashed": true, "updated": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := s.RunAutomationRule(p, rule.ID)
	if err != nil || len(rows) != 0 {
		t.Fatal("trashed file received rule intent", err)
	}
	var count int64
	if err := s.DB.Model(&FileLabelAssignment{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("trashed file labeled")
	}
}

func TestAutomationSafetyConcurrentRunsAndReplay(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "once")
	var wg sync.WaitGroup
	results := make(chan []AutomationIntent, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := s.RunAutomationRule(p, rule.ID)
			results <- rows
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	total := 0
	var first AutomationIntent
	for rows := range results {
		total += len(rows)
		if len(rows) > 0 {
			first = rows[0]
		}
	}
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if total != 1 {
		t.Fatal("concurrent calls duplicated effect")
	}
	if err := s.DB.Model(&AutomationRule{}).Where("id = ?", rule.ID).Update("checkpoint", 0).Error; err != nil {
		t.Fatal(err)
	}
	again, err := s.RunAutomationRule(p, rule.ID)
	if err != nil || len(again) != 1 || again[0].ID != first.ID {
		t.Fatal("event replay did not reuse original intent", err)
	}
	for _, model := range []any{&AutomationIntent{}, &FileLabelAssignment{}} {
		var n int64
		if err := s.DB.Model(model).Count(&n).Error; err != nil || n != 1 {
			t.Fatal("replay duplicated rows")
		}
	}
}
func TestAutomationSafetyPolicyRevocationAndIntentVisibility(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	rule := safetyRule(t, s, p, sp, "policy")
	policy := OrganizationPolicy{ID: ID(), Owner: p.UserID, SpaceID: sp.ID, Actions: "add_label", Prefixes: "auto", Revision: 1, Enabled: false}
	if err := s.DB.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Model(&AutomationRule{}).Where("id = ?", rule.ID).Update("policy_id", policy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunAutomationRule(p, rule.ID); err == nil || err.Error() != "policy_revoked" {
		t.Fatal("disabled policy allowed run", err)
	}
	if err := s.DB.Model(&OrganizationPolicy{}).Where("id = ?", policy.ID).Update("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := s.RunAutomationRule(p, rule.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal("authorized policy failed", err)
	}
	a, _, err := s.createAgent(p, "reader", sp.ID, "elsewhere", "read", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetAutomationIntent(Principal{UserID: p.UserID, Agent: &a}, rows[0].ID); err == nil {
		t.Fatal("outside-scope intent visible")
	}
	if _, err = s.GetAutomationIntent(p, rows[0].ID); err != nil {
		t.Fatal(err)
	}
}
