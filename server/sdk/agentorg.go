// Agent organization SDK surface. Field names match the design contract.
package sdk

import (
	"context"
	"net/url"
	"time"
)

type OrganizationAction struct {
	Kind         string `json:"kind"`
	Namespace    string `json:"namespace,omitempty"`
	Value        string `json:"value,omitempty"`
	TargetPrefix string `json:"target_prefix,omitempty"`
}

type CreateOrganizationPlanRequest struct {
	SourceSnapshotID string               `json:"source_snapshot_id"`
	Refs             []string             `json:"refs,omitempty"`
	Actions          []OrganizationAction `json:"actions"`
	PolicyID         string               `json:"policy_id,omitempty"`
}

type OrganizationPlanItemView struct {
	ItemKey       string `json:"item_key"`
	FileID        string `json:"file_id"`
	VersionID     string `json:"version_id"`
	Path          string `json:"path"`
	Action        string `json:"action"`
	ActionDetail  string `json:"action_detail,omitempty"`
	Before        string `json:"before,omitempty"`
	After         string `json:"after,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	ResultVersion string `json:"result_version,omitempty"`
}

type OrganizationPlanView struct {
	PlanID           string                     `json:"plan_id"`
	SpaceID          string                     `json:"space_id"`
	Revision         uint64                     `json:"revision"`
	Phase            string                     `json:"phase"`
	Risk             string                     `json:"risk"`
	ApprovalRequired bool                       `json:"approval_required"`
	PolicyID         string                     `json:"policy_id,omitempty"`
	PolicyRevision   uint64                     `json:"policy_revision,omitempty"`
	ImpactDigest     string                     `json:"impact_digest"`
	ItemsTotal       int64                      `json:"items_total"`
	ItemsDone        int64                      `json:"items_done"`
	ItemsStale       int64                      `json:"items_stale"`
	ItemsFailed      int64                      `json:"items_failed"`
	CompensationOf   string                     `json:"compensation_of,omitempty"`
	NextCursor       string                     `json:"next_cursor,omitempty"`
	Items            []OrganizationPlanItemView `json:"items,omitempty"`
}

type OrganizationPolicyRequest struct {
	Actions          []string `json:"actions"`
	Prefixes         []string `json:"prefixes,omitempty"`
	MaxItems         int64    `json:"max_items,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	Enabled          bool     `json:"enabled"`
	ExpiresAt        string   `json:"expires_at,omitempty"`
	ExpectedRevision uint64   `json:"expected_revision,omitempty"`
}

type OrganizationPolicyView struct {
	PolicyID  string     `json:"policy_id"`
	SpaceID   string     `json:"space_id"`
	Actions   []string   `json:"actions"`
	Prefixes  []string   `json:"prefixes,omitempty"`
	MaxItems  int64      `json:"max_items,omitempty"`
	MaxBytes  int64      `json:"max_bytes,omitempty"`
	Revision  uint64     `json:"revision"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (c *Client) CreateOrganizationPlan(ctx context.Context, q CreateOrganizationPlanRequest, key string) (OrganizationPlanView, error) {
	var out OrganizationPlanView
	e := c.JSON(ctx, "POST", "/api/v1/organization-plans", key, q, &out)
	return out, e
}

func (c *Client) GetOrganizationPlan(ctx context.Context, planID, cursor string, limit int) (OrganizationPlanView, error) {
	var out OrganizationPlanView
	path := "/api/v1/organization-plans/" + url.PathEscape(planID)
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconvItoa(limit))
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

func (c *Client) ExecuteOrganizationPlan(ctx context.Context, planID string, expectedRevision uint64) (OrganizationPlanView, error) {
	var out OrganizationPlanView
	e := c.JSON(ctx, "POST", "/api/v1/organization-plans/"+url.PathEscape(planID)+"/execute", "", map[string]any{"expected_revision": expectedRevision}, &out)
	return out, e
}

func (c *Client) CreateCompensationPlan(ctx context.Context, planID, key string) (OrganizationPlanView, error) {
	var out OrganizationPlanView
	e := c.JSON(ctx, "POST", "/api/v1/organization-plans/"+url.PathEscape(planID)+"/compensations", key, nil, &out)
	return out, e
}

func (c *Client) SetOrganizationPolicy(ctx context.Context, spaceID string, q OrganizationPolicyRequest) (OrganizationPolicyView, error) {
	var out OrganizationPolicyView
	e := c.JSON(ctx, "PUT", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/organization-policy", "", q, &out)
	return out, e
}

func strconvItoa(v int) string {
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
