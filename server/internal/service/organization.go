package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Virtual file organization: typed queries, label authority, collections
// and pinned snapshots. Queries never accept arbitrary SQL: the filter
// vocabulary is closed and validated field by field.

const (
	orgPageDefault = 50
	orgPageMax     = 200
)

func orgPageLimit(raw int) int {
	if raw <= 0 {
		return orgPageDefault
	}
	if raw > orgPageMax {
		return orgPageMax
	}
	return raw
}

// FileQueryFilter is one typed condition. Field names are the closed
// vocabulary; anything else is invalid_query.
type FileQueryFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// FileQuerySort is one sort term; stable cursors add the id tiebreaker.
type FileQuerySort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // asc|desc
}

// FileQueryRequest is the exact query input.
type FileQueryRequest struct {
	Mode     string            `json:"mode"` // exact|duplicates
	SpaceIDs []string          `json:"space_ids"`
	Filters  []FileQueryFilter `json:"filters,omitempty"`
	Sort     []FileQuerySort   `json:"sort,omitempty"`
	Limit    int               `json:"limit,omitempty"`
	Cursor   string            `json:"cursor,omitempty"`
}

// FileQueryItem is one result row: the file identity and the version the
// sort pinned for this enumeration.
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

// orgCursorPayload is the sealed cursor body: it binds the owner, the
// canonical query and the catalogue revision it was minted under.
type orgCursorPayload struct {
	Owner             string `json:"owner"`
	Query             string `json:"query"`
	CatalogueRevision string `json:"catalogue_revision"`
	LastKey           string `json:"last_key"`
	Kind              string `json:"kind"`
}

func (s *Service) sealOrgCursor(payload orgCursorPayload) (string, error) {
	body, e := json.Marshal(payload)
	if e != nil {
		return "", code("invalid_request")
	}
	nonce := make([]byte, s.crypt.NonceSize())
	rand.Read(nonce)
	sealed := s.crypt.Seal(nonce, nonce, body, []byte("org-cursor"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) openOrgCursor(cursor, owner, query, catalogueRevision string) (orgCursorPayload, error) {
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if e != nil || len(raw) < s.crypt.NonceSize() {
		return orgCursorPayload{}, code("query_changed")
	}
	plain, e := s.crypt.Open(nil, raw[:s.crypt.NonceSize()], raw[s.crypt.NonceSize():], []byte("org-cursor"))
	if e != nil {
		return orgCursorPayload{}, code("query_changed")
	}
	var payload orgCursorPayload
	if json.Unmarshal(plain, &payload) != nil {
		return orgCursorPayload{}, code("query_changed")
	}
	if payload.Owner != owner || payload.Query != query {
		return orgCursorPayload{}, code("query_changed")
	}
	if payload.CatalogueRevision != catalogueRevision {
		return orgCursorPayload{}, code("query_changed")
	}
	return payload, nil
}

// canonicalQueryJSON is the fingerprint basis: the same business inputs
// always produce the same bytes.
func canonicalQueryJSON(q FileQueryRequest) string {
	q.Cursor = ""
	b, _ := json.Marshal(q)
	return string(b)
}

var orgFilterFields = map[string]bool{
	"mime_type": true, "path_prefix": true, "size_bytes": true,
	"updated_at": true, "caller": true, "task_ref": true,
	"trashed": true, "labels.manual": true, "labels.rule": true, "labels.suggested": true,
}

var orgFilterOps = map[string]bool{"eq": true, "ne": true, "contains": true, "prefix": true, "gt": true, "lt": true, "gte": true, "lte": true, "in": true}

func validateFileQuery(q FileQueryRequest) error {
	if q.Mode != "exact" && q.Mode != "duplicates" {
		return code("invalid_query")
	}
	if len(q.SpaceIDs) == 0 {
		return code("invalid_query")
	}
	for _, f := range q.Filters {
		if !orgFilterFields[f.Field] || !orgFilterOps[f.Op] || f.Value == nil {
			return code("invalid_query")
		}
		if f.Field == "path_prefix" {
			value, ok := f.Value.(string)
			if !ok || (f.Op != "prefix" && f.Op != "contains") || len(value) > 4096 || strings.ContainsRune(value, '\x00') || (f.Op == "contains" && value == "") {
				return code("invalid_query")
			}
		}
	}
	for _, s := range q.Sort {
		if !orgFilterFields[s.Field] && s.Field != "name" {
			return code("invalid_query")
		}
		if s.Direction != "asc" && s.Direction != "desc" {
			return code("invalid_query")
		}
	}
	return nil
}

// catalogueRevision reports the observable metadata generation: the newest
// file mutation timestamp in the owner scope. It binds cursors so a page
// sequence never silently crosses a catalogue change.
func (s *Service) catalogueRevision(owner string) (string, error) {
	var latest time.Time
	var files []File
	if err := s.DB.Select("updated").Where("owner = ?", owner).Order("updated DESC").Limit(1).Find(&files).Error; err != nil {
		return "", code("database_unavailable")
	}
	if len(files) == 1 {
		latest = files[0].Updated
	}
	var labels []FileLabelAssignment
	if err := s.DB.Select("updated_at").Where("owner = ?", owner).Order("updated_at DESC").Limit(1).Find(&labels).Error; err != nil {
		// Additive-schema rollback retains legacy file queries after removing
		// the label table. Only the concrete missing-relation error qualifies;
		// connectivity, permissions and other query failures remain errors.
		var postgres interface{ SQLState() string }
		missing := s.DB.Dialector.Name() == "sqlite" && strings.Contains(err.Error(), "no such table: file_label_assignments")
		missing = missing || (s.DB.Dialector.Name() == "postgres" && errors.As(err, &postgres) && postgres.SQLState() == "42P01")
		if !missing {
			return "", code("database_unavailable")
		}
		return "labels-unavailable:" + latest.UTC().Format(time.RFC3339Nano), nil
	}
	if len(labels) == 1 && labels[0].UpdatedAt.After(latest) {
		latest = labels[0].UpdatedAt
	}
	return latest.UTC().Format(time.RFC3339Nano), nil
}

// QueryFiles runs one exact typed query. The SQL is assembled from the
// closed vocabulary with bound parameters only; keyset pagination keeps
// pages stable and the count is only reported when the enumeration reached
// its end inside the authorized scope.
func (s *Service) QueryFiles(p Principal, q FileQueryRequest) (FileQueryView, error) {
	if e := validateFileQuery(q); e != nil {
		return FileQueryView{}, e
	}
	limit := orgPageLimit(q.Limit)
	revision, err := s.catalogueRevision(p.dataOwner())
	if err != nil {
		return FileQueryView{}, err
	}
	canonical := canonicalQueryJSON(q)
	lastKey := ""
	if q.Cursor != "" {
		payload, e := s.openOrgCursor(q.Cursor, p.dataOwner(), canonical, revision)
		if e != nil {
			return FileQueryView{}, e
		}
		lastKey = payload.LastKey
	}
	tx := s.DB.Model(&File{}).Where("owner = ? AND space_id IN ?", p.dataOwner(), q.SpaceIDs)
	// Agent boundaries contract the query itself: only the bound space and
	// prefix are ever candidates, and a revoked or expired grant refuses
	// the whole read rather than leaking per-row.
	var countTx *gorm.DB
	if p.Agent != nil {
		var current Agent
		if s.DB.First(&current, "id = ? AND owner = ?", p.Agent.ID, p.dataOwner()).Error != nil || current.Revoked || !time.Now().Before(current.Expires) || !strings.Contains(","+current.Scopes+",", ",read,") {
			return FileQueryView{}, code("forbidden")
		}
		tx = orgAgentScope(tx, current)
		countTx = orgAgentScope(s.DB.Model(&File{}), current)
	}
	tx = orgApplyFilters(s, tx, q.Filters, p.dataOwner())
	// Trashed files stay invisible unless explicitly requested.
	explicitTrash := false
	for _, f := range q.Filters {
		if f.Field == "trashed" {
			explicitTrash = true
		}
	}
	if !explicitTrash {
		tx = tx.Where("trashed = ?", false)
	}
	sortField, sortDir := "updated", "desc"
	if len(q.Sort) > 0 {
		sortField, sortDir = orgSortColumn(q.Sort[0])
	}
	if lastKey != "" {
		// Keyset continuation: strictly past the last (value, id) pair in
		// the ORDER BY direction — the id tiebreak follows the sort
		// direction too, or equal-value runs lose rows between pages.
		value, id := splitLastKey(lastKey)
		valueOp, idOp := ">", ">"
		if sortDir == "desc" {
			valueOp, idOp = "<", "<"
		}
		if sortField == "size" {
			// Old size cursors incorrectly contained timestamps. Reject them
			// explicitly rather than repeating or silently dropping rows.
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil || size < 0 || id == "" || strconv.FormatInt(size, 10) != value {
				return FileQueryView{}, code("query_changed")
			}
			tx = tx.Where("(size "+valueOp+" ? OR (size = ? AND id "+idOp+" ?))", size, size, id)
		} else if value != "" {
			if sortField == "updated" {
				if valueTime, e := time.Parse(time.RFC3339Nano, value); e == nil {
					// Bound the ordered index scan before applying the equal-time ID tie.
					tx = tx.Where(sortField+" "+valueOp+"= ?", valueTime)
					tx = tx.Where("("+sortField+" "+valueOp+" ? OR ("+sortField+" = ? AND id "+idOp+" ?))", valueTime, valueTime, id)
				} else {
					tx = tx.Where("("+sortField+" "+valueOp+" ? OR ("+sortField+" = ? AND id "+idOp+" ?))", value, value, id)
				}
			} else {
				tx = tx.Where("("+sortField+" "+valueOp+" ? OR ("+sortField+" = ? AND id "+idOp+" ?))", value, value, id)
			}
		}
	}
	var files []File
	if e := tx.Order(sortField + " " + sortDir + ", id " + sortDir).Limit(limit + 1).Find(&files).Error; e != nil {
		return FileQueryView{}, code("database_unavailable")
	}
	view := FileQueryView{Mode: q.Mode, Items: []FileQueryItem{}, CatalogueRevision: revision, Coverage: "complete"}
	more := len(files) > limit
	if more {
		files = files[:limit]
		view.Coverage = "partial"
	}
	var total *int64
	if !more {
		var count int64
		c := s.DB.Model(&File{}).Where("owner = ? AND space_id IN ?", p.dataOwner(), q.SpaceIDs)
		if countTx != nil {
			c = countTx
		}
		c = orgApplyFilters(s, c, q.Filters, p.dataOwner())
		if !explicitTrash {
			c = c.Where("trashed = ?", false)
		}
		if e := c.Count(&count).Error; e == nil {
			total = &count
		}
	}
	view.TotalCount = total
	for _, f := range files {
		// Every page re-authorizes every row: a grant narrowed between
		// pages never leaks an item it no longer covers.
		if e := s.allow(p, f.SpaceID, f.Path, "read"); e != nil {
			if !workspaceVisibilityDenied(e) {
				return FileQueryView{}, e
			}
			continue
		}
		item := FileQueryItem{FileID: f.ID, VersionID: f.VersionID, Path: f.Path}
		view.Items = append(view.Items, item)
	}
	if more && len(files) > 0 {
		last := files[len(files)-1]
		// The cursor key carries the sort column's own value: timestamps
		// for updated, paths for the path-derived sorts, integers for size.
		key := last.Updated.UTC().Format(time.RFC3339Nano) + "|" + last.ID
		if sortField == "path" {
			key = last.Path + "|" + last.ID
		} else if sortField == "size" {
			key = strconv.FormatInt(last.Size, 10) + "|" + last.ID
		}
		cursor, e := s.sealOrgCursor(orgCursorPayload{Owner: p.dataOwner(), Query: canonical, CatalogueRevision: revision, LastKey: key, Kind: "files_query"})
		if e != nil {
			return FileQueryView{}, e
		}
		view.NextCursor = cursor
	}
	return view, nil
}

// orgAgentScope contracts one files query to the agent's bound space and
// prefix, so the query boundary itself cannot produce candidates outside
// the grant. Keyset pagination stays exact because the contraction is
// constant for the whole query.
func orgAgentScope(tx *gorm.DB, a Agent) *gorm.DB {
	tx = tx.Where("space_id = ?", a.SpaceID).
		Where("path <> ? AND path NOT LIKE ? AND path NOT LIKE ? AND path NOT LIKE ?", ".inbox-staging", ".inbox-staging/%", "%/.inbox-staging/%", "%/.inbox-staging")
	if a.Prefix != "" {
		tx = tx.Where("(path = ? OR path LIKE ?)", a.Prefix, a.Prefix+"/%")
	}
	return tx
}

func splitLastKey(key string) (string, string) {
	idx := strings.LastIndex(key, "|")
	if idx < 0 {
		return key, ""
	}
	return key[:idx], key[idx+1:]
}

func orgSortColumn(s FileQuerySort) (string, string) {
	dir := s.Direction
	switch s.Field {
	case "size_bytes":
		return "size", dir
	case "path_prefix":
		return "path", dir
	case "name":
		// "name" sorts by the path column: files carry no separate name
		// column and the path's base is the displayed name.
		return "path", dir
	default:
		return "updated", dir
	}
}

// orgApplyFilters translates the typed vocabulary into bound parameters.
// Label filters resolve through the assignment table with a scoped IN
// subquery; no string from the caller is ever interpolated as SQL.
func orgApplyFilters(s *Service, tx *gorm.DB, filters []FileQueryFilter, owner string) *gorm.DB {
	for _, f := range filters {
		switch f.Field {
		case "trashed":
			if b, ok := f.Value.(bool); ok && f.Op == "eq" {
				tx = tx.Where("trashed = ?", b)
			}
		case "labels.manual", "labels.rule", "labels.suggested":
			ns := strings.TrimPrefix(f.Field, "labels.")
			label, _ := f.Value.(string)
			sub := s.DB.Model(&FileLabelAssignment{}).Select("file_id").Where("owner = ? AND namespace = ? AND label = ?", owner, ns, label)
			switch f.Op {
			case "eq", "contains":
				tx = tx.Where("id IN (?)", sub)
			case "ne":
				tx = tx.Where("id NOT IN (?)", sub)
			}
		case "updated_at":
			if str, ok := f.Value.(string); ok {
				if t, e := time.Parse(time.RFC3339, str); e == nil {
					tx = orgTimeOp(tx, "updated", f.Op, t)
				}
			}
		case "size_bytes":
			if num, ok := toFloat(f.Value); ok {
				tx = orgNumOp(tx, "size", f.Op, num)
			}
		case "mime_type":
			if str, ok := f.Value.(string); ok && f.Op == "eq" {
				// The File column is derived from the MIME field name.
				tx = tx.Where("mime = ?", str)
			}
		case "path_prefix":
			str, _ := f.Value.(string)
			// Escape the escape character first. The pattern is always bound;
			// user '%' and '_' are literal path characters, never wildcards.
			escape := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
			if f.Op == "prefix" {
				tx = tx.Where("path LIKE ? ESCAPE '!'", escape.Replace(strings.TrimSuffix(str, "/"))+"/%")
			} else if f.Op == "contains" {
				tx = tx.Where("path LIKE ? ESCAPE '!'", "%"+escape.Replace(str)+"%")
			}
		case "caller":
			if str, ok := f.Value.(string); ok && f.Op == "eq" {
				tx = tx.Where("caller = ?", str)
			}
		case "task_ref":
			if str, ok := f.Value.(string); ok && f.Op == "eq" {
				tx = tx.Where("task_ref = ?", str)
			}
		}
	}
	return tx
}

func orgTimeOp(tx *gorm.DB, field, op string, t time.Time) *gorm.DB {
	switch op {
	case "gt":
		return tx.Where(field+" > ?", t)
	case "gte":
		return tx.Where(field+" >= ?", t)
	case "lt":
		return tx.Where(field+" < ?", t)
	case "lte":
		return tx.Where(field+" <= ?", t)
	}
	return tx
}

func orgNumOp(tx *gorm.DB, field, op string, v float64) *gorm.DB {
	switch op {
	case "gt":
		return tx.Where(field+" > ?", v)
	case "gte":
		return tx.Where(field+" >= ?", v)
	case "lt":
		return tx.Where(field+" < ?", v)
	case "lte":
		return tx.Where(field+" <= ?", v)
	}
	return tx
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// SetFileLabelsRequest replaces one namespace's label set on one file.
type SetFileLabelsRequest struct {
	Namespace        string   `json:"namespace"`
	Labels           []string `json:"labels"`
	ExpectedRevision string   `json:"expected_revision,omitempty"`
}

// LabelAssignmentView is the read model of one file's labels.
type LabelAssignmentView struct {
	FileID     string              `json:"file_id"`
	Revision   uint64              `json:"revision"`
	Namespaces map[string][]string `json:"namespaces"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// labelRevision derives the observable revision from the newest assignment
// timestamp: a monotonically moving fact callers can pin expectations to.
func labelRevision(assignments []FileLabelAssignment) uint64 {
	var latest time.Time
	for _, a := range assignments {
		if a.UpdatedAt.After(latest) {
			latest = a.UpdatedAt
		}
	}
	return uint64(latest.UnixNano())
}

// SetFileLabels writes one namespace's labels. Manual labels only move on
// explicit human action: an agent writing manual or rule gets
// label_namespace_forbidden and nothing changes. A stale expected_revision
// is version_conflict.
func (s *Service) SetFileLabels(p Principal, fileID string, q SetFileLabelsRequest) (LabelAssignmentView, error) {
	if q.Namespace != "manual" && q.Namespace != "rule" && q.Namespace != "suggested" {
		return LabelAssignmentView{}, code("invalid_query")
	}
	if len(q.Labels) > 100 {
		return LabelAssignmentView{}, code("invalid_request")
	}
	for _, l := range q.Labels {
		if l == "" || len(l) > 200 || strings.ContainsAny(l, "\x00\n\r") {
			return LabelAssignmentView{}, code("invalid_request")
		}
	}
	if _, e := s.file(p, fileID, "write"); e != nil {
		return LabelAssignmentView{}, e
	}
	if p.Agent != nil && q.Namespace != "suggested" {
		// Model output lands in suggested only; it can never overwrite the
		// human decision layer.
		return LabelAssignmentView{}, code("label_namespace_forbidden")
	}
	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		var file File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", fileID, p.dataOwner()).First(&file).Error; err != nil { return resourceReadError(err) }
		if file.Trashed || file.PurgeStartedAt != nil { return code("not_found") }
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil { return err }
		var existing []FileLabelAssignment
		if err := tx.Where("owner = ? AND file_id = ?", p.dataOwner(), fileID).Find(&existing).Error; err != nil { return code("database_unavailable") }
		if q.ExpectedRevision != "" && q.ExpectedRevision != revisionString(labelRevision(existing)) { return code("version_conflict") }
		now := time.Now().UTC()
		if err := tx.Where("owner = ? AND file_id = ? AND namespace = ?", p.dataOwner(), fileID, q.Namespace).Delete(&FileLabelAssignment{}).Error; err != nil {
			return code("database_unavailable")
		}
		author := "human"
		if p.Agent != nil {
			author = "model"
		} else if q.Namespace == "rule" {
			author = "rule"
		}
		for _, label := range q.Labels {
			row := FileLabelAssignment{ID: ID(), Owner: p.dataOwner(), FileID: fileID, Namespace: q.Namespace, Label: label, AuthorKind: author, ActorID: p.UserID, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return code("database_unavailable")
			}
		}
		return nil
	})
	if txErr != nil {
		return LabelAssignmentView{}, txErr
	}
	return s.LabelAssignmentsFor(p, fileID)
}

func revisionString(v uint64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// LabelAssignmentsFor reads one file's labels across namespaces.
func (s *Service) LabelAssignmentsFor(p Principal, fileID string) (LabelAssignmentView, error) {
	if _, e := s.file(p, fileID, "read"); e != nil {
		return LabelAssignmentView{}, e
	}
	var rows []FileLabelAssignment
	if e := s.DB.Where("owner = ? AND file_id = ?", p.dataOwner(), fileID).Find(&rows).Error; e != nil {
		return LabelAssignmentView{}, code("database_unavailable")
	}
	view := LabelAssignmentView{FileID: fileID, Namespaces: map[string][]string{"manual": {}, "rule": {}, "suggested": {}}}
	for _, r := range rows {
		view.Namespaces[r.Namespace] = append(view.Namespaces[r.Namespace], r.Label)
		if r.UpdatedAt.After(view.UpdatedAt) {
			view.UpdatedAt = r.UpdatedAt
		}
	}
	view.Revision = labelRevision(rows)
	return view, nil
}

// Preserve existing application and SDK callers through aliases to public DTOs.
type FileCollectionView = contract.FileCollectionView
type CreateFileCollectionRequest = contract.CreateFileCollectionRequest

// CreateFileCollection opens one manual or rule collection under a durable
// idempotency key.
func (s *Service) CreateFileCollection(p Principal, q CreateFileCollectionRequest, key string) (FileCollectionView, error) {
	if key == "" || len(key) > 200 {
		return FileCollectionView{}, code("invalid_request")
	}
	if (q.Kind != "manual" && q.Kind != "rule") || (q.Scope != "private" && q.Scope != "team") || q.Name == "" || len(q.Name) > 200 {
		return FileCollectionView{}, code("invalid_request")
	}
	if q.Kind == "rule" {
		if _, err := collectionRule(q.Rule, q.SpaceID); err != nil {
			return FileCollectionView{}, err
		}
	}
	rule := ""
	if q.Kind == "rule" {
		rule = string(q.Rule)
	}
	fingerprint := collectionCreationDigest(q)
	s.mu.Lock()
	defer s.mu.Unlock()
	var out FileCollectionView
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := savedQueryActorOn(tx, p); err != nil {
			return err
		}
		if err := s.collectionReadSpaceOn(tx, p, q.SpaceID); err != nil {
			return err
		}
		for _, action := range []string{"read", "write"} {
			if err := s.allowOn(tx, p, q.SpaceID, "", action); err != nil {
				return err
			}
		}
		if q.Scope == "team" {
			if p.Agent != nil {
				return code("forbidden")
			}
			if err := s.allowOn(tx, p, q.SpaceID, "", "manage"); err != nil {
				return err
			}
		}
		var collection FileCollection
		err := tx.First(&collection, "owner = ? AND key_id = ?", p.dataOwner(), key).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			now := time.Now().UTC()
			collection = FileCollection{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, SpaceID: q.SpaceID, Name: q.Name, Kind: q.Kind, Scope: q.Scope, Rule: rule, RuleRevision: 1, Revision: 1, CreatedAt: now, UpdatedAt: now, KeyID: key, Fingerprint: fingerprint}
			if err := tx.Create(&collection).Error; err != nil {
				return code("database_unavailable")
			}
			// New human creations persist the original fact atomically. Replaying
			// a legacy or Agent row must never manufacture an original actor.
			if collectionHumanPrincipal(p) {
				receipt := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "collection.create", RequestKey: key, RequestDigest: fingerprint, ResourceID: collection.ID, ResultRevision: 1, CreatedAt: now}
				if err := tx.Create(&receipt).Error; err != nil {
					return code("database_unavailable")
				}
			}
		}
		// Preserve the legacy owner/key uniqueness without revealing a row
		// from another tenant or treating a new scope as an old receipt.
		if collection.TenantID != p.TenantID {
			return code("idempotency_conflict")
		}
		if collection.Fingerprint != fingerprint || collection.SpaceID != q.SpaceID || collection.Kind != q.Kind || collection.Scope != q.Scope || collection.Name != q.Name || collection.Rule != rule {
			return code("idempotency_conflict")
		}
		out, err = s.fileCollectionViewOn(tx, collection)
		return err
	})
	if e != nil {
		return FileCollectionView{}, e
	}
	return out, nil
}

// ListFileCollections lists the caller's collections in one space.
func (s *Service) ListFileCollections(p Principal, spaceID string) ([]FileCollectionView, error) {
	out := []FileCollectionView{}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if p.UserID == "" {
			return code("unauthorized")
		}
		var user User
		if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("forbidden")
			}
			return code("database_unavailable")
		}
		if user.Disabled || user.Archived {
			return code("forbidden")
		}
		if err := s.collectionReadSpaceOn(tx, p, spaceID); err != nil {
			return err
		}
		// The legacy list exposes names, rules and whole-collection counts.
		// A path-prefix grant cannot authorize that space-wide metadata.
		if err := s.allowOn(tx, p, spaceID, "", "read"); err != nil {
			return err
		}
		var collections []FileCollection
		q := tx.Where("owner = ? AND tenant_id = ? AND space_id = ?", p.dataOwner(), p.TenantID, spaceID)
		if p.Agent != nil {
			q = q.Where("scope = ?", "private")
		}
		if err := q.Order("created_at ASC, id ASC").Limit(1001).Find(&collections).Error; err != nil {
			return code("database_unavailable")
		}
		if len(collections) > 1000 {
			return code("query_budget_exceeded")
		}
		for _, c := range collections {
			view, err := s.fileCollectionViewOn(tx, c)
			if err != nil {
				return err
			}
			out = append(out, view)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) fileCollectionView(p Principal, c FileCollection) (FileCollectionView, error) {
	return s.fileCollectionViewOn(s.DB, c)
}

func (s *Service) fileCollectionViewOn(tx *gorm.DB, c FileCollection) (FileCollectionView, error) {
	var members int64
	if err := tx.Model(&CollectionMember{}).Where("collection_id = ? AND decision = ?", c.ID, "include").Count(&members).Error; err != nil {
		return FileCollectionView{}, code("database_unavailable")
	}
	return FileCollectionView{CollectionID: c.ID, SpaceID: c.SpaceID, Name: c.Name, Kind: c.Kind, Scope: c.Scope, Rule: c.Rule, RuleRevision: c.RuleRevision, Revision: c.Revision, ComputedAt: c.ComputedAt, MembersTotal: members, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}, nil
}

func (s *Service) collection(p Principal, id string) (FileCollection, error) {
	var c FileCollection
	if s.DB.First(&c, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return c, code("not_found")
	}
	return c, nil
}

// SetCollectionMembersRequest replaces the explicit member decisions.
type SetCollectionMembersRequest struct {
	ExpectedRevision uint64   `json:"expected_revision"`
	Include          []string `json:"include,omitempty"`
	Exclude          []string `json:"exclude,omitempty"`
}

// SetCollectionMembers rewrites the explicit include/exclude decisions
// under an optimistic revision check; a stale writer gets
// collection_revision_conflict and the server keeps its own decisions.
func (s *Service) SetCollectionMembers(p Principal, collectionID string, q SetCollectionMembersRequest) (FileCollectionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out FileCollectionView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = s.setCollectionMembersOn(tx, p, collectionID, q)
		return err
	})
	if err != nil {
		return FileCollectionView{}, err
	}
	return out, nil
}

// Shared mutation engine: callers own serialization, transaction and any receipt.
func (s *Service) setCollectionMembersOn(tx *gorm.DB, p Principal, collectionID string, q SetCollectionMembersRequest) (FileCollectionView, error) {
	if q.ExpectedRevision == 0 || q.ExpectedRevision >= 9007199254740991 || len(q.Include)+len(q.Exclude) > 10000 {
		return FileCollectionView{}, code("invalid_request")
	}
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, q.Include...), q.Exclude...) {
		if len(id) != 48 || seen[id] {
			return FileCollectionView{}, code("invalid_request")
		}
		seen[id] = true
	}
	var out FileCollectionView
	err := func() error {
		if err := savedQueryActorOn(tx, p); err != nil {
			return err
		}
		c, err := s.collectionReadAdmissionOn(tx, p, collectionID)
		if err != nil {
			return err
		}
		if err := s.allowOn(tx, p, c.SpaceID, "", "write"); err != nil {
			return err
		}
		if c.Revision != q.ExpectedRevision {
			return code("collection_revision_conflict")
		}
		for id := range seen {
			var file File
			if err := tx.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&file).Error; err != nil {
				return resourceReadError(err)
			}
			if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
				return code("not_found")
			}
			if err := s.collectionReadSpaceOn(tx, p, file.SpaceID); err != nil {
				return err
			}
			if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		updated := tx.Model(&FileCollection{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ?", c.ID, c.Owner, c.TenantID, q.ExpectedRevision).Updates(map[string]any{"revision": q.ExpectedRevision + 1, "updated_at": now})
		if updated.Error != nil {
			return code("database_unavailable")
		}
		if updated.RowsAffected != 1 {
			return code("collection_revision_conflict")
		}
		if err := tx.Where("collection_id = ?", c.ID).Delete(&CollectionMember{}).Error; err != nil {
			return code("database_unavailable")
		}
		for _, decision := range []struct {
			ids  []string
			name string
		}{{q.Include, "include"}, {q.Exclude, "exclude"}} {
			for _, id := range decision.ids {
				if err := tx.Create(&CollectionMember{ID: ID(), CollectionID: c.ID, FileID: id, Owner: p.dataOwner(), Decision: decision.name, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
					return code("database_unavailable")
				}
			}
		}
		c.Revision, c.UpdatedAt = q.ExpectedRevision+1, now
		out, err = s.fileCollectionViewOn(tx, c)
		return err
	}()
	if err != nil {
		return FileCollectionView{}, err
	}
	return out, nil
}

// CollectionReadView is one collection read: the current visible members
// (dynamic collections follow the files' current version) with re-verified
// permissions. Revoked resources drop out of both members and counts.
type CollectionReadView struct {
	CollectionID string          `json:"collection_id"`
	Kind         string          `json:"kind"`
	Revision     uint64          `json:"revision"`
	ComputedAt   time.Time       `json:"computed_at"`
	Items        []FileQueryItem `json:"items"`
	VisibleCount int64           `json:"visible_count"`
	Coverage     string          `json:"coverage"`
}

// ReadFileCollection resolves one collection's current membership with
// per-entry re-authorization. Entries whose file is unreadable now are
// filtered from members and counts alike. Rule collections evaluate their
// stored typed conditions through the same query contract on every read;
// manual includes add to the rule result and manual excludes remove from
// it (include ∪ rule − exclude).
func (s *Service) ReadFileCollection(p Principal, collectionID string) (CollectionReadView, error) {
	c, e := s.collectionReadAdmission(p, collectionID)
	if e != nil {
		return CollectionReadView{}, e
	}
	now := time.Now().UTC()
	var memberRows []CollectionMember
	if err := s.DB.Where("collection_id = ?", c.ID).Order("created_at, id").Limit(10001).Find(&memberRows).Error; err != nil {
		return CollectionReadView{}, code("database_unavailable")
	}
	if len(memberRows) > 10000 {
		return CollectionReadView{}, code("query_budget_exceeded")
	}
	includes := map[string]bool{}
	excludes := map[string]bool{}
	for _, m := range memberRows {
		if m.Decision == "include" {
			includes[m.FileID] = true
		} else if m.Decision == "exclude" {
			excludes[m.FileID] = true
		}
	}
	view := CollectionReadView{CollectionID: c.ID, Kind: c.Kind, Revision: c.Revision, ComputedAt: now, Items: []FileQueryItem{}, Coverage: "complete"}
	if c.Kind != "manual" && c.Kind != "rule" {
		return CollectionReadView{}, code("invalid_query")
	}
	if c.Kind == "rule" {
		// The stored rule is a typed FileQueryRequest; evaluation goes
		// through the same query contract so permission injection and
		// pagination rules stay identical to direct queries. Pages walk
		// to the end: a rule matching more than one page of files still
		// evaluates its whole result set.
		rule, err := collectionRule([]byte(c.Rule), c.SpaceID)
		if err != nil {
			return CollectionReadView{}, err
		}
		coverage := "partial"
		finished := false
		for range 100 {
			ruled, e := s.QueryFiles(p, rule)
			if e != nil {
				return CollectionReadView{}, e
			}
			for _, it := range ruled.Items {
				if excludes[it.FileID] {
					continue
				}
				view.Items = append(view.Items, it)
			}
			if ruled.NextCursor == "" {
				coverage = ruled.Coverage
				finished = true
				break
			}
			rule.Cursor = ruled.NextCursor
			coverage = "partial"
		}
		if !finished {
			return CollectionReadView{}, code("query_budget_exceeded")
		}
		view.Coverage = coverage
	}
	for fileID := range includes {
		if excludes[fileID] {
			continue
		}
		dup := false
		for _, it := range view.Items {
			if it.FileID == fileID {
				dup = true
			}
		}
		if dup {
			continue
		}
		f, e := s.collectionReadableFile(p, fileID)
		if e != nil && !workspaceVisibilityDenied(e) {
			return CollectionReadView{}, e
		}
		if e != nil || f.Trashed || f.PurgeStartedAt != nil {
			// Lost permission or trashed: filtered from members and the
			// visible count alike.
			continue
		}
		view.Items = append(view.Items, FileQueryItem{FileID: f.ID, VersionID: f.VersionID, Path: f.Path})
	}
	// Rule results were produced under the query contract's own
	// authorization; every member is re-checked for current readability
	// before it counts.
	visible := make([]FileQueryItem, 0, len(view.Items))
	for _, it := range view.Items {
		if excludes[it.FileID] {
			continue
		}
		f, e := s.collectionReadableFile(p, it.FileID)
		if e != nil && !workspaceVisibilityDenied(e) {
			return CollectionReadView{}, e
		}
		if e != nil || f.Trashed || f.PurgeStartedAt != nil {
			continue
		}
		visible = append(visible, FileQueryItem{FileID: f.ID, VersionID: f.VersionID, Path: f.Path})
	}
	view.Items = visible
	view.VisibleCount = int64(len(view.Items))
	current, err := s.collectionReadAdmission(p, collectionID)
	if err != nil {
		return CollectionReadView{}, err
	}
	if current.Revision != c.Revision || current.SpaceID != c.SpaceID || current.Kind != c.Kind || current.Rule != c.Rule {
		return CollectionReadView{}, code("query_changed")
	}
	// ComputedAt is this response's observation, not a persisted collection edit.
	return view, nil
}

// Keep missing/hidden resources distinct from database failures during reads.
func (s *Service) collectionReadableFile(p Principal, id string) (File, error) {
	var file File
	if err := s.DB.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return file, code("not_found")
		}
		return file, code("database_unavailable")
	}
	if err := s.collectionReadSpace(p, file.SpaceID); err != nil {
		return file, err
	}
	return file, s.allow(p, file.SpaceID, file.Path, "read")
}

// SnapshotView reports one frozen read.
type SnapshotView struct {
	SnapshotID         string          `json:"snapshot_id"`
	CollectionID       string          `json:"collection_id"`
	CollectionRevision uint64          `json:"collection_revision"`
	EntriesTotal       int64           `json:"entries_total"`
	EntriesVisible     int64           `json:"entries_visible"`
	CreatedAt          time.Time       `json:"created_at"`
	Items              []FileQueryItem `json:"items"`
}

// CreateFileSnapshot freezes one collection read: each entry pins the exact
// file/version captured now. The snapshot grants no read permission; later
// reads re-authorize every entry and filter lost ones.
func (s *Service) CreateFileSnapshot(p Principal, collectionID string) (SnapshotView, error) {
	read, e := s.ReadFileCollection(p, collectionID)
	if e != nil {
		return SnapshotView{}, e
	}
	if read.Coverage != "complete" || len(read.Items) > 30000 {
		return SnapshotView{}, code("query_budget_exceeded")
	}
	var view SnapshotView
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		view, err = s.createFileSnapshotOn(tx, p, collectionID, read)
		return err
	})
	if e != nil {
		return SnapshotView{}, e
	}
	return view, nil
}

// Both legacy and keyed creation share this atomic publication engine.
func (s *Service) createFileSnapshotOn(tx *gorm.DB, p Principal, collectionID string, read CollectionReadView) (SnapshotView, error) {
	var view SnapshotView
	e := func() error {
		c, err := s.collectionReadAdmissionOn(tx, p, collectionID)
		if err != nil {
			return err
		}
		if c.Revision != read.Revision || c.Kind != read.Kind {
			return code("query_changed")
		}
		now := time.Now().UTC()
		snapshot := CollectionSnapshot{ID: ID(), CollectionID: c.ID, Owner: p.dataOwner(), Revision: read.Revision, EntriesTotal: int64(len(read.Items)), EntriesVisible: int64(len(read.Items)), CreatedAt: now}
		if err := tx.Create(&snapshot).Error; err != nil {
			return code("database_unavailable")
		}
		view = SnapshotView{SnapshotID: snapshot.ID, CollectionID: c.ID, CollectionRevision: read.Revision, EntriesTotal: snapshot.EntriesTotal, EntriesVisible: snapshot.EntriesVisible, CreatedAt: now, Items: []FileQueryItem{}}
		seen := map[string]bool{}
		for _, item := range read.Items {
			if seen[item.FileID] {
				return code("query_changed")
			}
			seen[item.FileID] = true
			var file File
			if err := tx.Where("id = ? AND owner = ?", item.FileID, p.dataOwner()).First(&file).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return code("query_changed")
				}
				return code("database_unavailable")
			}
			if file.Trashed || file.PurgeStartedAt != nil || file.VersionID != item.VersionID || file.Path != item.Path {
				return code("query_changed")
			}
			if err := s.collectionReadSpaceOn(tx, p, file.SpaceID); err != nil {
				return err
			}
			if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
				return err
			}
			var version Version
			if err := tx.Where("id = ? AND file_id = ? AND owner = ?", item.VersionID, item.FileID, p.dataOwner()).First(&version).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return code("query_changed")
				}
				return code("database_unavailable")
			}
			entry := CollectionSnapshotEntry{ID: ID(), SnapshotID: snapshot.ID, FileID: item.FileID, VersionID: item.VersionID, Path: item.Path, SHA256: version.SHA256, Visible: true, CreatedAt: now}
			if err := tx.Create(&entry).Error; err != nil {
				return code("database_unavailable")
			}
			view.Items = append(view.Items, FileQueryItem{FileID: item.FileID, VersionID: item.VersionID, Path: item.Path, SHA256: version.SHA256})
		}
		return nil
	}()
	if e != nil {
		return SnapshotView{}, e
	}
	// Return the committed frozen observation, not a second fallible read that
	// could hide a successful commit. Future reads must re-authorize separately.
	return view, nil
}

// ReadSnapshot reads one frozen snapshot. Entries whose file lost
// readability drop out of the visible list; the snapshot never expands
// access to what the reader could not see.
func (s *Service) ReadSnapshot(p Principal, snapshotID string) (SnapshotView, error) {
	var view SnapshotView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var snapshot CollectionSnapshot
		if err := tx.Where("id = ? AND owner = ?", snapshotID, p.dataOwner()).First(&snapshot).Error; err != nil {
			return resourceReadError(err)
		}
		// Even empty snapshots carry private collection metadata. Re-authorize the
		// original collection, but do not require its current revision to match.
		if _, err := s.collectionReadAdmissionOn(tx, p, snapshot.CollectionID); err != nil {
			return err
		}
		if snapshot.EntriesTotal > 30000 {
			return code("query_budget_exceeded")
		}
		if snapshot.Revision == 0 || snapshot.EntriesTotal < 0 || snapshot.EntriesVisible != snapshot.EntriesTotal {
			return code("dependency_unavailable")
		}
		var entries []CollectionSnapshotEntry
		if err := tx.Where("snapshot_id = ?", snapshot.ID).Order("created_at, id").Limit(30001).Find(&entries).Error; err != nil {
			return code("database_unavailable")
		}
		if len(entries) > 30000 {
			return code("query_budget_exceeded")
		}
		if int64(len(entries)) != snapshot.EntriesTotal {
			return code("dependency_unavailable")
		}
		view = SnapshotView{SnapshotID: snapshot.ID, CollectionID: snapshot.CollectionID, CollectionRevision: snapshot.Revision, EntriesTotal: snapshot.EntriesTotal, CreatedAt: snapshot.CreatedAt, Items: []FileQueryItem{}}
		seen := map[string]bool{}
		for _, entry := range entries {
			if entry.FileID == "" || entry.VersionID == "" || seen[entry.FileID] {
				return code("dependency_unavailable")
			}
			seen[entry.FileID] = true
			if !entry.Visible {
				continue
			}
			var file File
			if err := tx.Where("id = ? AND owner = ?", entry.FileID, p.dataOwner()).First(&file).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return code("database_unavailable")
			}
			if file.Trashed || file.PurgeStartedAt != nil {
				continue
			}
			if err := s.collectionReadSpaceOn(tx, p, file.SpaceID); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if err := s.allowOn(tx, p, file.SpaceID, entry.Path, "read"); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			var version Version
			if err := tx.Where("id = ? AND owner = ? AND file_id = ?", entry.VersionID, p.dataOwner(), file.ID).First(&version).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return code("database_unavailable")
			}
			if entry.SHA256 != "" && entry.SHA256 != version.SHA256 {
				return code("dependency_unavailable")
			}
			view.Items = append(view.Items, FileQueryItem{FileID: entry.FileID, VersionID: entry.VersionID, Path: entry.Path, SHA256: entry.SHA256})
		}
		view.EntriesVisible = int64(len(view.Items))
		return nil
	})
	if err != nil {
		return SnapshotView{}, err
	}
	return view, nil
}

// SavedQueryView is the saved-query read model.
type SavedQueryView struct {
	QueryID   string          `json:"query_id"`
	Name      string          `json:"name"`
	Query     json.RawMessage `json:"query"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SavedFileQueryCreate stores one exact typed query. Stored queries carry
// no permissions: execution re-authorizes everything at read time.
func (s *Service) SavedFileQueryCreate(p Principal, name string, q FileQueryRequest) (SavedQueryView, error) {
	if e := validateFileQuery(q); e != nil {
		return SavedQueryView{}, e
	}
	if name == "" || len(name) > 200 {
		return SavedQueryView{}, code("invalid_request")
	}
	now := time.Now().UTC()
	row := SavedFileQuery{ID: ID(), Owner: p.dataOwner(), Name: name, Query: canonicalQueryJSON(q), CreatedAt: now, UpdatedAt: now}
	if len(row.Query) > savedQueryBudget {
		return SavedQueryView{}, code("query_budget_exceeded")
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := savedQueryActorOn(tx, p); err != nil {
			return err
		}
		if err := s.savedQuerySpacesOn(tx, p, q); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	if err != nil {
		return SavedQueryView{}, err
	}
	return SavedQueryView{QueryID: row.ID, Name: row.Name, Query: json.RawMessage(row.Query), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

// SavedFileQueriesList lists the caller's saved queries.
func (s *Service) SavedFileQueriesList(p Principal) ([]SavedQueryView, error) {
	out := []SavedQueryView{}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := savedQueryActorOn(tx, p); err != nil {
			return err
		}
		var rows []SavedFileQuery
		if err := tx.Where("owner = ?", p.dataOwner()).Order("created_at, id").Limit(1001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 1000 {
			return code("query_budget_exceeded")
		}
		budget := 0
		for _, row := range rows {
			budget += len(row.Query)
			if budget > 1024*1024 {
				return code("query_budget_exceeded")
			}
			q, err := decodeSavedQuery(row.Query)
			if err != nil {
				return err
			}
			if err := s.savedQuerySpacesOn(tx, p, q); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			out = append(out, SavedQueryView{QueryID: row.ID, Name: row.Name, Query: json.RawMessage(canonicalQueryJSON(q)), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DuplicateGroupView is one exact-duplicate group: only verified equal
// SHA-256 digests inside the authorized scope group; similar or
// digest-unknown candidates are marked and never treated as deletable
// copies.
type DuplicateGroupView struct {
	Digest   string          `json:"digest,omitempty"`
	Strength string          `json:"strength"` // exact|similar|unknown
	Items    []FileQueryItem `json:"items"`
}

// DuplicateCandidatesView is the duplicates read.
type DuplicateCandidatesView struct {
	Groups   []DuplicateGroupView `json:"groups"`
	Coverage string               `json:"coverage"`
	Notes    []string             `json:"notes,omitempty"`
}

// QueryDuplicateCandidates groups exact duplicates by verified digest
// within the readable scope only; files with a similar normalized name but
// different digests surface as separate "similar" markers, and files
// without a verified digest are "unknown". Nothing here authorizes
// deletion.
func (s *Service) QueryDuplicateCandidates(p Principal, spaceIDs []string) (DuplicateCandidatesView, error) {
	if len(spaceIDs) == 0 {
		return DuplicateCandidatesView{}, code("invalid_query")
	}
	view := DuplicateCandidatesView{Groups: []DuplicateGroupView{}, Coverage: "complete", Notes: []string{"duplicate candidates never authorize purge; deletion is a separate confirmed action"}}
	var files []File
	if e := s.DB.Where("owner = ? AND space_id IN ? AND trashed = ?", p.dataOwner(), spaceIDs, false).Order("updated").Find(&files).Error; e != nil {
		return DuplicateCandidatesView{}, code("database_unavailable")
	}
	byDigest := map[string][]FileQueryItem{}
	similar := map[string][]FileQueryItem{}
	for _, f := range files {
		if e := s.allow(p, f.SpaceID, f.Path, "read"); e != nil {
			// Outside the current authorization: not part of any group.
			continue
		}
		var v Version
		if s.DB.First(&v, "id = ?", f.VersionID).Error != nil {
			continue
		}
		item := FileQueryItem{FileID: f.ID, VersionID: f.VersionID, Path: f.Path, SHA256: v.SHA256}
		if v.SHA256 == "" {
			continue
		}
		byDigest[v.SHA256] = append(byDigest[v.SHA256], item)
		similar[normalizeName(f.Path)] = append(similar[normalizeName(f.Path)], item)
	}
	for digest, items := range byDigest {
		if len(items) > 1 {
			view.Groups = append(view.Groups, DuplicateGroupView{Digest: digest, Strength: "exact", Items: items})
		}
	}
	for _, items := range similar {
		if len(items) < 2 {
			continue
		}
		distinct := map[string]bool{}
		for _, it := range items {
			distinct[it.SHA256] = true
		}
		if len(distinct) > 1 {
			view.Groups = append(view.Groups, DuplicateGroupView{Strength: "similar", Items: items})
		}
	}
	return view, nil
}

func normalizeName(path string) string {
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	if idx := strings.Index(base, "."); idx > 0 {
		base = base[:idx]
	}
	// "report v1" / "report final" share the "report" stem: similarity is
	// name-level only and never implies equal bytes.
	if idx := strings.Index(base, " "); idx > 0 {
		base = base[:idx]
	}
	return strings.ToLower(strings.TrimSpace(base))
}

var _ = http.StatusOK
