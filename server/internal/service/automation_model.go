package service

import "time"

// ChangeSubscription is one authenticated polling consumer inside an
// authorized scope. The first batch is authenticated polling; no arbitrary
// webhook URLs exist.
type ChangeSubscription struct {
	ID         string `json:"subscription_id" gorm:"primaryKey"`
	Owner      string `json:"-" gorm:"index"`
	ConsumerID string `json:"consumer_id"`
	SpaceID    string `json:"space_id"`
	// Checkpoint is the acknowledged high-water sequence: delivery only
	// ever moves it forward and never past undelivered pages.
	Checkpoint int64     `json:"checkpoint"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ChangeSubscription) TableName() string { return "change_subscriptions" }

// AutomationRule is the human-managed automation pre-authorization: one
// action over one prefix scope with a budget and validity. Agents never
// write rules; a paused rule stays paused until an authorized principal
// clears it.
type AutomationRule struct {
	ID            string `json:"rule_id" gorm:"primaryKey"`
	Owner         string `json:"-" gorm:"index"`
	SpaceID       string `json:"space_id"`
	ActionKind    string `json:"action"` // add_label
	ActionDetail  string `json:"action_detail"`
	Prefix        string `json:"prefix"`
	PolicyID      string `json:"policy_id,omitempty"`
	BudgetItems   int64  `json:"budget_items,omitempty"`
	Revision      uint64 `json:"revision"`
	Enabled       bool   `json:"enabled"`
	Paused        bool   `json:"paused"`
	PauseReason   string `json:"pause_reason,omitempty"`
	FailureStreak int    `json:"failure_streak"`
	// Checkpoint is the rule's own consumption position on the outbox.
	Checkpoint int64     `json:"checkpoint"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (AutomationRule) TableName() string { return "automation_rules" }

// AutomationIntent is one rule reaction to one event. The unique key
// (rule_id, event_id, rule_revision, actor, scope) makes replays land on the
// original intent and its original operation instead of duplicating work.
type AutomationIntent struct {
	ID                  string    `json:"intent_id" gorm:"primaryKey"`
	RuleID              string    `json:"rule_id" gorm:"index;uniqueIndex:automation_intent_rule_identity"`
	EventID             string    `json:"event_id" gorm:"uniqueIndex:automation_intent_rule_identity"`
	RuleRevision        uint64    `json:"rule_revision" gorm:"uniqueIndex:automation_intent_rule_identity"`
	ActorID             string    `json:"actor_id" gorm:"uniqueIndex:automation_intent_rule_identity"`
	Scope               string    `json:"scope" gorm:"uniqueIndex:automation_intent_rule_identity"`
	Owner               string    `json:"-"`
	FileID              string    `json:"file_id"`
	OriginalOperationID string    `json:"original_operation_id,omitempty"`
	Status              string    `json:"status"` // pending|completed|failed|paused|cancelled
	PauseReason         string    `json:"pause_reason,omitempty"`
	Error               string    `json:"error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (AutomationIntent) TableName() string { return "automation_intents" }
