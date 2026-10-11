package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

type LabelDefinitionInput = contract.LabelDefinitionInput
type LabelDefinition = contract.LabelDefinition
type LabelDefinitionUpdate = contract.LabelDefinitionUpdate
type LabelDefinitionPage = contract.LabelDefinitionPage
type LabelDefinitionReceipt = contract.LabelDefinitionReceipt
type LabelMergePreview = contract.LabelMergePreview
type LabelMergePlanView = contract.LabelMergePlanView
type LabelMergePlanItemView = contract.LabelMergePlanItemView
type LabelMergeConfirm = contract.LabelMergeConfirm
type LabelMergeExecute = contract.LabelMergeExecute

func (c *Client) LabelDefinitions(ctx context.Context, spaceID, cursor string, limit int) (LabelDefinitionPage, error) {
	var out LabelDefinitionPage
	path := workspaceQuery("/api/v1/spaces/"+url.PathEscape(spaceID)+"/label-definitions", url.Values{"namespace": {"manual"}}, cursor, limit)
	err := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, err
}

func (c *Client) UpdateLabelDefinitions(ctx context.Context, spaceID string, input LabelDefinitionUpdate, key string) (LabelDefinitionReceipt, error) {
	var out LabelDefinitionReceipt
	err := c.JSON(ctx, "PUT", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/label-definitions", key, input, &out)
	return out, err
}

func (c *Client) LookupLabelDefinitionUpdate(ctx context.Context, spaceID, key string) (LabelDefinitionReceipt, error) {
	var out LabelDefinitionReceipt
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/label-definitions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) CreateLabelMergePlan(ctx context.Context, spaceID string, input LabelMergePreview, key string) (LabelMergePlanView, error) {
	var out LabelMergePlanView
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/label-merge-plans", key, input, &out)
	return out, err
}

func (c *Client) LookupLabelMergePlan(ctx context.Context, spaceID, key string) (LabelMergePlanView, error) {
	var out LabelMergePlanView
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/label-merge-plans/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) GetLabelMergePlan(ctx context.Context, planID, cursor string, limit int) (LabelMergePlanView, error) {
	var out LabelMergePlanView
	path := workspaceQuery("/api/v1/organization-plans/"+url.PathEscape(planID), url.Values{}, cursor, limit)
	err := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, err
}

func (c *Client) ConfirmLabelMergePlan(ctx context.Context, planID string, input LabelMergeConfirm) (LabelMergePlanView, error) {
	var out LabelMergePlanView
	err := c.JSON(ctx, "POST", "/api/v1/organization-plans/"+url.PathEscape(planID)+"/confirm", "", input, &out)
	return out, err
}

func (c *Client) ExecuteLabelMergePlan(ctx context.Context, planID string, input LabelMergeExecute) (LabelMergePlanView, error) {
	var out LabelMergePlanView
	err := c.JSON(ctx, "POST", "/api/v1/organization-plans/"+url.PathEscape(planID)+"/execute", "", input, &out)
	return out, err
}
