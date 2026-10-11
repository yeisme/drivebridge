// Content retrieval SDK surface. Field names match the design contract.
package sdk

import (
	"context"
	"net/url"
	"time"
)

type ContentPolicyRequest struct {
	Prefixes         []string `json:"prefixes,omitempty"`
	MimeAllowlist    []string `json:"mime_allowlist,omitempty"`
	RequestedLayers  []string `json:"requested_layers,omitempty"`
	MaxItems         int64    `json:"max_items,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	BudgetRef        string   `json:"budget_ref,omitempty"`
	EgressScope      string   `json:"egress_scope,omitempty"`
	Enabled          bool     `json:"enabled"`
	ExpectedRevision uint64   `json:"expected_revision,omitempty"`
}

type ContentPolicyView struct {
	SpaceID         string    `json:"space_id"`
	PolicyRevision  uint64    `json:"policy_revision"`
	Prefixes        []string  `json:"prefixes,omitempty"`
	MimeAllowlist   []string  `json:"mime_allowlist,omitempty"`
	RequestedLayers []string  `json:"requested_layers,omitempty"`
	MaxItems        int64     `json:"max_items,omitempty"`
	MaxBytes        int64     `json:"max_bytes,omitempty"`
	BudgetRef       string    `json:"budget_ref,omitempty"`
	EgressScope     string    `json:"egress_scope,omitempty"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateContentBindingRequest struct {
	Ref               string `json:"ref"`
	ManjuConnectionID string `json:"manju_connection_id"`
	ProjectRef        string `json:"project_ref"`
	Purpose           string `json:"purpose"`
}

type ContentBindingView struct {
	BindingID         string    `json:"binding_id"`
	FileRef           string    `json:"file_ref"`
	ManjuConnectionID string    `json:"manju_connection_id"`
	ProjectRef        string    `json:"project_ref"`
	Purpose           string    `json:"purpose"`
	State             string    `json:"state"`
	PolicyRevision    uint64    `json:"policy_revision"`
	IndexGeneration   string    `json:"index_generation,omitempty"`
	OperationID       string    `json:"operation_id,omitempty"`
	Error             string    `json:"error,omitempty"`
	RecoveryRef       string    `json:"recovery_ref,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ContentQueryRequest struct {
	Mode              string   `json:"mode"`
	Text              string   `json:"text,omitempty"`
	ImageRef          string   `json:"image_ref,omitempty"`
	SpaceIDs          []string `json:"space_ids"`
	Modalities        []string `json:"modalities,omitempty"`
	EvidenceRequired  bool     `json:"evidence_required"`
	Limit             int      `json:"limit,omitempty"`
	ManjuConnectionID string   `json:"manju_connection_id,omitempty"`
}

type ContentQueryItem struct {
	FileRef          string         `json:"file_ref"`
	AssetRevisionRef string         `json:"asset_revision_ref,omitempty"`
	EvidenceRef      string         `json:"evidence_ref,omitempty"`
	Locator          map[string]any `json:"locator,omitempty"`
	TruthLevel       string         `json:"truth_level,omitempty"`
	IndexGeneration  string         `json:"index_generation,omitempty"`
}

type ContentQueryView struct {
	Mode       string             `json:"mode"`
	Coverage   string             `json:"coverage"`
	Items      []ContentQueryItem `json:"items"`
	Missing    []string           `json:"missing,omitempty"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

func (c *Client) SetContentPolicy(ctx context.Context, spaceID string, q ContentPolicyRequest) (ContentPolicyView, error) {
	var out ContentPolicyView
	e := c.JSON(ctx, "PUT", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/content-policy", "", q, &out)
	return out, e
}

func (c *Client) GetContentPolicy(ctx context.Context, spaceID string) (ContentPolicyView, error) {
	var out ContentPolicyView
	e := c.JSON(ctx, "GET", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/content-policy", "", nil, &out)
	return out, e
}

func (c *Client) CreateContentBinding(ctx context.Context, q CreateContentBindingRequest, key string) (ContentBindingView, error) {
	var out ContentBindingView
	e := c.JSON(ctx, "POST", "/api/v1/content-bindings", key, q, &out)
	return out, e
}

func (c *Client) ListContentBindings(ctx context.Context, spaceID string) ([]ContentBindingView, error) {
	var out []ContentBindingView
	e := c.JSON(ctx, "GET", "/api/v1/content-bindings?space_id="+url.QueryEscape(spaceID), "", nil, &out)
	return out, e
}

func (c *Client) GetContentBinding(ctx context.Context, bindingID string) (ContentBindingView, error) {
	var out ContentBindingView
	e := c.JSON(ctx, "GET", "/api/v1/content-bindings/"+url.PathEscape(bindingID), "", nil, &out)
	return out, e
}

func (c *Client) ProcessContent(ctx context.Context, bindingID string, policyRevision uint64, key string) (ContentBindingView, error) {
	var out ContentBindingView
	e := c.JSON(ctx, "POST", "/api/v1/content-bindings/"+url.PathEscape(bindingID)+"/process", key, map[string]any{"policy_revision": policyRevision}, &out)
	return out, e
}

func (c *Client) QueryContent(ctx context.Context, q ContentQueryRequest) (ContentQueryView, error) {
	var out ContentQueryView
	e := c.JSON(ctx, "POST", "/api/v1/content-queries", "", q, &out)
	return out, e
}
