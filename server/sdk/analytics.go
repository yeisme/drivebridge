// Typed analytical query SDK surface. Field names match the design
// contract; observed consistency reports backend unavailability honestly.
package sdk

import (
	"context"
	"net/url"
	"time"
)

type AnalyticsFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// AnalyticsSortClause is one typed ordering term: a vocabulary field plus
// asc|desc. It is not a filter: no operator, no comparison value.
type AnalyticsSortClause struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type AnalyticsQueryRequest struct {
	Dataset     string                `json:"dataset"`
	Filters     []AnalyticsFilter     `json:"filters,omitempty"`
	Dimensions  []string              `json:"dimensions,omitempty"`
	Metrics     []string              `json:"metrics,omitempty"`
	Sort        []AnalyticsSortClause `json:"sort,omitempty"`
	Limit       int                   `json:"limit,omitempty"`
	Cursor      string                `json:"cursor,omitempty"`
	Consistency string                `json:"consistency,omitempty"`
}

type AnalyticsQueryView struct {
	QueryID            string           `json:"query_id"`
	Backend            string           `json:"backend"`
	Generation         string           `json:"generation,omitempty"`
	SourceWatermark    int64            `json:"source_watermark,omitempty"`
	AppliedWatermark   int64            `json:"applied_watermark,omitempty"`
	AuthorizationEpoch int64            `json:"authorization_epoch,omitempty"`
	ObservedAt         time.Time        `json:"observed_at"`
	Coverage           string           `json:"coverage"`
	Partial            bool             `json:"partial"`
	Missing            string           `json:"missing,omitempty"`
	Rows               []map[string]any `json:"rows"`
	NextCursor         string           `json:"next_cursor,omitempty"`
}

type AnalyticsSnapshotRequest struct {
	Dataset    string            `json:"dataset"`
	Filters    []AnalyticsFilter `json:"filters,omitempty"`
	Fields     []string          `json:"fields,omitempty"`
	SpaceScope string            `json:"space_scope,omitempty"`
}

type AnalyticsSnapshot struct {
	ID               string    `json:"snapshot_id"`
	Dataset          string    `json:"dataset"`
	SchemaVersion    string    `json:"schema_version"`
	Generation       string    `json:"generation,omitempty"`
	SourceWatermark  int64     `json:"source_watermark,omitempty"`
	AppliedWatermark int64     `json:"applied_watermark,omitempty"`
	SpaceScope       string    `json:"space_scope,omitempty"`
	Fields           string    `json:"fields"`
	Rows             int64     `json:"rows"`
	ContentSHA256    string    `json:"content_sha256,omitempty"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

func (c *Client) QueryAnalytics(ctx context.Context, q AnalyticsQueryRequest) (AnalyticsQueryView, error) {
	var out AnalyticsQueryView
	e := c.JSON(ctx, "POST", "/api/v1/analytics/queries", "", q, &out)
	return out, e
}

func (c *Client) GetAnalyticsQuery(ctx context.Context, queryID string) (AnalyticsQueryView, error) {
	var out AnalyticsQueryView
	e := c.JSON(ctx, "GET", "/api/v1/analytics/queries/"+url.PathEscape(queryID), "", nil, &out)
	return out, e
}

func (c *Client) CreateAnalyticsSnapshot(ctx context.Context, q AnalyticsSnapshotRequest, key string) (AnalyticsSnapshot, error) {
	var out AnalyticsSnapshot
	e := c.JSON(ctx, "POST", "/api/v1/analytics/snapshots", key, q, &out)
	return out, e
}

func (c *Client) QueryAnalyticsSnapshot(ctx context.Context, snapshotID string, q AnalyticsQueryRequest) (AnalyticsQueryView, error) {
	var out AnalyticsQueryView
	e := c.JSON(ctx, "POST", "/api/v1/analytics/snapshots/"+url.PathEscape(snapshotID)+"/queries", "", q, &out)
	return out, e
}
