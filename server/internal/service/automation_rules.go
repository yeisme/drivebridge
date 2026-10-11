package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AutomationRuleRequest is the existing human rule upsert. Its natural identity
// remains space+prefix+action_detail; changing that identity creates another rule.
type AutomationRuleRequest struct {
	ActionKind       string `json:"action"`
	ActionDetail     string `json:"action_detail"`
	Prefix           string `json:"prefix"`
	PolicyID         string `json:"policy_id,omitempty"`
	BudgetItems      int64  `json:"budget_items,omitempty"`
	Enabled          bool   `json:"enabled"`
	ClearPause       bool   `json:"clear_pause,omitempty"`
	ExpectedRevision uint64 `json:"expected_revision,omitempty"`
}

func automationDBRead(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return code("not_found")
	}
	return code("database_unavailable")
}

// Authentication is rechecked inside the same transaction as the effect. A
// caller-provided Principal is not proof that its user or credential is current.
func automationPrincipalOn(tx *gorm.DB, p Principal, write bool) error {
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	query := tx
	if write {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user User
	if err := query.Where("id = ?", p.UserID).First(&user).Error; err != nil {
		return automationDBRead(err)
	}
	if user.Disabled || user.Archived {
		return code("forbidden")
	}
	if p.Agent != nil {
		agentQuery := tx
		if write {
			agentQuery = tx.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var current Agent
		if err := agentQuery.Where("id = ? AND owner = ?", p.Agent.ID, p.UserID).First(&current).Error; err != nil {
			return automationDBRead(err)
		}
		if err := rejectWorkspaceLegacyAccess(tx, current); err != nil {
			return err
		}
		if current.Revoked || !time.Now().Before(current.Expires) {
			return code("forbidden")
		}
	}
	return nil
}
func (s *Service) automationSpaceOn(tx *gorm.DB, p Principal, id, prefix, action string, lock bool) error {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var space Space
	if err := query.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&space).Error; err != nil {
		return automationDBRead(err)
	}
	return s.allowOn(tx, p, id, prefix, action)
}
func validAutomationRule(q *AutomationRuleRequest) bool {
	namespace, label, ok := strings.Cut(q.ActionDetail, ":")
	if q.ActionKind != "add_label" || !ok || namespace == "" || len(namespace) > 96 || strings.ContainsAny(namespace, "\x00\r\n") || label == "" || len(label) > 200 || strings.ContainsAny(label, "\x00\r\n") || q.BudgetItems < 0 {
		return false
	}
	if q.Prefix != "" {
		if _, err := cleanPath(q.Prefix); err != nil {
			return false
		}
	}
	return true
}

// Rule writes serialize on the owning space before the natural-key lookup;
// updates additionally compare the revision in the write predicate.
func (s *Service) AutomationRules(p Principal, spaceID string, q *AutomationRuleRequest) ([]AutomationRule, error) {
	if q != nil && (p.Agent != nil || !validAutomationRule(q)) {
		if p.Agent != nil {
			return nil, code("forbidden")
		}
		return nil, code("invalid_request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []AutomationRule
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = s.automationRulesOn(tx, p, spaceID, q)
		return err
	})
	return result, err
}

func (s *Service) automationRulesOn(tx *gorm.DB, p Principal, spaceID string, q *AutomationRuleRequest) ([]AutomationRule, error) {
	result := []AutomationRule{}
	err := func() error {
		if err := automationPrincipalOn(tx, p, q != nil); err != nil {
			return err
		}
		prefix, action := "", "read"
		if p.Agent != nil {
			prefix = p.Agent.Prefix
		}
		if q != nil {
			prefix, action = q.Prefix, "write"
		}
		if err := s.automationSpaceOn(tx, p, spaceID, prefix, action, q != nil); err != nil {
			return err
		}
		if q == nil {
			var rows []AutomationRule
			if err := tx.Where("owner = ? AND space_id = ?", p.dataOwner(), spaceID).Order("created_at, id").Find(&rows).Error; err != nil {
				return code("database_unavailable")
			}
			for _, r := range rows {
				if p.Agent == nil || contains(prefix, r.Prefix) {
					result = append(result, r)
				}
			}
			return nil
		}
		var rule AutomationRule
		err := tx.Where("owner = ? AND space_id = ? AND prefix = ? AND action_detail = ?", p.dataOwner(), spaceID, q.Prefix, q.ActionDetail).First(&rule).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if (rule.ID != "" && q.ExpectedRevision != rule.Revision) || (rule.ID == "" && q.ExpectedRevision != 0) {
			return code("version_conflict")
		}
		if q.PolicyID != "" {
			var policy OrganizationPolicy
			if e := tx.Where("id = ? AND owner = ? AND space_id = ?", q.PolicyID, p.dataOwner(), spaceID).First(&policy).Error; e != nil {
				return automationDBRead(e)
			}
		}
		now := time.Now().UTC()
		if rule.ID == "" {
			rule = AutomationRule{ID: ID(), Owner: p.dataOwner(), SpaceID: spaceID, ActionKind: q.ActionKind, ActionDetail: q.ActionDetail, Prefix: q.Prefix, PolicyID: q.PolicyID, BudgetItems: q.BudgetItems, Revision: 1, Enabled: q.Enabled, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&rule).Error; err != nil {
				return code("database_unavailable")
			}
		} else {
			updates := map[string]any{"policy_id": q.PolicyID, "budget_items": q.BudgetItems, "enabled": q.Enabled, "revision": rule.Revision + 1, "updated_at": now}
			if q.ClearPause {
				updates["paused"] = false
				updates["pause_reason"] = ""
				updates["failure_streak"] = 0
			}
			saved := tx.Model(&AutomationRule{}).Where("id = ? AND revision = ?", rule.ID, q.ExpectedRevision).Updates(updates)
			if saved.Error != nil {
				return code("database_unavailable")
			}
			if saved.RowsAffected != 1 {
				return code("version_conflict")
			}
			if err := tx.Where("id = ?", rule.ID).First(&rule).Error; err != nil {
				return code("database_unavailable")
			}
		}
		result = append(result, rule)
		return nil
	}()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) GetAutomationIntent(p Principal, intentID string) (AutomationIntent, error) {
	var intent AutomationIntent
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := automationPrincipalOn(tx, p, false); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND owner = ?", intentID, p.dataOwner()).First(&intent).Error; err != nil {
			return automationDBRead(err)
		}
		var rule AutomationRule
		if err := tx.Where("id = ? AND owner = ?", intent.RuleID, p.dataOwner()).First(&rule).Error; err != nil {
			return automationDBRead(err)
		}
		if err := s.automationSpaceOn(tx, p, rule.SpaceID, intent.Scope, "read", false); err != nil {
			return err
		}
		var file File
		if err := tx.Where("id = ? AND owner = ? AND space_id = ?", intent.FileID, p.dataOwner(), rule.SpaceID).First(&file).Error; err != nil {
			return automationDBRead(err)
		}
		return s.allowOn(tx, p, file.SpaceID, file.Path, "read")
	})
	if err != nil {
		return AutomationIntent{}, err
	}
	return intent, nil
}

// One bounded run is atomic in the local metadata database. No external work is
// dispatched here. Transient DB failures roll back without consuming events;
// deterministic action failures retain their original intent and pause counter.
func (s *Service) RunAutomationRule(p Principal, ruleID string) ([]AutomationIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []AutomationIntent
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = s.runAutomationRuleOn(tx, p, ruleID, nil)
		return err
	})
	return result, err
}

func (s *Service) runAutomationRuleOn(tx *gorm.DB, p Principal, ruleID string, upper *int64) ([]AutomationIntent, error) {
	result := []AutomationIntent{}
	err := func() error {
		if err := automationPrincipalOn(tx, p, true); err != nil {
			return err
		}
		var rule AutomationRule
		if err := tx.Where("id = ? AND owner = ?", ruleID, p.dataOwner()).First(&rule).Error; err != nil {
			return automationDBRead(err)
		}
		if err := s.automationSpaceOn(tx, p, rule.SpaceID, rule.Prefix, "write", true); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", ruleID, p.dataOwner()).First(&rule).Error; err != nil {
			return automationDBRead(err)
		}
		if err := s.allowOn(tx, p, rule.SpaceID, rule.Prefix, "write"); err != nil {
			return err
		}
		if rule.Paused {
			return code("rule_paused")
		}
		if !rule.Enabled {
			return code("invalid_request")
		}
		namespace, label, ok := strings.Cut(rule.ActionDetail, ":")
		if !ok || !validAutomationRule(&AutomationRuleRequest{ActionKind: rule.ActionKind, ActionDetail: rule.ActionDetail, Prefix: rule.Prefix, BudgetItems: rule.BudgetItems}) {
			return code("invalid_request")
		}
		var policy *OrganizationPolicy
		if rule.PolicyID != "" {
			var row OrganizationPolicy
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND owner = ? AND space_id = ?", rule.PolicyID, p.dataOwner(), rule.SpaceID).First(&row).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return code("policy_revoked")
				}
				return code("database_unavailable")
			}
			if !row.Enabled || (row.ExpiresAt != nil && !time.Now().Before(*row.ExpiresAt)) {
				return code("policy_revoked")
			}
			policy = &row
		}
		var events []OutboxEvent
		eventsQuery := tx.Where("owner_scope = ? AND source_sequence > ? AND tombstone = ?", p.dataOwner(), rule.Checkpoint, false)
		if upper != nil {
			eventsQuery = eventsQuery.Where("source_sequence <= ?", *upper)
		}
		if err := eventsQuery.Order("source_sequence").Limit(200).Find(&events).Error; err != nil {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		consumed := rule.Checkpoint
		attempted := int64(0)
		totalBytes := int64(0)
		paths := []string{}
		for _, event := range events {
			var file File
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", event.ResourceID, p.dataOwner()).First(&file).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				consumed = event.SourceSequence
				continue
			}
			if err != nil {
				return code("database_unavailable")
			}
			if file.SpaceID != rule.SpaceID || file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) || !contains(rule.Prefix, file.Path) {
				consumed = event.SourceSequence
				continue
			}
			if rule.BudgetItems > 0 && attempted >= rule.BudgetItems {
				break
			}
			if err := s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
				return err
			}
			consumed = event.SourceSequence
			attempted++
			var intent AutomationIntent
			err = tx.Where("rule_id = ? AND event_id = ? AND rule_revision = ? AND actor_id = ? AND scope = ?", rule.ID, event.EventID, rule.Revision, p.UserID, rule.Prefix).First(&intent).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return code("database_unavailable")
			}
			if intent.ID == "" {
				intent = AutomationIntent{ID: ID(), RuleID: rule.ID, EventID: event.EventID, RuleRevision: rule.Revision, ActorID: p.UserID, Scope: rule.Prefix, Owner: p.dataOwner(), FileID: file.ID, Status: "pending", CreatedAt: now, UpdatedAt: now}
				if err := tx.Create(&intent).Error; err != nil {
					return code("database_unavailable")
				}
			}
			if intent.Owner != p.dataOwner() || intent.FileID != file.ID {
				return code("invalid_request")
			}
			if intent.Status != "pending" {
				result = append(result, intent)
				continue
			}
			reason := ""
			if p.Agent != nil && namespace != "rule" && namespace != "suggested" {
				reason = "label_namespace_forbidden"
			}
			paths = append(paths, file.Path)
			totalBytes += file.Size
			if policy != nil && !organizationPolicyCovers(*policy, []OrganizationAction{{Kind: "add_label", Namespace: namespace, Value: label}}, paths, totalBytes) {
				reason = "policy_revoked"
			}
			if reason != "" {
				if err := automationFailureOn(tx, &rule, &intent, reason, now); err != nil {
					return err
				}
				result = append(result, intent)
				if rule.Paused {
					break
				}
				continue
			}
			row := FileLabelAssignment{ID: ID(), Owner: p.dataOwner(), FileID: file.ID, Namespace: namespace, Label: label, AuthorKind: "rule", ActorID: p.UserID, CreatedAt: now, UpdatedAt: now}
			// Repeated file events or another rule's identical label do not replace the
			// existing provenance or turn an already-present label into an error.
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner"}, {Name: "file_id"}, {Name: "namespace"}, {Name: "label"}}, DoNothing: true}).Create(&row).Error; err != nil {
				return code("database_unavailable")
			}
			if rule.FailureStreak != 0 {
				rule.FailureStreak = 0
				rule.PauseReason = ""
			}
			intent.Status = "completed"
			intent.UpdatedAt = now
			written := tx.Model(&AutomationIntent{}).Where("id = ? AND status = ?", intent.ID, "pending").Updates(map[string]any{"status": intent.Status, "updated_at": now})
			if written.Error != nil || written.RowsAffected != 1 {
				return code("database_unavailable")
			}
			result = append(result, intent)
		}
		written := tx.Model(&AutomationRule{}).Where("id = ? AND revision = ?", rule.ID, rule.Revision).Updates(map[string]any{"checkpoint": consumed, "failure_streak": rule.FailureStreak, "pause_reason": rule.PauseReason, "paused": rule.Paused, "updated_at": now})
		if written.Error != nil || written.RowsAffected != 1 {
			return code("database_unavailable")
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func automationFailureOn(tx *gorm.DB, rule *AutomationRule, intent *AutomationIntent, reason string, now time.Time) error {
	if rule.PauseReason != reason {
		rule.FailureStreak = 0
	}
	rule.FailureStreak++
	rule.PauseReason = reason
	rule.Paused = rule.FailureStreak >= 3
	intent.Status = "paused"
	intent.PauseReason = reason
	intent.Error = reason
	intent.UpdatedAt = now
	written := tx.Model(&AutomationIntent{}).Where("id = ? AND status = ?", intent.ID, "pending").Updates(map[string]any{"status": intent.Status, "pause_reason": reason, "error": reason, "updated_at": now})
	if written.Error != nil || written.RowsAffected != 1 {
		return code("database_unavailable")
	}
	return nil
}

// Add the rule-aware index before removing only the over-narrow legacy index.
// Historical rows and identifiers are unchanged; no event is replayed here.
func migrateAutomationIntentIdentity(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&AutomationIntent{}); err != nil {
			return err
		}
		if tx.Migrator().HasIndex(&AutomationIntent{}, "automation_intent_identity") {
			return tx.Migrator().DropIndex(&AutomationIntent{}, "automation_intent_identity")
		}
		return nil
	})
}
