// Cloud connector SDK surface. Every method maps one application-service
// operation over the shared HTTP contract; field names match the design
// contract exactly because remote client mocks bind to them.
package sdk

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// CloudCapabilities is the per-provider capability matrix: independently
// declared facts, never inferred from the provider's name.
type CloudCapabilities struct {
	Provider         string   `json:"provider"`
	Browse           bool     `json:"browse"`
	Transfer         bool     `json:"transfer"`
	HistoricalRead   bool     `json:"historical_read"`
	NativeCopy       bool     `json:"native_copy"`
	SoftDelete       bool     `json:"soft_delete"`
	ConditionalWrite bool     `json:"conditional_write"`
	ChangeCursor     bool     `json:"change_cursor"`
	Notes            []string `json:"notes,omitempty"`
}

// CloudRoot is one bound external scope inside a connection.
type CloudRoot struct {
	RootID             string     `json:"root_id"`
	ConnectionID       string     `json:"connection_id"`
	Provider           string     `json:"provider"`
	RootRef            string     `json:"root_ref"`
	Name               string     `json:"name,omitempty"`
	Status             string     `json:"status"`
	Generation         uint64     `json:"generation"`
	LastScanAt         *time.Time `json:"last_scan_at,omitempty"`
	LastScanCoverage   string     `json:"last_scan_coverage,omitempty"`
	LastScanObserved   int64      `json:"last_scan_observed,omitempty"`
	LastScanTombstoned int64      `json:"last_scan_tombstoned,omitempty"`
}

// CloudEntry is one external observation. VersionStrength observed_only
// means provider revision fact; sha256_verified appears only after an
// explicit verified import of that exact revision.
type CloudEntry struct {
	EntryRef         string     `json:"entry_ref"`
	ProviderItemID   string     `json:"provider_item_id"`
	RootID           string     `json:"root_id"`
	ParentRef        string     `json:"parent_ref,omitempty"`
	Path             string     `json:"path"`
	Kind             string     `json:"kind"`
	SizeBytes        int64      `json:"size_bytes"`
	MIME             string     `json:"mime_type,omitempty"`
	ModifiedAt       *time.Time `json:"modified_at,omitempty"`
	ObservedRevision string     `json:"observed_revision"`
	VersionStrength  string     `json:"version_strength"`
	ContentSHA256    string     `json:"content_sha256,omitempty"`
	ManagedRef       string     `json:"managed_ref,omitempty"`
	Tombstoned       bool       `json:"tombstoned,omitempty"`
	ObservedAt       time.Time  `json:"observed_at"`
}

// CloudEntries is one bounded observation page.
type CloudEntries struct {
	Items      []CloudEntry `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
	Coverage   string       `json:"coverage"`
	ObservedAt time.Time    `json:"observed_at"`
}

// CloudChanges is one bounded provider delta page.
type CloudChanges struct {
	Items      []CloudEntry `json:"items"`
	Removed    []string     `json:"removed,omitempty"`
	NextCursor string       `json:"next_cursor,omitempty"`
	Complete   bool         `json:"complete"`
	ObservedAt time.Time    `json:"observed_at"`
}

// CloudScan is the scan outcome on the original Operation.
type CloudScan struct {
	OperationID string    `json:"operation_id"`
	RootID      string    `json:"root_id"`
	Status      string    `json:"status"`
	Coverage    string    `json:"coverage"`
	Observed    int64     `json:"observed"`
	Tombstoned  int64     `json:"tombstoned"`
	Generation  uint64    `json:"generation"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListCloudEntriesRequest is the listing input: root_id, optional
// parent_ref, opaque cursor and limit<=200.
type ListCloudEntriesRequest struct {
	RootID    string `json:"root_id"`
	ParentRef string `json:"parent_ref,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// ImportCloudEntryRequest is the explicit import intent.
type ImportCloudEntryRequest struct {
	EntryRef         string `json:"entry_ref"`
	ExpectedRevision string `json:"expected_revision,omitempty"`
	SpaceID          string `json:"space_id"`
	Path             string `json:"path"`
}

// CloudCapabilities reads the per-connection provider matrix.
func (c *Client) CloudCapabilities(ctx context.Context, connectionID string) (CloudCapabilities, error) {
	var out CloudCapabilities
	e := c.JSON(ctx, "GET", "/api/v1/connections/"+url.PathEscape(connectionID)+"/cloud-capabilities", "", nil, &out)
	return out, e
}

// BindCloudRoot binds one provider-native root to the connection (human
// sessions only; the server refuses agent tokens).
func (c *Client) BindCloudRoot(ctx context.Context, connectionID, rootRef, name string) (CloudRoot, error) {
	var out CloudRoot
	body := map[string]string{"root_ref": rootRef, "name": name}
	e := c.JSON(ctx, "POST", "/api/v1/connections/"+url.PathEscape(connectionID)+"/roots", "", body, &out)
	return out, e
}

// ListCloudRoots lists the connection's bound roots.
func (c *Client) ListCloudRoots(ctx context.Context, connectionID string) ([]CloudRoot, error) {
	var out []CloudRoot
	e := c.JSON(ctx, "GET", "/api/v1/connections/"+url.PathEscape(connectionID)+"/roots", "", nil, &out)
	return out, e
}

// ListCloudEntries returns one bounded observation page under the root.
func (c *Client) ListCloudEntries(ctx context.Context, connectionID string, q ListCloudEntriesRequest) (CloudEntries, error) {
	var out CloudEntries
	values := url.Values{}
	values.Set("root_id", q.RootID)
	if q.ParentRef != "" {
		values.Set("parent_ref", q.ParentRef)
	}
	if q.Cursor != "" {
		values.Set("cursor", q.Cursor)
	}
	if q.Limit > 0 {
		values.Set("limit", strconv.Itoa(q.Limit))
	}
	e := c.JSON(ctx, "GET", "/api/v1/connections/"+url.PathEscape(connectionID)+"/entries?"+values.Encode(), "", nil, &out)
	return out, e
}

// StatCloudEntry checks one entry against the live provider; a mismatched
// expected_revision returns source_changed, never a silent refresh.
func (c *Client) StatCloudEntry(ctx context.Context, connectionID, entryRef, expectedRevision string) (CloudEntry, error) {
	var out CloudEntry
	body := map[string]string{"entry_ref": entryRef, "expected_revision": expectedRevision}
	e := c.JSON(ctx, "POST", "/api/v1/connections/"+url.PathEscape(connectionID)+"/entries/stat", "", body, &out)
	return out, e
}

// ReadCloudChanges advances the root's provider change feed.
func (c *Client) ReadCloudChanges(ctx context.Context, connectionID, rootID, cursor string, limit int) (CloudChanges, error) {
	var out CloudChanges
	values := url.Values{}
	values.Set("root_id", rootID)
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	e := c.JSON(ctx, "GET", "/api/v1/connections/"+url.PathEscape(connectionID)+"/changes?"+values.Encode(), "", nil, &out)
	return out, e
}

// ScanCloudConnection runs one budgeted scan under a durable idempotency
// key and returns the original Operation-scoped facts.
func (c *Client) ScanCloudConnection(ctx context.Context, connectionID, rootID string, budget int64, key string) (CloudScan, error) {
	var out CloudScan
	body := map[string]any{"root_id": rootID, "budget": budget}
	e := c.JSON(ctx, "POST", "/api/v1/connections/"+url.PathEscape(connectionID)+"/scan", key, body, &out)
	return out, e
}

// ImportCloudEntry downloads the pinned observation, verifies the digest
// and publishes a managed copy linked to the external source.
func (c *Client) ImportCloudEntry(ctx context.Context, connectionID string, q ImportCloudEntryRequest, key string) (Operation, error) {
	var out Operation
	e := c.JSON(ctx, "POST", "/api/v1/connections/"+url.PathEscape(connectionID)+"/entries/import", key, q, &out)
	return out, e
}
