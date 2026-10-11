package service

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) automationManagementOn(tx *gorm.DB, p Principal) error {
	if !s.Config.AutomationManagement {
		return code("unsupported_action")
	}
	if p.Agent != nil {
		return code("forbidden")
	}
	return automationPrincipalOn(tx, p, true)
}
func automationReceipt(row WorkspaceRequest, space string) contract.AutomationReceipt {
	return contract.AutomationReceipt{RuleID: row.ResourceID, SpaceID: space, Action: row.Action, RequestDigest: row.RequestDigest, ResultRevision: row.ResultRevision, CommittedAt: row.CreatedAt}
}
func automationRequestQuery(tx *gorm.DB, p Principal, action, key string) *gorm.DB {
	return tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.dataOwner(), p.TenantID, p.UserID, action, key)
}
func (s *Service) AutomationManagementAvailable(p Principal, space string) (contract.AutomationManagement, error) {
	var out contract.AutomationManagement
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		if err := automationPrincipalOn(tx, p, false); err != nil {
			return err
		}
		if err := s.automationSpaceOn(tx, p, space, "", "read", false); err != nil {
			return err
		}
		if !s.Config.AutomationManagement {
			return nil
		}
		if err := s.allowOn(tx, p, space, "", "write"); err != nil {
			if workspaceVisibilityDenied(err) {
				return nil
			}
			return err
		}
		out.Available = true
		return nil
	})
	return out, err
}
func (s *Service) ChangeAutomationRule(p Principal, space string, q contract.AutomationChangeRequest, key string) (contract.AutomationReceipt, error) {
	var out contract.AutomationReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	input := AutomationRuleRequest{ActionKind: q.Action, ActionDetail: q.ActionDetail, Prefix: q.Prefix, PolicyID: q.PolicyID, BudgetItems: q.BudgetItems, Enabled: q.Enabled, ClearPause: q.ClearPause, ExpectedRevision: q.ExpectedRevision}
	if !validAutomationRule(&input) || q.BudgetItems < 1 || q.BudgetItems > 200 || q.ExpectedRevision >= 9007199254740991 {
		return out, code("invalid_request")
	}
	raw, _ := json.Marshal(struct {
		Space string                           `json:"space"`
		Input contract.AutomationChangeRequest `json:"input"`
	}{space, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.automationManagementOn(tx, p); err != nil {
			return err
		}
		if err := s.automationSpaceOn(tx, p, space, q.Prefix, "write", true); err != nil {
			return err
		}
		var prior WorkspaceRequest
		err := automationRequestQuery(tx, p, "automation.change", key).First(&prior).Error
		if err == nil {
			if prior.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = automationReceipt(prior, space)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		rules, err := s.automationRulesOn(tx, p, space, &input)
		if err != nil {
			return err
		}
		row := WorkspaceRequest{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, ActorID: p.UserID, Action: "automation.change", RequestKey: key, RequestDigest: digest, ResourceID: rules[0].ID, ResultRevision: rules[0].Revision, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		out = automationReceipt(row, space)
		return nil
	})
	if err != nil {
		return contract.AutomationReceipt{}, err
	}
	return out, nil
}
func (s *Service) LookupAutomationRequest(p Principal, target, action, key string) (contract.AutomationReceipt, error) {
	var out contract.AutomationReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if action != "automation.change" && action != "automation.execute" {
		return out, code("invalid_request")
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.automationManagementOn(tx, p); err != nil {
			return err
		}
		var row WorkspaceRequest
		if err := automationRequestQuery(tx, p, action, key).First(&row).Error; err != nil {
			return automationDBRead(err)
		}
		var rule AutomationRule
		if err := tx.Where("id = ? AND owner = ?", row.ResourceID, p.dataOwner()).First(&rule).Error; err != nil {
			return automationDBRead(err)
		}
		if (action == "automation.change" && rule.SpaceID != target) || (action == "automation.execute" && rule.ID != target) {
			return code("not_found")
		}
		if err := s.automationSpaceOn(tx, p, rule.SpaceID, rule.Prefix, "read", false); err != nil {
			return err
		}
		out = automationReceipt(row, rule.SpaceID)
		return nil
	})
	if err != nil {
		return contract.AutomationReceipt{}, err
	}
	return out, nil
}

type automationPreviewEvent struct {
	EventID     string
	Sequence    int64
	FileID      string
	VersionID   string
	Path        string
	Size        int64
	Unavailable bool
}

// Locks are held through execute, so a preview and its effect share one exact
// file/rule/policy snapshot. New events after enumeration are bounded out.
func (s *Service) automationPreviewOn(tx *gorm.DB, p Principal, id string) (contract.AutomationPreview, int64, error) {
	out := contract.AutomationPreview{Items: []contract.AutomationPreviewItem{}, Coverage: "complete"}
	if err := s.automationManagementOn(tx, p); err != nil {
		return out, 0, err
	}
	var rule AutomationRule
	if err := tx.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&rule).Error; err != nil {
		return out, 0, automationDBRead(err)
	}
	if err := s.automationSpaceOn(tx, p, rule.SpaceID, rule.Prefix, "write", true); err != nil {
		return out, 0, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", id, p.dataOwner()).First(&rule).Error; err != nil {
		return out, 0, automationDBRead(err)
	}
	if err := s.allowOn(tx, p, rule.SpaceID, rule.Prefix, "write"); err != nil {
		return out, 0, err
	}
	out.RuleID, out.SpaceID, out.Revision, out.Checkpoint = rule.ID, rule.SpaceID, rule.Revision, rule.Checkpoint
	out.Executable = rule.Enabled && !rule.Paused
	var policy *OrganizationPolicy
	if rule.PolicyID != "" {
		var row OrganizationPolicy
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND owner = ? AND space_id = ?", rule.PolicyID, p.dataOwner(), rule.SpaceID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return out, 0, code("policy_revoked")
			}
			return out, 0, code("database_unavailable")
		}
		policy = &row
		out.PolicyMaxBytes, out.PolicyExpiresAt = row.MaxBytes, row.ExpiresAt
		if !row.Enabled || (row.ExpiresAt != nil && !time.Now().Before(*row.ExpiresAt)) {
			out.Executable = false
		}
	}
	var events []OutboxEvent
	if err := tx.Where("owner_scope = ? AND source_sequence > ? AND tombstone = ?", p.dataOwner(), rule.Checkpoint, false).Order("source_sequence").Limit(201).Find(&events).Error; err != nil {
		return out, 0, code("database_unavailable")
	}
	if len(events) > 200 {
		out.Coverage = "partial"
		events = events[:200]
	}
	observed := []automationPreviewEvent{}
	upper := rule.Checkpoint
	for _, event := range events {
		item := automationPreviewEvent{EventID: event.EventID, Sequence: event.SourceSequence, FileID: event.ResourceID}
		var file File
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", event.ResourceID, p.dataOwner()).First(&file).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return out, 0, code("database_unavailable")
		}
		if err == nil {
			item.VersionID, item.Path, item.Size = file.VersionID, file.Path, file.Size
		}
		item.Unavailable = err != nil || file.SpaceID != rule.SpaceID || file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) || !contains(rule.Prefix, file.Path)
		if !item.Unavailable && rule.BudgetItems > 0 && int64(len(out.Items)) >= rule.BudgetItems {
			out.Coverage = "partial"
			break
		}
		out.Scanned++
		upper = event.SourceSequence
		observed = append(observed, item)
		if item.Unavailable {
			continue
		}
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
			return out, 0, err
		}
		out.Items = append(out.Items, contract.AutomationPreviewItem{FileID: file.ID, VersionID: file.VersionID, Path: file.Path, SizeBytes: file.Size})
	}
	// Include all rule/policy fields and every consumed event, not just displayed
	// matches. A checkpoint or skipped-file change invalidates the preview too.
	raw, _ := json.Marshal(struct {
		Actor, Tenant string
		Rule          AutomationRule
		Policy        *OrganizationPolicy
		Events        []automationPreviewEvent
		Coverage      string
	}{p.UserID, p.TenantID, rule, policy, observed, out.Coverage})
	out.PreviewDigest = Hash(string(raw))
	return out, upper, nil
}
func (s *Service) PreviewAutomationRule(p Principal, id string) (contract.AutomationPreview, error) {
	var out contract.AutomationPreview
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error { var err error; out, _, err = s.automationPreviewOn(tx, p, id); return err })
	if err != nil {
		return contract.AutomationPreview{}, err
	}
	return out, nil
}
func (s *Service) ExecuteAutomationPreview(p Principal, id string, q contract.AutomationExecuteRequest, key string) (contract.AutomationReceipt, error) {
	var out contract.AutomationReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	decoded, decodeErr := hex.DecodeString(q.PreviewDigest)
	if q.ExpectedRevision == 0 || q.ExpectedRevision > 9007199254740991 || decodeErr != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != q.PreviewDigest {
		return out, code("invalid_request")
	}
	raw, _ := json.Marshal(struct {
		Rule  string                            `json:"rule"`
		Input contract.AutomationExecuteRequest `json:"input"`
	}{id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.automationManagementOn(tx, p); err != nil {
			return err
		}
		var rule AutomationRule
		if err := tx.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&rule).Error; err != nil {
			return automationDBRead(err)
		}
		if err := s.automationSpaceOn(tx, p, rule.SpaceID, rule.Prefix, "write", true); err != nil {
			return err
		}
		var prior WorkspaceRequest
		err := automationRequestQuery(tx, p, "automation.execute", key).First(&prior).Error
		if err == nil {
			if prior.ResourceID != id || prior.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = automationReceipt(prior, rule.SpaceID)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		preview, upper, err := s.automationPreviewOn(tx, p, id)
		if err != nil {
			return err
		}
		if preview.Revision != q.ExpectedRevision || preview.PreviewDigest != q.PreviewDigest {
			return code("plan_stale")
		}
		if !preview.Executable {
			return code("rule_paused")
		}
		if _, err := s.runAutomationRuleOn(tx, p, id, &upper); err != nil {
			return err
		}
		receipt := WorkspaceRequest{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, ActorID: p.UserID, Action: "automation.execute", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: preview.Revision, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = automationReceipt(receipt, rule.SpaceID)
		return nil
	})
	if err != nil {
		return contract.AutomationReceipt{}, err
	}
	return out, nil
}
