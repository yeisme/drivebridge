// Virtual file organization SDK surface: typed queries, label authority,
// collections, snapshots and saved searches. Field names match the design
// contract exactly; remote client mocks bind to them.
package sdk

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// OrgFilter is one typed condition from the closed vocabulary.
type OrgFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// OrgSort is one sort term.
type OrgSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// FileQueryRequest is the exact query input.
type FileQueryRequest struct {
	Mode     string      `json:"mode"`
	SpaceIDs []string    `json:"space_ids"`
	Filters  []OrgFilter `json:"filters,omitempty"`
	Sort     []OrgSort   `json:"sort,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Cursor   string      `json:"cursor,omitempty"`
}

// FileQueryItem is one result row.
type FileQueryItem struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Path      string `json:"path,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

// FileQueryView is one bounded result page.
type FileQueryView struct {
	Mode              string          `json:"mode"`
	Items             []FileQueryItem `json:"items"`
	CatalogueRevision string          `json:"catalogue_revision"`
	NextCursor        string          `json:"next_cursor,omitempty"`
	Coverage          string          `json:"coverage"`
	TotalCount        *int64          `json:"total_count"`
}

// LabelsView is the label read model of one file.
type LabelsView struct {
	FileID     string              `json:"file_id"`
	Revision   uint64              `json:"revision"`
	Namespaces map[string][]string `json:"namespaces"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// CollectionView is the collection read model.
type CollectionView struct {
	CollectionID string     `json:"collection_id"`
	SpaceID      string     `json:"space_id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	Scope        string     `json:"scope"`
	Rule         string     `json:"rule,omitempty"`
	RuleRevision uint64     `json:"rule_revision,omitempty"`
	Revision     uint64     `json:"revision"`
	ComputedAt   *time.Time `json:"computed_at,omitempty"`
	MembersTotal int64      `json:"members_total"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CollectionReadView is one re-authorized collection read.
type CollectionReadView struct {
	CollectionID string          `json:"collection_id"`
	Kind         string          `json:"kind"`
	Revision     uint64          `json:"revision"`
	ComputedAt   time.Time       `json:"computed_at"`
	Items        []FileQueryItem `json:"items"`
	VisibleCount int64           `json:"visible_count"`
	Coverage     string          `json:"coverage"`
}

// SnapshotView is one frozen read.
type SnapshotView struct {
	SnapshotID         string          `json:"snapshot_id"`
	CollectionID       string          `json:"collection_id"`
	CollectionRevision uint64          `json:"collection_revision"`
	EntriesTotal       int64           `json:"entries_total"`
	EntriesVisible     int64           `json:"entries_visible"`
	CreatedAt          time.Time       `json:"created_at"`
	Items              []FileQueryItem `json:"items"`
}

// SavedQueryView is the saved-query read model.
type SavedQueryView struct {
	QueryID   string          `json:"query_id"`
	Name      string          `json:"name"`
	Query     json.RawMessage `json:"query"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// DuplicateGroup is one duplicate-candidate group.
type DuplicateGroup struct {
	Digest   string          `json:"digest,omitempty"`
	Strength string          `json:"strength"`
	Items    []FileQueryItem `json:"items"`
}

// DuplicateCandidates is the duplicates read.
type DuplicateCandidates struct {
	Groups   []DuplicateGroup `json:"groups"`
	Coverage string           `json:"coverage"`
	Notes    []string         `json:"notes,omitempty"`
}

// QueryFiles runs one exact typed query.
func (c *Client) QueryFiles(ctx context.Context, q FileQueryRequest) (FileQueryView, error) {
	var out FileQueryView
	e := c.JSON(ctx, "POST", "/api/v1/file-queries", "", q, &out)
	return out, e
}

// QueryDuplicateCandidates groups verified-digest duplicates in scope.
func (c *Client) QueryDuplicateCandidates(ctx context.Context, spaceIDs []string) (DuplicateCandidates, error) {
	var out DuplicateCandidates
	e := c.JSON(ctx, "POST", "/api/v1/file-queries", "", map[string]any{"mode": "duplicates", "space_ids": spaceIDs}, &out)
	return out, e
}

// GetFileLabels reads one file's labels across namespaces.
func (c *Client) GetFileLabels(ctx context.Context, fileID string) (LabelsView, error) {
	var out LabelsView
	e := c.JSON(ctx, "GET", "/api/v1/files/"+url.PathEscape(fileID)+"/labels", "", nil, &out)
	return out, e
}

// SetFileLabels replaces one namespace's labels on one file.
func (c *Client) SetFileLabels(ctx context.Context, fileID, namespace string, labels []string, expectedRevision string) (LabelsView, error) {
	var out LabelsView
	body := map[string]any{"namespace": namespace, "labels": labels}
	if expectedRevision != "" {
		body["expected_revision"] = expectedRevision
	}
	e := c.JSON(ctx, "PUT", "/api/v1/files/"+url.PathEscape(fileID)+"/labels", "", body, &out)
	return out, e
}

// ListFileCollections lists the caller's collections in one space.
func (c *Client) ListFileCollections(ctx context.Context, spaceID string) ([]CollectionView, error) {
	var out []CollectionView
	e := c.JSON(ctx, "GET", "/api/v1/file-collections?space_id="+url.QueryEscape(spaceID), "", nil, &out)
	return out, e
}

// CreateFileCollection opens one collection under a durable idempotency
// key.
func (c *Client) CreateFileCollection(ctx context.Context, spaceID, name, kind, scope, rule, key string) (CollectionView, error) {
	var out CollectionView
	body := map[string]any{"space_id": spaceID, "name": name, "kind": kind, "scope": scope}
	if rule != "" {
		body["rule"] = json.RawMessage(rule)
	}
	e := c.JSON(ctx, "POST", "/api/v1/file-collections", key, body, &out)
	return out, e
}

// ReadFileCollection resolves one collection's current membership.
func (c *Client) ReadFileCollection(ctx context.Context, collectionID string) (CollectionReadView, error) {
	var out CollectionReadView
	e := c.JSON(ctx, "GET", "/api/v1/file-collections/"+url.PathEscape(collectionID), "", nil, &out)
	return out, e
}

// SetCollectionMembers replaces the explicit member decisions.
func (c *Client) SetCollectionMembers(ctx context.Context, collectionID string, expectedRevision uint64, include, exclude []string) (CollectionView, error) {
	var out CollectionView
	body := map[string]any{"expected_revision": expectedRevision}
	if len(include) > 0 {
		body["include"] = include
	}
	if len(exclude) > 0 {
		body["exclude"] = exclude
	}
	e := c.JSON(ctx, "PUT", "/api/v1/file-collections/"+url.PathEscape(collectionID)+"/members", "", body, &out)
	return out, e
}

// CreateFileSnapshot freezes one collection read.
func (c *Client) CreateFileSnapshot(ctx context.Context, collectionID string) (SnapshotView, error) {
	var out SnapshotView
	e := c.JSON(ctx, "POST", "/api/v1/file-collections/"+url.PathEscape(collectionID)+"/snapshots", "", nil, &out)
	return out, e
}

// ReadSnapshot reads one frozen snapshot with re-authorization.
func (c *Client) ReadSnapshot(ctx context.Context, snapshotID string) (SnapshotView, error) {
	var out SnapshotView
	e := c.JSON(ctx, "GET", "/api/v1/collection-snapshots/"+url.PathEscape(snapshotID), "", nil, &out)
	return out, e
}

// SavedSearchesList lists the caller's saved queries.
func (c *Client) SavedSearchesList(ctx context.Context) ([]SavedQueryView, error) {
	var out []SavedQueryView
	e := c.JSON(ctx, "GET", "/api/v1/saved-searches", "", nil, &out)
	return out, e
}

// SavedSearchCreate stores one exact query by name.
func (c *Client) SavedSearchCreate(ctx context.Context, name string, q FileQueryRequest) (SavedQueryView, error) {
	var out SavedQueryView
	body := map[string]any{"name": name, "query": q}
	e := c.JSON(ctx, "POST", "/api/v1/saved-searches", "", body, &out)
	return out, e
}

// formatRevision renders a revision for callers building their own bodies.
func formatRevision(v uint64) string { return strconv.FormatUint(v, 10) }
