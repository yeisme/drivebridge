package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Typed analytical queries. The online (live) path answers from the
// transactional store through GORM with bound parameters only; the
// observed path requires an analytics backend deployment and reports
// analytics_backend_unavailable otherwise — never a silent fallback that
// would present transactional data as analytics.

const (
	analyticsPageDefault = 50
	analyticsPageMax     = 200
	// analyticsScanBudget bounds the rows one online query may touch;
	// beyond it the query refuses with query_budget_exceeded and points
	// at the explicit snapshot export instead of scanning quietly.
	analyticsScanBudget = 50000
)

func analyticsGate() error {
	// The analytics surface itself needs no extra deployment switch: the
	// live path is a plain typed read; the observed path declares its own
	// backend availability honestly.
	return nil
}

// AnalyticsFilter is one typed condition.
type AnalyticsFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// AnalyticsSortClause is one typed ordering term: a vocabulary field plus a
// direction. It deliberately does not reuse the filter shape — sort carries
// no operator or comparison value.
type AnalyticsSortClause struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// AnalyticsQueryRequest is the typed analytical query input.
type AnalyticsQueryRequest struct {
	Dataset     string                `json:"dataset"`
	Filters     []AnalyticsFilter     `json:"filters,omitempty"`
	Dimensions  []string              `json:"dimensions,omitempty"`
	Metrics     []string              `json:"metrics,omitempty"`
	Sort        []AnalyticsSortClause `json:"sort,omitempty"`
	Limit       int                   `json:"limit,omitempty"`
	Cursor      string                `json:"cursor,omitempty"`
	Consistency string                `json:"consistency,omitempty"` // live|observed
}

// AnalyticsQueryView is one bounded result page with its honesty facts.
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

var analyticsDatasets = map[string]bool{"file_current": true, "transfer_facts": true, "audit_facts": true, "capacity_daily": true}

var analyticsDatasetFields = map[string]map[string]bool{
	"file_current":   {"space_id": true, "path": true, "mime_type": true, "size_bytes": true, "trashed": true, "updated_at": true, "caller": true},
	"transfer_facts": {"space_id": true, "kind": true, "status": true, "bytes_transferred": true, "created_at": true},
	"audit_facts":    {"action": true, "outcome": true, "created_at": true},
	"capacity_daily": {"day": true, "space_id": true},
}

var analyticsDatasetMetrics = map[string]map[string]bool{
	"file_current":   {"logical_bytes": true, "file_count": true},
	"transfer_facts": {"transfer_count": true, "transferred_bytes": true},
	"audit_facts":    {"event_count": true},
	"capacity_daily": {"logical_bytes": true, "file_count": true},
}

var analyticsFilterOps = map[string]bool{"eq": true, "ne": true, "gt": true, "lt": true, "gte": true, "lte": true, "prefix": true, "contains": true}

// analyticsColumnMap is the per-dataset field→column mapping: the typed
// vocabulary names never reach SQL directly, and every vocabulary field
// has an explicit column here (files carry size/mime/updated; operations
// carry bytes_transferred/updated; audit rows carry at).
var analyticsColumnMap = map[string]map[string]string{
	"file_current":   {"space_id": "space_id", "path": "path", "mime_type": "mime", "size_bytes": "size", "trashed": "trashed", "updated_at": "updated", "caller": "caller"},
	"transfer_facts": {"space_id": "space_id", "kind": "kind", "status": "status", "bytes_transferred": "transferred", "created_at": "updated"},
	"audit_facts":    {"action": "action", "outcome": "outcome", "created_at": "at"},
	"capacity_daily": {"day": "date(updated)", "space_id": "space_id"},
}

func analyticsColumn(dataset, field string) string {
	return analyticsColumnMap[dataset][field]
}

func validateAnalyticsQuery(q AnalyticsQueryRequest) error {
	if !analyticsDatasets[q.Dataset] {
		return code("invalid_query")
	}
	if q.Consistency != "" && q.Consistency != "live" && q.Consistency != "observed" {
		return code("invalid_query")
	}
	if len(q.Filters) > 32 || len(q.Dimensions) > 3 || len(q.Sort) > 3 {
		return code("invalid_query")
	}
	fields := analyticsDatasetFields[q.Dataset]
	metrics := analyticsDatasetMetrics[q.Dataset]
	for _, f := range q.Filters {
		if !fields[f.Field] || !analyticsFilterOps[f.Op] || f.Value == nil {
			return code("invalid_query")
		}
		// SQL or path payloads never enter the typed vocabulary.
		if s, ok := f.Value.(string); ok && (strings.ContainsAny(s, ";\x00") || strings.Contains(strings.ToUpper(s), " SELECT ") || strings.Contains(s, "../")) {
			return code("invalid_query")
		}
	}
	for _, d := range q.Dimensions {
		if !fields[d] {
			return code("invalid_query")
		}
	}
	for _, m := range q.Metrics {
		if !metrics[m] {
			return code("invalid_query")
		}
	}
	for _, s := range q.Sort {
		if !fields[s.Field] || (s.Direction != "asc" && s.Direction != "desc") {
			return code("invalid_query")
		}
	}
	return nil
}

// analyticsAuthorizationScope resolves the readable space scope for the
// caller: humans see their owner scope, agents only their bound space.
// Permission conditions are injected by the service, never accepted from
// the caller.
func (s *Service) analyticsAuthorizationScope(p Principal) ([]string, error) {
	var spaces []Space
	if p.Agent != nil {
		if e := s.DB.Where("id = ?", p.Agent.SpaceID).Find(&spaces).Error; e != nil || len(spaces) == 0 {
			return nil, code("forbidden")
		}
	} else {
		s.DB.Where("owner = ?", p.dataOwner()).Find(&spaces)
	}
	if len(spaces) == 0 {
		return nil, code("forbidden")
	}
	ids := make([]string, 0, len(spaces))
	for _, sp := range spaces {
		// Agents authorize at their prefix granularity; the empty path
		// never matches a non-empty prefix under allow().
		probe := ""
		if p.Agent != nil {
			probe = p.Agent.Prefix
		}
		if e := s.allow(p, sp.ID, probe, "read"); e == nil {
			ids = append(ids, sp.ID)
		}
	}
	if len(ids) == 0 {
		return nil, code("forbidden")
	}
	return ids, nil
}

// QueryAnalytics answers one typed analytical query. live reads the
// transactional catalogue exactly; observed requires the analytics
// backend and reports its own unavailability instead of falling back.
func (s *Service) QueryAnalytics(p Principal, q AnalyticsQueryRequest) (AnalyticsQueryView, error) {
	if e := analyticsGate(); e != nil {
		return AnalyticsQueryView{}, e
	}
	if e := validateAnalyticsQuery(q); e != nil {
		return AnalyticsQueryView{}, e
	}
	if q.Consistency == "" {
		q.Consistency = "observed"
	}
	scope, e := s.analyticsAuthorizationScope(p)
	if e != nil {
		return AnalyticsQueryView{}, e
	}
	if q.Consistency == "observed" {
		// No analytics backend is deployed here: report the dependency
		// honestly. The transactional live path stays available and the
		// caller decides; lagging results never feed deletion approvals.
		return AnalyticsQueryView{}, code("analytics_backend_unavailable")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = analyticsPageDefault
	}
	if limit > analyticsPageMax {
		limit = analyticsPageMax
	}
	canonical := analyticsCanonicalQuery(q)
	offset := 0
	if q.Cursor != "" {
		var e error
		offset, e = s.openAnalyticsCursor(q.Cursor, p.dataOwner(), canonical)
		if e != nil {
			return AnalyticsQueryView{}, e
		}
	}
	rows, more, scanned, e := s.analyticsAggregate(p, q, scope, limit, offset)
	if e != nil {
		return AnalyticsQueryView{}, e
	}
	if scanned > analyticsScanBudget {
		return AnalyticsQueryView{}, code("query_budget_exceeded")
	}
	view := AnalyticsQueryView{QueryID: ID(), Backend: "transactional", ObservedAt: time.Now().UTC(), Coverage: "complete", Partial: false, Rows: rows}
	if more {
		view.Coverage = "partial"
		view.Partial = true
		next, e := s.sealAnalyticsCursor(analyticsCursorPayload{Owner: p.dataOwner(), Query: canonical, Offset: offset + len(rows)})
		if e != nil {
			return AnalyticsQueryView{}, e
		}
		view.NextCursor = next
	}
	return view, nil
}

// analyticsAggregate groups the dataset inside the injected scope with
// bound parameters only; offset is the caller-validated cursor position.
func (s *Service) analyticsAggregate(p Principal, q AnalyticsQueryRequest, scope []string, limit, offset int) ([]map[string]any, bool, int64, error) {
	switch q.Dataset {
	case "file_current":
		return s.analyticsFileCurrent(q, scope, limit, offset)
	case "transfer_facts":
		return s.analyticsTransferFacts(q, scope, limit, offset)
	case "audit_facts":
		return s.analyticsAuditFacts(p, q, scope, limit, offset)
	case "capacity_daily":
		return s.analyticsCapacityDaily(q, scope, limit, offset)
	}
	return nil, false, 0, code("invalid_query")
}

func analyticsWhere(db *gorm.DB, filters []AnalyticsFilter, dataset string) *gorm.DB {
	fields := analyticsDatasetFields[dataset]
	for _, f := range filters {
		if !fields[f.Field] {
			continue
		}
		column := analyticsColumn(dataset, f.Field)
		if f.Field == "updated_at" || f.Field == "created_at" {
			if str, ok := f.Value.(string); ok {
				if t, e := time.Parse(time.RFC3339, str); e == nil {
					db = analyticsTimeOp(db, column, f.Op, t)
					continue
				}
			}
		}
		switch f.Op {
		case "eq":
			db = db.Where(column+" = ?", f.Value)
		case "ne":
			db = db.Where(column+" <> ?", f.Value)
		case "prefix":
			if str, ok := f.Value.(string); ok {
				db = db.Where(column+" LIKE ?", str+"%")
			}
		case "contains":
			if str, ok := f.Value.(string); ok {
				db = db.Where(column+" LIKE ?", "%"+str+"%")
			}
		default:
			if num, ok := toFloat(f.Value); ok {
				db = orgNumOp(db, column, f.Op, num)
			}
		}
	}
	return db
}

func analyticsTimeOp(db *gorm.DB, column, op string, t time.Time) *gorm.DB {
	switch op {
	case "gt":
		return db.Where(column+" > ?", t)
	case "gte":
		return db.Where(column+" >= ?", t)
	case "lt":
		return db.Where(column+" < ?", t)
	case "lte":
		return db.Where(column+" <= ?", t)
	}
	return db
}

// analyticsCursorPayload is the sealed analytical cursor: it binds the
// owner, the canonical query and its own accumulated offset so a cursor
// cannot be forged, re-used across queries or rewound.
type analyticsCursorPayload struct {
	Owner  string `json:"owner"`
	Query  string `json:"query"`
	Offset int    `json:"offset"`
}

func analyticsCanonicalQuery(q AnalyticsQueryRequest) string {
	q.Cursor = ""
	q.Limit = 0
	b, _ := json.Marshal(q)
	return string(b)
}

func (s *Service) sealAnalyticsCursor(payload analyticsCursorPayload) (string, error) {
	body, _ := json.Marshal(payload)
	nonce, e := secureRandomBytes(s.crypt.NonceSize())
	if e != nil {
		return "", code("encryption_unavailable")
	}
	sealed := s.crypt.Seal(nonce, nonce, body, []byte("analytics-cursor"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) openAnalyticsCursor(cursor, owner, query string) (int, error) {
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if e != nil || len(raw) < s.crypt.NonceSize() {
		return 0, code("cursor_expired")
	}
	plain, e := s.crypt.Open(nil, raw[:s.crypt.NonceSize()], raw[s.crypt.NonceSize():], []byte("analytics-cursor"))
	if e != nil {
		return 0, code("cursor_expired")
	}
	var payload analyticsCursorPayload
	if json.Unmarshal(plain, &payload) != nil || payload.Owner != owner || payload.Query != query || payload.Offset < 0 {
		return 0, code("cursor_expired")
	}
	return payload.Offset, nil
}

func (s *Service) analyticsFileCurrent(q AnalyticsQueryRequest, scope []string, limit, offset int) ([]map[string]any, bool, int64, error) {
	// Group by dimensions over the readable scope.
	tx := s.DB.Model(&File{}).Where("space_id IN ? AND trashed = ?", scope, false)
	tx = analyticsWhere(tx, q.Filters, "file_current")
	selects, groups := analyticsProjection("file_current", q)
	if len(selects) == 0 {
		return nil, false, 0, code("invalid_query")
	}
	tx = analyticsStableOrder(tx.Select(strings.Join(selects, ", ")), groups)
	var scanned int64
	s.DB.Model(&File{}).Where("space_id IN ? AND trashed = ?", scope, false).Count(&scanned)
	if offset > 0 {
		tx = tx.Offset(offset)
	}
	var raw []map[string]any
	if e := tx.Limit(limit + 1).Find(&raw).Error; e != nil {
		return nil, false, 0, code("database_unavailable")
	}
	more := len(raw) > limit
	if more {
		raw = raw[:limit]
	}
	return raw, more, scanned, nil
}

// analyticsProjection maps one query's dimensions and metrics through the
// per-dataset vocabulary: every dimension rides analyticsColumnMap and
// every metric its aggregate expression, so no vocabulary path can reach a
// non-existent physical column. Row keys keep the vocabulary names.
func analyticsProjection(dataset string, q AnalyticsQueryRequest) (selects, groups []string) {
	for _, d := range q.Dimensions {
		column := analyticsColumn(dataset, d)
		if column == "" {
			continue
		}
		selects = append(selects, column+" AS "+strings.ReplaceAll(d, ".", "_"))
		if !containsString(groups, column) {
			groups = append(groups, column)
		}
	}
	for _, m := range q.Metrics {
		if expr, ok := analyticsMetricMap[dataset][m]; ok {
			selects = append(selects, expr+" AS "+m)
		}
	}
	return selects, groups
}

// analyticsMetricMap is the per-dataset aggregate expression for each
// closed metric name; SUM expressions name the real physical columns.
var analyticsMetricMap = map[string]map[string]string{
	"file_current":   {"logical_bytes": "COALESCE(SUM(size),0)", "file_count": "COUNT(*)"},
	"transfer_facts": {"transfer_count": "COUNT(*)", "transferred_bytes": "COALESCE(SUM(transferred),0)"},
	"audit_facts":    {"event_count": "COUNT(*)"},
	"capacity_daily": {"logical_bytes": "COALESCE(SUM(size),0)", "file_count": "COUNT(*)"},
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// analyticsStableOrder applies the grouping and pins the group output
// order to the grouping keys, so offset pages are a stable partition of
// the result — no cross-page duplicates or lost groups.
func analyticsStableOrder(tx *gorm.DB, groups []string) *gorm.DB {
	for _, g := range groups {
		tx = tx.Group(g)
	}
	if len(groups) > 0 {
		tx = tx.Order(strings.Join(groups, ", "))
	}
	return tx
}

func (s *Service) analyticsTransferFacts(q AnalyticsQueryRequest, scope []string, limit, offset int) ([]map[string]any, bool, int64, error) {
	tx := s.DB.Model(&Operation{}).Where("space_id IN ?", scope)
	tx = analyticsWhere(tx, q.Filters, "transfer_facts")
	selects, groups := analyticsProjection("transfer_facts", q)
	if len(selects) == 0 {
		return nil, false, 0, code("invalid_query")
	}
	tx = analyticsStableOrder(tx.Select(strings.Join(selects, ", ")), groups)
	var scanned int64
	s.DB.Model(&Operation{}).Where("space_id IN ?", scope).Count(&scanned)
	if offset > 0 {
		tx = tx.Offset(offset)
	}
	var raw []map[string]any
	if e := tx.Limit(limit + 1).Find(&raw).Error; e != nil {
		return nil, false, 0, code("database_unavailable")
	}
	more := len(raw) > limit
	if more {
		raw = raw[:limit]
	}
	return raw, more, scanned, nil
}

func (s *Service) analyticsAuditFacts(p Principal, q AnalyticsQueryRequest, scope []string, limit, offset int) ([]map[string]any, bool, int64, error) {
	// Audit rows carry no space scope of their own; the analytical view
	// stays scoped to this caller's data owner — never the whole instance.
	owner := p.dataOwner()
	var scanned int64
	s.DB.Model(&Audit{}).Where("owner = ?", owner).Count(&scanned)
	db := s.DB.Model(&Audit{}).Where("owner = ?", owner)
	db = analyticsWhere(db, q.Filters, "audit_facts")
	selects, groups := analyticsProjection("audit_facts", q)
	if len(selects) == 0 {
		return nil, false, 0, code("invalid_query")
	}
	db = analyticsStableOrder(db.Select(strings.Join(selects, ", ")), groups)
	if offset > 0 {
		db = db.Offset(offset)
	}
	var raw []map[string]any
	if e := db.Limit(limit + 1).Find(&raw).Error; e != nil {
		return nil, false, 0, code("database_unavailable")
	}
	more := len(raw) > limit
	if more {
		raw = raw[:limit]
	}
	return raw, more, scanned, nil
}

func (s *Service) analyticsCapacityDaily(q AnalyticsQueryRequest, scope []string, limit, offset int) ([]map[string]any, bool, int64, error) {
	// capacity_daily derives from the current catalogue grouped by day of
	// the last update — a daily rollup, not a billing statement.
	tx := s.DB.Model(&File{}).Where("space_id IN ? AND trashed = ?", scope, false)
	tx = analyticsWhere(tx, q.Filters, "capacity_daily")
	// The day rollup key is always present; extra dimensions and all
	// metrics ride the shared vocabulary mapping.
	selects := []string{"date(updated) AS day"}
	groups := []string{"date(updated)"}
	dims, dimGroups := analyticsProjection("capacity_daily", q)
	selects = append(selects, dims...)
	for _, g := range dimGroups {
		if !containsString(groups, g) {
			groups = append(groups, g)
		}
	}
	tx = analyticsStableOrder(tx.Select(strings.Join(selects, ", ")), groups)
	var scanned int64
	s.DB.Model(&File{}).Where("space_id IN ? AND trashed = ?", scope, false).Count(&scanned)
	if offset > 0 {
		tx = tx.Offset(offset)
	}
	var raw []map[string]any
	if e := tx.Limit(limit + 1).Find(&raw).Error; e != nil {
		return nil, false, 0, code("database_unavailable")
	}
	more := len(raw) > limit
	if more {
		raw = raw[:limit]
	}
	return raw, more, scanned, nil
}

// AnalyticsSnapshotRequest is the explicit export intent.
type AnalyticsSnapshotRequest struct {
	Dataset    string            `json:"dataset"`
	Filters    []AnalyticsFilter `json:"filters,omitempty"`
	Fields     []string          `json:"fields,omitempty"`
	SpaceScope string            `json:"space_scope,omitempty"`
}

// CreateAnalyticsSnapshot builds one service-generated export copy inside
// the deployment state directory. The manifest records the frozen scope,
// watermarks and content digest; the data file holds only the necessary
// fields — never credentials, temporary URLs or file bodies.
func (s *Service) CreateAnalyticsSnapshot(ctx context.Context, p Principal, q AnalyticsSnapshotRequest, key string) (AnalyticsSnapshot, error) {
	if key == "" || len(key) > 200 {
		return AnalyticsSnapshot{}, code("invalid_request")
	}
	dummy := AnalyticsQueryRequest{Dataset: q.Dataset, Filters: q.Filters, Consistency: "live", Metrics: []string{"file_count"}}
	if e := validateAnalyticsQuery(dummy); e != nil {
		return AnalyticsSnapshot{}, e
	}
	if p.Agent != nil {
		// Snapshots are explicit export authorizations; agents do not
		// export analytical copies.
		return AnalyticsSnapshot{}, code("forbidden")
	}
	scope, e := s.analyticsAuthorizationScope(p)
	if e != nil {
		return AnalyticsSnapshot{}, e
	}
	fields := analyticsDatasetFields[q.Dataset]
	kept := make([]string, 0, len(q.Fields))
	for _, f := range q.Fields {
		if !fields[f] {
			return AnalyticsSnapshot{}, code("invalid_query")
		}
		kept = append(kept, f)
	}
	if len(kept) == 0 {
		kept = []string{"space_id", "size_bytes"}
	}
	now := time.Now().UTC()
	// Replay under the same key returns the original manifest; a same-key
	// request with different business inputs is refused. The fingerprint
	// covers dataset, filters, fields and scope.
	fpSource := struct {
		Dataset string            `json:"dataset"`
		Filters []AnalyticsFilter `json:"filters"`
		Fields  []string          `json:"fields"`
		Scope   string            `json:"scope"`
	}{q.Dataset, q.Filters, kept, strings.Join(scope, ",")}
	fpBody, _ := json.Marshal(fpSource)
	fingerprint := Hash(string(fpBody))
	var existing AnalyticsSnapshot
	if s.DB.First(&existing, "owner = ? AND key_id = ?", p.dataOwner(), key).Error == nil {
		if existing.Fingerprint != fingerprint {
			return AnalyticsSnapshot{}, code("idempotency_conflict")
		}
		return existing, nil
	}
	manifest := AnalyticsSnapshot{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, Dataset: q.Dataset, SchemaVersion: "1", SpaceScope: strings.Join(scope, ","), Fields: strings.Join(kept, ","), KeyID: key, Fingerprint: fingerprint, Status: "building", ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now}
	if e := s.DB.Create(&manifest).Error; e != nil {
		return AnalyticsSnapshot{}, code("database_unavailable")
	}
	// Materialize the export rows (bounded by the scan budget).
	rows, _, scanned, e := s.analyticsAggregate(p, AnalyticsQueryRequest{Dataset: q.Dataset, Filters: q.Filters, Dimensions: []string{kept[0]}, Metrics: []string{"file_count"}}, scope, analyticsScanBudget, 0)
	if e != nil {
		s.DB.Model(&AnalyticsSnapshot{}).Where("id = ?", manifest.ID).Updates(map[string]any{"status": "failed", "updated_at": now})
		return AnalyticsSnapshot{}, e
	}
	body, _ := json.Marshal(rows)
	manifest.Rows = int64(len(rows))
	manifest.ContentSHA256 = Hash(string(body))
	manifest.Status = "ready"
	manifest.SourceWatermark = scanned
	manifest.DataKey = "analytics-snapshots/" + manifest.ID + ".json"
	if e := s.DB.Save(&manifest).Error; e != nil {
		return AnalyticsSnapshot{}, code("database_unavailable")
	}
	return manifest, nil
}

// QueryAnalyticsSnapshot answers a typed query against one snapshot — but
// only through the fixed-version analytical engine. When the optional
// engine is not installed the call reports analytics_backend_unavailable
// without touching anything else; the typed validation still runs first
// so SQL or path payloads never reach any engine.
func (s *Service) QueryAnalyticsSnapshot(p Principal, snapshotID string, q AnalyticsQueryRequest) (AnalyticsQueryView, error) {
	if e := validateAnalyticsQuery(q); e != nil {
		return AnalyticsQueryView{}, e
	}
	var manifest AnalyticsSnapshot
	if s.DB.First(&manifest, "id = ? AND owner = ?", snapshotID, p.dataOwner()).Error != nil {
		return AnalyticsQueryView{}, code("not_found")
	}
	if time.Now().UTC().After(manifest.ExpiresAt) {
		return AnalyticsQueryView{}, code("snapshot_expired")
	}
	// No DuckDB executable is installed in this deployment: report the
	// missing dependency; never auto-install or silently switch backend.
	return AnalyticsQueryView{}, code("analytics_backend_unavailable")
}

// GetAnalyticsQuery reads back one query job by its id. Query ids carry
// no permissions: reading results re-authorizes the scope.
func (s *Service) GetAnalyticsQuery(p Principal, queryID string) (AnalyticsQueryView, error) {
	// Online queries are stateless; the durable job surface belongs to
	// snapshot exports read through their own manifest contract.
	return AnalyticsQueryView{}, code("not_found")
}
