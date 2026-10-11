package sdk

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// PageOptions leaves the server default limit intact when Limit is zero.
// Cursors are opaque and bound to the original filters and authenticated actor.
type PageOptions struct {
	Cursor string
	Limit  int
}

func workspaceQuery(path string, q url.Values, cursor string, limit int) string {
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

func workspaceString(q url.Values, name, value string) {
	if value != "" {
		q.Set(name, value)
	}
}

func workspaceBool(q url.Values, name string, value *bool) {
	if value != nil {
		q.Set(name, strconv.FormatBool(*value))
	}
}

func workspaceTime(q url.Values, name string, value *time.Time) {
	if value != nil {
		q.Set(name, value.Format(time.RFC3339Nano))
	}
}

type WorkspaceTaskListOptions struct {
	Kind    string
	SpaceID string
	AgentID string
	State   string
	Cursor  string
	Limit   int
}

type AgentInvocationListOptions struct {
	SpaceID          string
	AgentID          string
	Result           string
	StartedAfter     *time.Time
	StartedAtOrAfter *time.Time
	StartedBefore    *time.Time
	Order            string
	Cursor           string
	Limit            int
}

func (c *Client) GetWorkspaceHome(ctx context.Context, spaceID string, recentLimit int) (WorkspaceHome, error) {
	q := url.Values{"space_id": {spaceID}}
	if recentLimit != 0 {
		q.Set("recent_limit", strconv.Itoa(recentLimit))
	}
	var out WorkspaceHome
	err := c.JSON(ctx, "GET", "/api/v1/workspace/home?"+q.Encode(), "", nil, &out)
	return out, err
}

func (c *Client) ListWorkspaceTasks(ctx context.Context, options WorkspaceTaskListOptions) (Page[TaskProjection], error) {
	q := url.Values{"kind": {options.Kind}}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "agent_id", options.AgentID)
	workspaceString(q, "state", options.State)
	var out Page[TaskProjection]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/workspace/tasks", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

// GetWorkspaceOperationTask reads the implemented operation detail adapter.
// Other task kinds retain their existing owner-specific detail APIs.
func (c *Client) GetWorkspaceOperationTask(ctx context.Context, operationID string) (OperationTaskDetail, error) {
	var out OperationTaskDetail
	err := c.JSON(ctx, "GET", "/api/v1/workspace/tasks/operation/"+url.PathEscape(operationID), "", nil, &out)
	return out, err
}

func (c *Client) ListWorkspaceBatchTaskItems(ctx context.Context, batchID string, options PageOptions) (BatchTaskItems, error) {
	var out BatchTaskItems
	path := workspaceQuery("/api/v1/workspace/tasks/upload_batch/"+url.PathEscape(batchID)+"/items", url.Values{}, options.Cursor, options.Limit)
	err := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, err
}

func (c *Client) ListAgentInvocations(ctx context.Context, options AgentInvocationListOptions) (InvocationPage, error) {
	q := url.Values{}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "agent_id", options.AgentID)
	workspaceString(q, "result", options.Result)
	workspaceString(q, "order", options.Order)
	workspaceTime(q, "started_after", options.StartedAfter)
	workspaceTime(q, "started_at_or_after", options.StartedAtOrAfter)
	workspaceTime(q, "started_before", options.StartedBefore)
	var out InvocationPage
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/agent-invocations", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}
