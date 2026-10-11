package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// AgentGrantListOptions keeps nil boolean filters distinct from explicit false.
type AgentGrantListOptions struct {
	SpaceID   string
	State     string
	Search    string
	GroupID   string
	Ungrouped *bool
	Pinned    *bool
	Order     string
	Cursor    string
	Limit     int
}

type AgentOrganizationListOptions struct {
	SpaceID   string
	GroupID   string
	Ungrouped *bool
	Pinned    *bool
	Cursor    string
	Limit     int
}

// Grant and organization management is human-only. Calling these methods with
// an Agent credential does not bypass the service's identity checks.
func (c *Client) CreateAgentGrant(ctx context.Context, input CreateGrantRequest, key string) (GrantWriteView, error) {
	var out GrantWriteView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants", key, input, &out)
	return out, err
}

func (c *Client) ListAgentGrants(ctx context.Context, options AgentGrantListOptions) (Page[GrantView], error) {
	q := url.Values{}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "state", options.State)
	workspaceString(q, "q", options.Search)
	workspaceString(q, "group_id", options.GroupID)
	workspaceBool(q, "ungrouped", options.Ungrouped)
	workspaceBool(q, "pinned", options.Pinned)
	workspaceString(q, "order", options.Order)
	var out Page[GrantView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/agent-grants", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

func (c *Client) GetAgentGrant(ctx context.Context, grantID string) (GrantView, error) {
	var out GrantView
	err := c.JSON(ctx, "GET", "/api/v1/agent-grants/"+url.PathEscape(grantID), "", nil, &out)
	return out, err
}

func (c *Client) PatchAgentGrant(ctx context.Context, grantID string, input PatchGrantRequest, key string) (GrantView, error) {
	var out GrantView
	err := c.JSON(ctx, "PATCH", "/api/v1/agent-grants/"+url.PathEscape(grantID), key, input, &out)
	return out, err
}

func (c *Client) RevokeAgentGrant(ctx context.Context, grantID string, input RevisionRequest, key string) (GrantView, error) {
	var out GrantView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/"+url.PathEscape(grantID)+"/revoke", key, input, &out)
	return out, err
}

func (c *Client) CheckAgentGrant(ctx context.Context, grantID string, input RevisionRequest) (GrantChecksView, error) {
	var out GrantChecksView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/"+url.PathEscape(grantID)+"/checks", "", input, &out)
	return out, err
}

func (c *Client) LookupAgentGrant(ctx context.Context, key string) (GrantReplayView, error) {
	var out GrantReplayView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) LookupAgentGrantChange(ctx context.Context, grantID string, input GrantChangeLookupRequest) (GrantChangeReceiptView, error) {
	var out GrantChangeReceiptView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/"+url.PathEscape(grantID)+"/changes/lookup", "", input, &out)
	return out, err
}

func (c *Client) RotateAgentGrant(ctx context.Context, grantID string, input RevisionRequest, key string) (GrantWriteView, error) {
	var out GrantWriteView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/"+url.PathEscape(grantID)+"/rotate", key, input, &out)
	return out, err
}

func (c *Client) LookupAgentGrantRotation(ctx context.Context, grantID, key string) (RotationReplayView, error) {
	var out RotationReplayView
	err := c.JSON(ctx, "POST", "/api/v1/agent-grants/"+url.PathEscape(grantID)+"/rotate/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) ListAgentGroups(ctx context.Context, options PageOptions) (Page[AgentGroupView], error) {
	var out Page[AgentGroupView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/agent-groups", url.Values{}, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

func (c *Client) CreateAgentGroup(ctx context.Context, input CreateGroupRequest, key string) (AgentGroupView, error) {
	var out AgentGroupView
	err := c.JSON(ctx, "POST", "/api/v1/agent-groups", key, input, &out)
	return out, err
}

func (c *Client) GetAgentGroup(ctx context.Context, groupID string) (AgentGroupView, error) {
	var out AgentGroupView
	err := c.JSON(ctx, "GET", "/api/v1/agent-groups/"+url.PathEscape(groupID), "", nil, &out)
	return out, err
}

func (c *Client) RenameAgentGroup(ctx context.Context, groupID string, input RenameGroupRequest, key string) (AgentGroupView, error) {
	var out AgentGroupView
	err := c.JSON(ctx, "PATCH", "/api/v1/agent-groups/"+url.PathEscape(groupID), key, input, &out)
	return out, err
}

func (c *Client) DeleteAgentGroup(ctx context.Context, groupID string, input RevisionRequest, key string) (DeletedGroupView, error) {
	var out DeletedGroupView
	err := c.JSON(ctx, "DELETE", "/api/v1/agent-groups/"+url.PathEscape(groupID), key, input, &out)
	return out, err
}

func (c *Client) ListAgentOrganizations(ctx context.Context, options AgentOrganizationListOptions) (Page[AgentOrganizationView], error) {
	q := url.Values{}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "group_id", options.GroupID)
	workspaceBool(q, "ungrouped", options.Ungrouped)
	workspaceBool(q, "pinned", options.Pinned)
	var out Page[AgentOrganizationView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/agent-organizations", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

func (c *Client) GetAgentOrganization(ctx context.Context, agentID string) (AgentOrganizationView, error) {
	var out AgentOrganizationView
	err := c.JSON(ctx, "GET", "/api/v1/agent-organizations/"+url.PathEscape(agentID), "", nil, &out)
	return out, err
}

func (c *Client) PatchAgentOrganization(ctx context.Context, agentID string, input PatchOrganizationRequest, key string) (AgentOrganizationView, error) {
	var out AgentOrganizationView
	err := c.JSON(ctx, "PATCH", "/api/v1/agent-organizations/"+url.PathEscape(agentID), key, input, &out)
	return out, err
}
