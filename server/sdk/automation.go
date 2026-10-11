// Controlled automation SDK surface. Field names match the design
// contract.
package sdk

import (
	"context"
	"net/url"
	"time"
)

type ChangeEventView struct {
	EventID          string `json:"event_id"`
	ResourceRef      string `json:"resource_ref"`
	ResourceRevision int64  `json:"resource_revision"`
	Kind             string `json:"kind"`
	Origin           string `json:"origin"`
	Sequence         int64  `json:"sequence"`
}

type ChangesView struct {
	Events     []ChangeEventView `json:"events"`
	NextCursor string            `json:"next_cursor,omitempty"`
	Coverage   string            `json:"coverage"`
}

type CreateChangeSubscriptionRequest struct {
	ConsumerID string `json:"consumer_id"`
	SpaceID    string `json:"space_id"`
}

type ChangeSubscription struct {
	ID         string    `json:"subscription_id"`
	ConsumerID string    `json:"consumer_id"`
	SpaceID    string    `json:"space_id"`
	Checkpoint int64     `json:"checkpoint"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AckChangesRequest struct {
	Checkpoint int64 `json:"checkpoint"`
}

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

type AutomationRule struct {
	ID            string    `json:"rule_id"`
	SpaceID       string    `json:"space_id"`
	ActionKind    string    `json:"action"`
	ActionDetail  string    `json:"action_detail"`
	Prefix        string    `json:"prefix"`
	PolicyID      string    `json:"policy_id,omitempty"`
	BudgetItems   int64     `json:"budget_items,omitempty"`
	Revision      uint64    `json:"revision"`
	Enabled       bool      `json:"enabled"`
	Paused        bool      `json:"paused"`
	PauseReason   string    `json:"pause_reason,omitempty"`
	FailureStreak int       `json:"failure_streak"`
	Checkpoint    int64     `json:"checkpoint"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AutomationIntent struct {
	ID                  string    `json:"intent_id"`
	RuleID              string    `json:"rule_id"`
	EventID             string    `json:"event_id"`
	RuleRevision        uint64    `json:"rule_revision"`
	ActorID             string    `json:"actor_id"`
	Scope               string    `json:"scope"`
	FileID              string    `json:"file_id"`
	OriginalOperationID string    `json:"original_operation_id,omitempty"`
	Status              string    `json:"status"`
	PauseReason         string    `json:"pause_reason,omitempty"`
	Error               string    `json:"error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (c *Client) ReadChanges(ctx context.Context, spaceID, cursor string, limit int) (ChangesView, error) {
	var out ChangesView
	q := url.Values{}
	if spaceID != "" {
		q.Set("space_id", spaceID)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconvItoa(limit))
	}
	path := "/api/v1/changes"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

func (c *Client) CreateChangeSubscription(ctx context.Context, q CreateChangeSubscriptionRequest) (ChangeSubscription, error) {
	var out ChangeSubscription
	e := c.JSON(ctx, "POST", "/api/v1/change-subscriptions", "", q, &out)
	return out, e
}

func (c *Client) AckChanges(ctx context.Context, subscriptionID string, checkpoint int64) (ChangeSubscription, error) {
	var out ChangeSubscription
	e := c.JSON(ctx, "POST", "/api/v1/change-subscriptions/"+url.PathEscape(subscriptionID)+"/ack", "", AckChangesRequest{Checkpoint: checkpoint}, &out)
	return out, e
}

func (c *Client) AutomationRules(ctx context.Context, spaceID string, q *AutomationRuleRequest) ([]AutomationRule, error) {
	var out []AutomationRule
	if q == nil {
		e := c.JSON(ctx, "GET", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/automation-rules", "", nil, &out)
		return out, e
	}
	e := c.JSON(ctx, "PUT", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/automation-rules", "", q, &out)
	return out, e
}

func (c *Client) RunAutomationRule(ctx context.Context, ruleID string) ([]AutomationIntent, error) {
	var out []AutomationIntent
	e := c.JSON(ctx, "POST", "/api/v1/automation-rules/"+url.PathEscape(ruleID)+"/run", "", nil, &out)
	return out, e
}

func (c *Client) GetAutomationIntent(ctx context.Context, intentID string) (AutomationIntent, error) {
	var out AutomationIntent
	e := c.JSON(ctx, "GET", "/api/v1/automation-intents/"+url.PathEscape(intentID), "", nil, &out)
	return out, e
}
