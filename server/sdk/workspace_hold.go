package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

type WorkspaceHoldCreate = contract.WorkspaceHoldCreate
type WorkspaceHoldRelease = contract.WorkspaceHoldRelease
type WorkspaceHoldLookup = contract.WorkspaceHoldLookup
type WorkspaceHoldView = contract.WorkspaceHoldView
type WorkspaceHoldList = contract.WorkspaceHoldList
type WorkspaceHoldReceipt = contract.WorkspaceHoldReceipt

func (c *Client) ListWorkspaceHolds(ctx context.Context, fileID, versionID, cursor string, limit int) (WorkspaceHoldList, error) {
	q := url.Values{}
	workspaceString(q, "version_id", versionID)
	var out WorkspaceHoldList
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/files/"+url.PathEscape(fileID)+"/retention-holds", q, cursor, limit), "", nil, &out)
	return out, err
}

func (c *Client) CreateWorkspaceHold(ctx context.Context, fileID string, input WorkspaceHoldCreate, key string) (WorkspaceHoldReceipt, error) {
	var out WorkspaceHoldReceipt
	err := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/retention-holds", key, input, &out)
	return out, err
}

func (c *Client) ReleaseWorkspaceHold(ctx context.Context, holdID string, input WorkspaceHoldRelease, key string) (WorkspaceHoldReceipt, error) {
	var out WorkspaceHoldReceipt
	err := c.JSON(ctx, "POST", "/api/v1/retention/holds/"+url.PathEscape(holdID)+"/release-cas", key, input, &out)
	return out, err
}

func (c *Client) LookupWorkspaceHold(ctx context.Context, fileID string, input WorkspaceHoldLookup) (WorkspaceHoldReceipt, error) {
	var out WorkspaceHoldReceipt
	err := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/retention-holds/lookup", "", input, &out)
	return out, err
}
