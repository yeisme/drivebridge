// Cross-storage transfer SDK surface. Field names match the design
// contract; copying and source cleanup stay separate authorizations.
package sdk

import (
	"context"
	"net/url"
	"strconv"
)

type TransferSource struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256,omitempty"`
}

type CreateTransferPlanRequest struct {
	Sources             []TransferSource `json:"sources"`
	TargetSpaceID       string           `json:"target_space_id"`
	TargetPath          string           `json:"target_path"`
	Mode                string           `json:"mode"`
	ConflictPolicy      string           `json:"conflict_policy"`
	CrossSpaceConfirmed bool             `json:"cross_space_confirmed,omitempty"`
}

type TransferItem struct {
	ItemKey         string `json:"item_key"`
	SourceFileID    string `json:"source_file_id"`
	SourceVersionID string `json:"source_version_id"`
	SourceSHA256    string `json:"source_sha256,omitempty"`
	TargetPath      string `json:"target_path"`
	CopyOperationID string `json:"copy_operation_id,omitempty"`
	TargetRef       string `json:"target_ref,omitempty"`
	TargetSHA256    string `json:"target_sha256,omitempty"`
	Status          string `json:"status"`
	ResultCertainty string `json:"result_certainty,omitempty"`
	Error           string `json:"error,omitempty"`
	CleanupState    string `json:"cleanup_state,omitempty"`
}

type TransferPlan struct {
	PlanID                  string         `json:"plan_id"`
	Mode                    string         `json:"mode"`
	TargetSpaceID           string         `json:"target_space_id"`
	TargetPath              string         `json:"target_path"`
	ConflictPolicy          string         `json:"conflict_policy"`
	Revision                uint64         `json:"revision"`
	Phase                   string         `json:"phase"`
	OperationID             string         `json:"operation_id,omitempty"`
	ItemsTotal              int64          `json:"items_total"`
	ItemsCopied             int64          `json:"items_copied"`
	ItemsPending            int64          `json:"items_pending"`
	ItemsPaused             int64          `json:"items_paused"`
	ItemsFailed             int64          `json:"items_failed"`
	SourceCleanupAuthorized bool           `json:"source_cleanup_authorized"`
	NextCursor              string         `json:"next_cursor,omitempty"`
	Items                   []TransferItem `json:"items,omitempty"`
}

func (c *Client) CreateTransferPlan(ctx context.Context, q CreateTransferPlanRequest, key string) (TransferPlan, error) {
	var out TransferPlan
	e := c.JSON(ctx, "POST", "/api/v1/transfer-plans", key, q, &out)
	return out, e
}

func (c *Client) GetTransferPlan(ctx context.Context, planID, cursor string, limit int) (TransferPlan, error) {
	var out TransferPlan
	path := "/api/v1/transfer-plans/" + url.PathEscape(planID)
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

func (c *Client) ExecuteTransferPlan(ctx context.Context, planID string, expectedRevision uint64) (TransferPlan, error) {
	var out TransferPlan
	e := c.JSON(ctx, "POST", "/api/v1/transfer-plans/"+url.PathEscape(planID)+"/execute", "", map[string]any{"expected_revision": expectedRevision}, &out)
	return out, e
}

func (c *Client) CleanupTransferSource(ctx context.Context, planID string, expectedRevision uint64) (TransferPlan, error) {
	var out TransferPlan
	e := c.JSON(ctx, "POST", "/api/v1/transfer-plans/"+url.PathEscape(planID)+"/cleanup-source", "", map[string]any{"expected_revision": expectedRevision}, &out)
	return out, e
}

func (c *Client) CancelTransferPlan(ctx context.Context, planID string) (TransferPlan, error) {
	var out TransferPlan
	e := c.JSON(ctx, "POST", "/api/v1/transfer-plans/"+url.PathEscape(planID)+"/cancel", "", nil, &out)
	return out, e
}
