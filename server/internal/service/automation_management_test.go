package service

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func automationChange(label string) contract.AutomationChangeRequest {
	return contract.AutomationChangeRequest{Action: "add_label", ActionDetail: "rule:" + label, Prefix: "auto", BudgetItems: 10, Enabled: false}
}
func TestAutomationManagementDefaultOffAndHumanOnly(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	view, err := s.AutomationManagementAvailable(p, sp.ID)
	if err != nil || view.Available {
		t.Fatal("management not default-closed", err)
	}
	if _, err = s.ChangeAutomationRule(p, sp.ID, automationChange("closed"), "change"); err == nil || err.Error() != "unsupported_action" {
		t.Fatal("closed write allowed", err)
	}
	s.Config.AutomationManagement = true
	a, _, err := s.createAgent(p, "agent", sp.ID, "", "read,write", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	actor := Principal{UserID: p.UserID, Agent: &a}
	if _, err = s.ChangeAutomationRule(actor, sp.ID, automationChange("closed"), "change"); err == nil {
		t.Fatal("Agent self-approved rule")
	}
	view, err = s.AutomationManagementAvailable(p, sp.ID)
	if err != nil || !view.Available {
		t.Fatal("explicit management unavailable", err)
	}
}
func TestAutomationManagementRuleReceiptReplaysOriginalRevision(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	s.Config.AutomationManagement = true
	input := automationChange("receipt")
	first, err := s.ChangeAutomationRule(p, sp.ID, input, "change-one")
	if err != nil || first.ResultRevision != 1 {
		t.Fatal(err)
	}
	input.Enabled = true
	input.ExpectedRevision = 1
	second, err := s.ChangeAutomationRule(p, sp.ID, input, "change-two")
	if err != nil || second.RuleID != first.RuleID || second.ResultRevision != 2 {
		t.Fatal("rule change failed", err)
	}
	if _, err = s.ChangeAutomationRule(p, sp.ID, input, "change-three"); err == nil || err.Error() != "version_conflict" {
		t.Fatal("stale revision accepted", err)
	}
	replay, err := s.ChangeAutomationRule(p, sp.ID, automationChange("receipt"), "change-one")
	if err != nil || replay != first {
		t.Fatal("replay replaced historical result", err)
	}
	lookup, err := s.LookupAutomationRequest(p, sp.ID, "automation.change", "change-one")
	if err != nil || lookup != first {
		t.Fatal("original receipt lookup failed", err)
	}
	if _, err = s.ChangeAutomationRule(p, sp.ID, automationChange("different"), "change-one"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed rule", err)
	}
	var count int64
	if err := s.DB.Model(&AutomationRule{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("replay duplicated rule")
	}
}
func TestAutomationManagementPreviewAndExactExecutionReceipt(t *testing.T) {
	s, p, sp, op := automationSafetyRig(t)
	s.Config.AutomationManagement = true
	q := automationChange("execute")
	q.Enabled = true
	rule, err := s.ChangeAutomationRule(p, sp.ID, q, "create")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil || len(preview.Items) != 1 || !preview.Executable || preview.Items[0].VersionID != op.VersionID {
		t.Fatal("preview failed", err)
	}
	for _, model := range []any{&AutomationIntent{}, &FileLabelAssignment{}} {
		var count int64
		if err := s.DB.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("preview emitted work")
		}
	}
	again, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil || again.PreviewDigest != preview.PreviewDigest {
		t.Fatal("unchanged preview drifted", err)
	}
	input := contract.AutomationExecuteRequest{ExpectedRevision: preview.Revision, PreviewDigest: preview.PreviewDigest}
	receipt, err := s.ExecuteAutomationPreview(p, rule.RuleID, input, "execute-one")
	if err != nil {
		t.Fatal(err)
	}
	uploadBytes(t, s, p, sp.ID, "auto/later.txt", []byte("not approved by original preview"))
	replay, err := s.ExecuteAutomationPreview(p, rule.RuleID, input, "execute-one")
	if err != nil || replay != receipt {
		t.Fatal("replay consumed new events", err)
	}
	lookup, err := s.LookupAutomationRequest(p, rule.RuleID, "automation.execute", "execute-one")
	if err != nil || lookup != receipt {
		t.Fatal("execution lookup failed", err)
	}
	var count int64
	if err := s.DB.Model(&AutomationIntent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("original execution duplicated work")
	}
	if _, err = s.ExecuteAutomationPreview(p, rule.RuleID, input, "execute-new"); err == nil || err.Error() != "plan_stale" {
		t.Fatal("stale preview reran", err)
	}
	later, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil || len(later.Items) != 1 || later.Items[0].FileID == op.FileID {
		t.Fatal("unconsumed later event lost", err)
	}
}
func TestAutomationManagementPreviewDriftRefusesBeforeEffects(t *testing.T) {
	for _, kind := range []string{"version", "path", "trash", "new-event", "rule"} {
		t.Run(kind, func(t *testing.T) {
			s, p, sp, op := automationSafetyRig(t)
			s.Config.AutomationManagement = true
			q := automationChange("drift")
			q.Enabled = true
			rule, err := s.ChangeAutomationRule(p, sp.ID, q, "create")
			if err != nil {
				t.Fatal(err)
			}
			preview, err := s.PreviewAutomationRule(p, rule.RuleID)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "version":
				err = s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("version_id", ID()).Error
			case "path":
				err = s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("path", "elsewhere/input.txt").Error
			case "trash":
				err = s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("trashed", true).Error
			case "new-event":
				uploadBytes(t, s, p, sp.ID, "auto/new.txt", []byte("new unapproved event"))
			case "rule":
				q.Enabled = false
				q.ExpectedRevision = 1
				_, err = s.ChangeAutomationRule(p, sp.ID, q, "disable")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ExecuteAutomationPreview(p, rule.RuleID, contract.AutomationExecuteRequest{ExpectedRevision: preview.Revision, PreviewDigest: preview.PreviewDigest}, "execute"); err == nil || err.Error() != "plan_stale" {
				t.Fatal("preview drift accepted", err)
			}
			var count int64
			if err := s.DB.Model(&AutomationIntent{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("stale preview allocated work")
			}
		})
	}
}
func TestAutomationManagementReceiptFailureRollsBackExecution(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	s.Config.AutomationManagement = true
	q := automationChange("atomic")
	q.Enabled = true
	rule, err := s.ChangeAutomationRule(p, sp.ID, q, "create")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil {
		t.Fatal(err)
	}
	const fault = "automation_original_receipt_fault"
	if err = s.DB.Callback().Create().Before("gorm:create").Register(fault, func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			tx.AddError(errors.New("synthetic receipt failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.ExecuteAutomationPreview(p, rule.RuleID, contract.AutomationExecuteRequest{ExpectedRevision: preview.Revision, PreviewDigest: preview.PreviewDigest}, "execute")
	if removeErr := s.DB.Callback().Create().Remove(fault); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err == nil {
		t.Fatal("failed receipt committed effects")
	}
	for _, model := range []any{&AutomationIntent{}, &FileLabelAssignment{}} {
		var n int64
		if err := s.DB.Model(model).Count(&n).Error; err != nil || n != 0 {
			t.Fatal("receipt failure retained side effect")
		}
	}
	same, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil || same.PreviewDigest != preview.PreviewDigest {
		t.Fatal("rollback advanced checkpoint", err)
	}
}
func TestAutomationManagementScopeAndPausedRules(t *testing.T) {
	s, p, sp, _ := automationSafetyRig(t)
	s.Config.AutomationManagement = true
	rule, err := s.ChangeAutomationRule(p, sp.ID, automationChange("disabled"), "create")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAutomationRule(p, rule.RuleID)
	if err != nil || preview.Executable {
		t.Fatal("disabled rule executable", err)
	}
	if _, err = s.ExecuteAutomationPreview(p, rule.RuleID, contract.AutomationExecuteRequest{ExpectedRevision: 1, PreviewDigest: preview.PreviewDigest}, "execute"); err == nil || err.Error() != "rule_paused" {
		t.Fatal("disabled rule executed", err)
	}
	other, err := s.ensureUser("fixture", "other", "Other")
	if err != nil {
		t.Fatal(err)
	}
	outsider := Principal{UserID: other.ID}
	if _, err = s.PreviewAutomationRule(outsider, rule.RuleID); err == nil {
		t.Fatal("foreign preview visible")
	}
	if _, err = s.LookupAutomationRequest(outsider, sp.ID, "automation.change", "create"); err == nil {
		t.Fatal("foreign receipt visible")
	}
}
