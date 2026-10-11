package sdk

import (
	"context"
	"net/url"
	"time"
)

// CapabilityRecovery names the next safe step for a blocked or unknown
// readiness verdict: an action identifier, the acting party, where it runs,
// and at most a safe parameter name. Never tokens or private paths.
type CapabilityRecovery struct {
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Location  string `json:"location"`
	Parameter string `json:"parameter,omitempty"`
}

// ActionReadiness mirrors the server's read-only capability projection. All
// boolean fields report independent facts; limits use pointers so an unknown
// value stays JSON null instead of collapsing to zero.
type ActionReadiness struct {
	Action               string              `json:"action"`
	Implemented          bool                `json:"implemented"`
	Configured           bool                `json:"configured"`
	Authorized           bool                `json:"authorized"`
	DependencyStatus     string              `json:"dependency_status"`
	ConnectionObservedAt *time.Time          `json:"connection_observed_at"`
	Availability         string              `json:"availability"`
	BlockReason          string              `json:"block_reason,omitempty"`
	Recovery             *CapabilityRecovery `json:"recovery"`
	Limits               map[string]*int64   `json:"limits,omitempty"`
}

// CheckCapability runs the read-only preflight for one action in one space.
// The optional ref narrows the verdict to one pinned file version.
func (c *Client) CheckCapability(ctx context.Context, action, spaceID, ref string) (ActionReadiness, error) {
	var out ActionReadiness
	q := map[string]string{"action": action, "space_id": spaceID}
	if ref != "" {
		q["ref"] = ref
	}
	e := c.JSON(ctx, "POST", "/api/v1/capabilities/check", "", q, &out)
	return out, e
}

// Capabilities fetches the capability document; a non-empty spaceID narrows
// the limits block to that space. The map keeps every legacy key parseable.
func (c *Client) Capabilities(ctx context.Context, spaceID string) (map[string]any, error) {
	path := "/api/v1/capabilities"
	if spaceID != "" {
		path += "?space_id=" + url.QueryEscape(spaceID)
	}
	var out map[string]any
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

type RuntimeStatus struct {
	Queue       map[string]int64 `json:"queue"`
	Workers     []map[string]any `json:"workers"`
	Projections []map[string]any `json:"projections"`
}

// RuntimeStatus fetches GET /api/v1/runtime/status.
func (c *Client) RuntimeStatus(ctx context.Context) (RuntimeStatus, error) {
	var out RuntimeStatus
	e := c.JSON(ctx, "GET", "/api/v1/runtime/status", "", nil, &out)
	return out, e
}

// ProjectionRebuild is the original operation view of one rebuild lifecycle.
type ProjectionRebuild struct {
	OperationID      string `json:"operation_id"`
	Generation       string `json:"generation"`
	Phase            string `json:"phase"`
	SourceWatermark  int64  `json:"source_watermark"`
	AppliedWatermark int64  `json:"applied_watermark"`
	Ready            bool   `json:"ready"`
}

// RebuildProjection starts (or replays) an operator rebuild of one
// projection; the returned operation id is the recovery handle.
func (c *Client) RebuildProjection(ctx context.Context, projection, expectedGeneration string) (ProjectionRebuild, error) {
	var out ProjectionRebuild
	body := map[string]string{}
	if expectedGeneration != "" {
		body["expected_generation"] = expectedGeneration
	}
	e := c.JSON(ctx, "POST", "/api/v1/projections/"+url.PathEscape(projection)+"/rebuild", "", body, &out)
	return out, e
}
