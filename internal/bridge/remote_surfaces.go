package bridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ───────────────────────────── transport boundary ─────────────────────────────

// RemoteJSON is the server's public JSON contract. The shared server SDK
// client satisfies it; tests inject contract fixtures. Only documented
// /api/v1 routes cross this boundary.
type RemoteJSON interface {
	JSON(ctx context.Context, method, path, key string, body, out any) error
}

// RemoteStream pushes one durable part through the server's public byte
// path. Bytes never flow through MCP: only the local adapter (or the web
// client talking to the server directly) opens this channel.
type RemoteStream interface {
	PutPart(ctx context.Context, operationID string, index, size int64, body io.Reader) error
}

// PartUploader is the default RemoteStream over the documented
// PUT /api/v1/operations/{id}/parts/{index} route. It mirrors the JSON
// client's bearer policy and error envelope and never logs tokens or
// payload bytes.
type PartUploader struct {
	Base  string
	Token string
	HTTP  *http.Client
}

func (p *PartUploader) PutPart(ctx context.Context, operationID string, index, size int64, body io.Reader) error {
	client := p.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	target := fmt.Sprintf("%s/api/v1/operations/%s/parts/%d", strings.TrimSuffix(p.Base, "/"), url.PathEscape(operationID), index)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, body)
	if err != nil {
		return errors.New("invalid_request")
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = size
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("service_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&env)
	if safeErrorString(env.Error.Code) != "service_request_failed" {
		return errors.New(env.Error.Code)
	}
	return errors.New("service_request_failed")
}

// safeErrorString normalizes one server error code to the safe machine set;
// anything malformed collapses instead of leaking transport text.
func safeErrorString(code string) string {
	if code == "" || len(code) > 80 {
		return "service_request_failed"
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return "service_request_failed"
		}
	}
	return code
}

// ───────────────────────────── errors ─────────────────────────────

// RemoteError carries a stable machine code plus adapter-local recovery
// detail. Error() stays the bare code so every existing surface (string
// comparisons, hint lookup, HTTP mapping) keeps working byte-for-byte.
type RemoteError struct {
	Code    string   `json:"code"`
	Message string   `json:"message,omitempty"`
	Hint    string   `json:"next_action,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

func (e *RemoteError) Error() string        { return e.Code }
func (e *RemoteError) RecoveryHint() string { return e.Hint }
func (e *RemoteError) MissingActions() []string {
	if len(e.Missing) == 0 {
		return nil
	}
	return e.Missing
}

// ───────────────────────────── journal model ─────────────────────────────

// Remote intent phases describe what the adapter knows about one issued
// write. They never touch the legacy Operation.status enum.
const (
	remoteIntentIssued    = "issued"    // request left; no verdict recorded yet
	remoteIntentConfirmed = "confirmed" // server receipt recorded under its original ID
	remoteIntentUnknown   = "unknown"   // transport lost after issue; same-key lookup pending
	remoteIntentFailed    = "failed"    // server refused; the key stays reserved
)

// RemoteIntent journals one adapter-issued remote write intent BEFORE the
// request leaves: the idempotency key, the canonical fingerprint of the
// original business request and, once known, the server's original IDs
// stored verbatim. On a lost response the same key plus the same
// fingerprint replays lookup; a different fingerprint under the same key is
// refused instead of issued. The journal is adapter-owned transfer state;
// the server remains the source of truth for the operations themselves.
type RemoteIntent struct {
	ID          uint64    `json:"id" gorm:"primaryKey"`
	Instance    string    `json:"instance_id" gorm:"uniqueIndex:idx_remote_intents_scope"`
	Action      string    `json:"action" gorm:"uniqueIndex:idx_remote_intents_scope"`
	Key         string    `json:"idempotency_key" gorm:"uniqueIndex:idx_remote_intents_scope"`
	Fingerprint string    `json:"fingerprint"`
	Request     string    `json:"original_request"`
	Phase       string    `json:"phase"`
	RemoteRef   string    `json:"remote_ref,omitempty"`
	RefKind     string    `json:"ref_kind,omitempty"`
	Certainty   string    `json:"result_certainty,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// remoteFingerprint digests the canonical form of one original request.
// JSON object encoding is key-sorted, so the digest is stable without a
// second hash rule; the server recomputes its own fingerprint from the same
// business inputs and the adapter only needs local equality.
func remoteFingerprint(action string, original map[string]any) string {
	b, err := json.Marshal(original)
	if err != nil {
		b = []byte("{}")
	}
	h := sha256.New()
	h.Write([]byte(action))
	h.Write([]byte{0})
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// journalIntent records the intent before the request leaves. Same key with
// the same fingerprint returns the prior row (a safe replay); a different
// fingerprint is refused outright so a lost response can never turn into a
// second, divergent write under the same key. A missing journal (thin
// remote mode) is not an error: the caller-held key still governs recovery.
func (r *RemoteSurfaces) journalIntent(action, key, fingerprint string, original map[string]any) (*RemoteIntent, error) {
	if r.DB == nil {
		return nil, nil
	}
	request, _ := json.Marshal(original)
	var row RemoteIntent
	err := r.DB.Where("instance = ? AND action = ? AND key = ?", r.Instance, action, key).First(&row).Error
	if err == nil {
		if row.Fingerprint != fingerprint {
			return nil, &RemoteError{Code: "idempotency_conflict", Hint: "The idempotency key was already used with different business inputs; keep the original request or issue a new key."}
		}
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("state_unavailable")
	}
	// A unique-index race collapses to the same conflict check on re-read.
	row = RemoteIntent{Instance: r.Instance, Action: action, Key: key, Fingerprint: fingerprint, Request: string(request), Phase: remoteIntentIssued, UpdatedAt: time.Now().UTC()}
	if err = r.DB.Create(&row).Error; err != nil {
		if err2 := r.DB.Where("instance = ? AND action = ? AND key = ?", r.Instance, action, key).First(&row).Error; err2 == nil && row.Fingerprint == fingerprint {
			return &row, nil
		}
		return nil, &RemoteError{Code: "idempotency_conflict", Hint: "The idempotency key was already used with different business inputs; keep the original request or issue a new key."}
	}
	return &row, nil
}

// settleIntent freezes the verdict of one issued write. Settle is
// best-effort by design: a journal write failure must never mask a server
// receipt the caller already holds.
func (r *RemoteSurfaces) settleIntent(row *RemoteIntent, phase, refKind, remoteRef, certainty string) {
	if r.DB == nil || row == nil {
		return
	}
	row.Phase = phase
	if refKind != "" {
		row.RefKind = refKind
	}
	if remoteRef != "" {
		row.RemoteRef = remoteRef
	}
	row.Certainty = certainty
	row.UpdatedAt = time.Now().UTC()
	r.DB.Save(row)
}

// ───────────────────────────── frozen action catalog ─────────────────────────────

// remoteSurfaceActionDef freezes one public action consumed from the server
// contracts: method, route template, whether it writes (idempotency key
// scoped to instance+actor+action), the path argument name and where the
// server's original ID lands in the receipt.
type remoteSurfaceActionDef struct {
	Method  string
	Path    string
	PathArg string
	Write   bool
	RefKind string
	Query   []string
}

// remoteSurfaceCatalog is the frozen union of the five dependency contract
// tables (upload batch recovery, cloud connectors, analytics search, agent
// organization, controlled automation). Human-only management entries —
// OAuth connection begin, plan confirmation, organization policy, automation
// rule authoring — are deliberately absent: they are never registered for
// agents and cannot be reached through this adapter.
var remoteSurfaceCatalog = map[string]remoteSurfaceActionDef{
	"upload_batch_create":        {Method: "POST", Path: "/api/v1/upload-batches", Write: true, RefKind: "batch"},
	"upload_batch_add_items":     {Method: "POST", Path: "/api/v1/upload-batches/%s/items", PathArg: "batch_id", Write: true, RefKind: "batch"},
	"upload_batch_seal":          {Method: "POST", Path: "/api/v1/upload-batches/%s/seal", PathArg: "batch_id", Write: true, RefKind: "operation"},
	"upload_batch_get":           {Method: "GET", Path: "/api/v1/upload-batches/%s/items", PathArg: "batch_id", Query: []string{"cursor", "limit"}},
	"upload_batch_pause":         {Method: "POST", Path: "/api/v1/upload-batches/%s/pause", PathArg: "batch_id", Write: true, RefKind: "batch"},
	"upload_batch_resume":        {Method: "POST", Path: "/api/v1/upload-batches/%s/resume", PathArg: "batch_id", Write: true, RefKind: "batch"},
	"upload_batch_cancel":        {Method: "POST", Path: "/api/v1/upload-batches/%s/cancel", PathArg: "batch_id", Write: true, RefKind: "batch"},
	"operation_lookup":           {Method: "POST", Path: "/api/v1/operations/lookup"},
	"analytics_query":            {Method: "POST", Path: "/api/v1/analytics/queries", Write: true, RefKind: "query"},
	"analytics_query_get":        {Method: "GET", Path: "/api/v1/analytics/queries/%s", PathArg: "query_id"},
	"analytics_snapshot":         {Method: "POST", Path: "/api/v1/analytics/snapshots", Write: true, RefKind: "snapshot"},
	"analytics_snapshot_query":   {Method: "POST", Path: "/api/v1/analytics/snapshots/%s/queries", PathArg: "snapshot_id"},
	"organize_plan":              {Method: "POST", Path: "/api/v1/organization-plans", Write: true, RefKind: "plan"},
	"organize_plan_get":          {Method: "GET", Path: "/api/v1/organization-plans/%s", PathArg: "plan_id"},
	"organize_apply":             {Method: "POST", Path: "/api/v1/organization-plans/%s/execute", PathArg: "plan_id", Write: true},
	"organize_compensation_plan": {Method: "POST", Path: "/api/v1/organization-plans/%s/compensations", PathArg: "plan_id", Write: true, RefKind: "plan"},
	"cloud_entries":              {Method: "GET", Path: "/api/v1/connections/%s/entries", PathArg: "connection_id", Query: []string{"cursor", "limit", "parent_ref"}},
	"cloud_scan":                 {Method: "POST", Path: "/api/v1/connections/%s/scan", PathArg: "connection_id", Write: true, RefKind: "operation"},
	"changes_v2":                 {Method: "GET", Path: "/api/v1/changes", Query: []string{"cursor", "space_id", "limit"}},
	"change_subscription_create": {Method: "POST", Path: "/api/v1/change-subscriptions", Write: true, RefKind: "subscription"},
	"change_subscription_ack":    {Method: "POST", Path: "/api/v1/change-subscriptions/%s/ack", PathArg: "subscription_id", Write: true},
	"automation_intent_get":      {Method: "GET", Path: "/api/v1/automation-intents/%s", PathArg: "intent_id"},
}

// RemoteSurfaceActionNames returns the frozen catalog names in stable
// (sorted) order so discovery gaps report deterministically.
func RemoteSurfaceActionNames() []string {
	out := make([]string, 0, len(remoteSurfaceCatalog))
	for name := range remoteSurfaceCatalog {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// IsRemoteSurfaceAction reports whether one action name belongs to the
// frozen remote-surface catalog.
func IsRemoteSurfaceAction(name string) bool {
	_, ok := remoteSurfaceCatalog[name]
	return ok
}

// remoteRefFields maps a ref kind to the receipt fields that may carry the
// server's original ID, in priority order. IDs are read verbatim.
var remoteRefFields = map[string][]string{
	"batch":        {"batch_id"},
	"operation":    {"operation_id"},
	"plan":         {"plan_id"},
	"query":        {"query_id"},
	"snapshot":     {"snapshot_id"},
	"subscription": {"subscription_id"},
}

// remoteRefFrom picks the server's original ID out of one receipt. The
// adapter never renumbers or rewrites remote IDs.
func remoteRefFrom(kind string, receipt json.RawMessage) string {
	fields, ok := remoteRefFields[kind]
	if !ok || len(receipt) == 0 {
		return ""
	}
	var doc map[string]any
	if json.Unmarshal(receipt, &doc) != nil {
		return ""
	}
	for _, field := range fields {
		if v, ok := doc[field].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// ───────────────────────────── paging ─────────────────────────────

// New DriveBridge paging defaults: 50 per page, at most 200.
const (
	remotePageDefault = 50
	remotePageMax     = 200
)

// remotePageLimit clamps one requested page size onto the frozen window.
func remotePageLimit(v any) string {
	limit := remotePageDefault
	switch n := v.(type) {
	case float64:
		limit = int(n)
	case int:
		limit = n
	case int64:
		limit = int(n)
	case string:
		if parsed, err := strconv.Atoi(n); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > remotePageMax {
		limit = remotePageMax
	}
	return strconv.Itoa(limit)
}

// ───────────────────────────── service ─────────────────────────────

// RemoteAnalyticsRunner executes typed queries against a locally held,
// explicitly authorized snapshot copy. The translation from the typed
// contract to any local engine belongs to the server's shared analytics
// adapter; this adapter only forwards the typed query and reports the
// offline-copy provenance. A nil runner keeps the optional path disabled.
type RemoteAnalyticsRunner interface {
	QuerySnapshot(ctx context.Context, snapshotID string, query map[string]any) (json.RawMessage, error)
}

// RemoteSurfaces consumes the server's public remote contract: capability
// discovery, batched upload, analytics queries and snapshots, organization
// plans, connector listings and change subscriptions. It copies no
// server-side authorization, cloud provider state or analytics rules, and
// never shells out to a product CLI or reads the server's private stores.
type RemoteSurfaces struct {
	Instance  string
	Call      RemoteJSON
	Stream    RemoteStream
	DB        *gorm.DB
	Root      *os.Root
	Analytics RemoteAnalyticsRunner
}

// NewRemoteSurfaces binds one server instance. db and root are optional:
// without a journal the caller-held idempotency key still governs recovery,
// and without a local root the adapter never pretends to read local paths.
func NewRemoteSurfaces(instance string, call RemoteJSON, stream RemoteStream, db *gorm.DB, root *os.Root) *RemoteSurfaces {
	return &RemoteSurfaces{Instance: instance, Call: call, Stream: stream, DB: db, Root: root}
}

// Actions discovers the actions the server currently advertises through the
// public capabilities document. Registration decisions come from this live
// answer, never from assumptions about a server version.
func (r *RemoteSurfaces) Actions(ctx context.Context) ([]string, error) {
	var doc struct {
		Actions []string `json:"actions"`
	}
	if err := r.Call.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &doc); err != nil {
		return nil, err
	}
	return doc.Actions, nil
}

// localSourceExplanation is the honest answer when a caller asks this
// adapter to read a local path it has no root for: name the real byte
// entries instead of pretending the remote control plane can read disks.
func localSourceExplanation() *RemoteError {
	return &RemoteError{
		Code: "unsupported",
		Hint: "This adapter has no local byte source configured. Upload bytes through the service web/HTTP upload entry; the MCP surface only carries references, digests and IDs.",
	}
}

// Run executes one catalog action against the server through the shared
// application contract. Write actions journal the intent before the request
// leaves and settle it after; a transport failure after issue settles the
// intent as unknown with the same-key lookup recovery attached. Server IDs
// in the receipt pass through verbatim.
func (r *RemoteSurfaces) Run(ctx context.Context, action string, args map[string]any) (any, error) {
	if local, ok, err := runRemoteSurfaceLocalAction(r, ctx, action, args); ok {
		return local, err
	}
	def, ok := remoteSurfaceCatalog[action]
	if !ok {
		return nil, &RemoteError{
			Code:    "unsupported",
			Missing: RemoteSurfaceActionNames(),
			Hint:    "The action is not in this adapter's frozen remote-surface catalog; re-discover via capabilities/tools/list and do not guess aliases or auto-install anything.",
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	// The caller's argument map is never mutated: every surface shares this
	// one application contract and callers may replay the same map.
	original := make(map[string]any, len(args))
	for k, v := range args {
		original[k] = v
	}
	args = original
	// A local path is adapter-private, never server business input. Without
	// a configured root the request is answered with the real byte entry
	// points; with one, the path must resolve inside it.
	pathValue := ""
	if v, ok := args["local_path"].(string); ok && v != "" {
		if r.Root == nil {
			return nil, localSourceExplanation()
		}
		f, err := r.Root.Open(v)
		if err != nil {
			return nil, errors.New("path_not_allowed")
		}
		f.Close()
		pathValue = v
		delete(args, "local_path")
	}
	// Typed analytics validation stays client-side to avoid a doomed round
	// trip; the server revalidates authoritatively.
	if action == "analytics_query" || action == "analytics_snapshot_query" {
		if dataset, _ := args["dataset"].(string); action == "analytics_query" && dataset != "" {
			switch dataset {
			case "file_current", "transfer_facts", "audit_facts", "capacity_daily":
			default:
				return nil, errors.New("invalid_request")
			}
		}
		if v, ok := args["limit"]; ok && v != nil {
			args["limit"] = remotePageLimit(v)
		}
	}
	pathArg := ""
	if def.PathArg != "" {
		v, _ := args[def.PathArg].(string)
		if v == "" {
			return nil, errors.New("invalid_request")
		}
		pathArg = v
		delete(args, def.PathArg)
	}
	var row *RemoteIntent
	key := ""
	if def.Write {
		key, _ = args["idempotency_key"].(string)
		if key == "" || len(key) > 200 {
			return nil, errors.New("invalid_request")
		}
		// The idempotency key scopes the request (instance+actor+action);
		// the fingerprint and the request body carry business inputs only,
		// exactly the original_request shape the server recomputes. Keeping
		// the key out of both makes the pre-issued journal and this
		// dispatch agree on one fingerprint for the same intent.
		delete(args, "idempotency_key")
		original := map[string]any{}
		for k, v := range args {
			original[k] = v
		}
		var err error
		if row, err = r.journalIntent(action, key, remoteFingerprint(action, original), original); err != nil {
			return nil, err
		}
	}
	path := def.Path
	if pathArg != "" {
		path = fmt.Sprintf(path, url.PathEscape(pathArg))
	}
	var body any
	if def.Method != "GET" {
		body = args
	} else if len(def.Query) > 0 {
		q := url.Values{}
		paged := false
		for _, field := range def.Query {
			if field == "limit" {
				paged = true
				continue
			}
			if v, ok := args[field].(string); ok && v != "" {
				q.Set(field, v)
			}
		}
		if paged {
			// The frozen paging window applies even when the caller omits
			// the page size: default 50, maximum 200.
			q.Set("limit", remotePageLimit(args["limit"]))
		}
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		body = nil
	}
	if pathValue != "" {
		// Restore the adapter-private source for the composite upload path.
		args["local_path"] = pathValue
	}
	// The optional local snapshot execution path: the server's shared typed
	// adapter owns any local engine; without one configured the optional
	// path reports its own availability instead of degrading the query.
	if action == "analytics_snapshot_query" && args["execution"] == "local" {
		if r.Analytics == nil {
			return nil, &RemoteError{Code: "analytics_backend_unavailable", Hint: "The optional local snapshot backend is not configured on this adapter; query the snapshot server-side or configure the local runner. Server-side queries and uploads are unaffected."}
		}
		snapshotID := pathArg
		query := map[string]any{}
		for k, v := range args {
			query[k] = v
		}
		delete(query, "execution")
		out, err := r.Analytics.QuerySnapshot(ctx, snapshotID, query)
		if err != nil {
			return nil, err
		}
		return remoteDecode(out), nil
	}
	var receipt json.RawMessage
	err := r.Call.JSON(ctx, def.Method, path, key, body, &receipt)
	if err != nil {
		if row != nil {
			code := safeErrorString(err.Error())
			if code == "service_unavailable" {
				// Transport lost after the request left: the outcome is
				// unknown. The same key plus the same original request
				// replays lookup; the adapter never re-keys, swaps targets
				// or guesses a duplicate write.
				r.settleIntent(row, remoteIntentUnknown, def.RefKind, "", "unknown")
				return nil, &RemoteError{Code: code, Hint: "The request outcome is unknown. Replay operation_lookup with the same idempotency_key and original_request; never re-key or create a replacement operation."}
			}
			// Any other code is a definitive server refusal: the key stays
			// reserved and a retry needs a new key or an identical replay.
			r.settleIntent(row, remoteIntentFailed, def.RefKind, "", "refused")
		}
		return nil, err
	}
	if row != nil {
		// A decoded receipt means the server confirmed under its original
		// ID; nothing about the verdict is inferred beyond that.
		r.settleIntent(row, remoteIntentConfirmed, def.RefKind, remoteRefFrom(def.RefKind, receipt), "confirmed")
	}
	return remoteDecode(receipt), nil
}

// remoteDecode widens one raw receipt into the generic JSON object every
// surface renders; a non-object receipt passes through raw instead of
// failing a call the server already accepted.
func remoteDecode(receipt json.RawMessage) any {
	var doc map[string]any
	if len(receipt) > 0 && json.Unmarshal(receipt, &doc) == nil {
		return doc
	}
	return receipt
}

// LookupOperation replays the frozen recovery: the same action, the same
// idempotency key and the original request (authentication excluded). The
// server answers with the original server IDs or an explicit not-found; the
// adapter adds nothing.
func (r *RemoteSurfaces) LookupOperation(ctx context.Context, action, key string, originalRequest map[string]any) (any, error) {
	body := map[string]any{"action": action, "idempotency_key": key, "original_request": originalRequest}
	var receipt json.RawMessage
	if err := r.Call.JSON(ctx, "POST", "/api/v1/operations/lookup", "", body, &receipt); err != nil {
		return nil, err
	}
	return remoteDecode(receipt), nil
}

// ───────────────────────────── bounded local upload ─────────────────────────────

// defaultRemotePartSize is the frozen client part size of the upload batch
// contract; the server may answer seal with its own part_size which then
// wins.
const defaultRemotePartSize = 8 << 20

// UploadLocalFile runs the bounded single-file remote upload flow through
// the public contracts: guard the configured root, digest the source, freeze
// one batch item, seal, then stream parts with a single reused part buffer.
// A lost create response recovers through operation_lookup under the
// original key and original request — never a new key or target.
func (r *RemoteSurfaces) UploadLocalFile(ctx context.Context, space, localPath, conflictPolicy, key string) (map[string]any, error) {
	if r.Root == nil {
		return nil, localSourceExplanation()
	}
	if key == "" || len(key) > 200 {
		return nil, errors.New("invalid_request")
	}
	// Clean collapses any ".."/"." segments before the path crosses the
	// contract boundary: the server receives one canonical relative path.
	relative := path.Clean(strings.TrimPrefix(filepath.ToSlash(localPath), "/"))
	openFile := func() (io.ReadCloser, error) {
		f, err := r.Root.Open(localPath)
		if err != nil {
			return nil, errors.New("path_not_allowed")
		}
		return f, nil
	}
	f, err := openFile()
	if err != nil {
		return nil, err
	}
	file, ok := f.(*os.File)
	if !ok {
		f.Close()
		return nil, errors.New("path_not_allowed")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("path_not_allowed")
	}
	size := info.Size()
	f.Close()
	return r.uploadBounded(ctx, space, relative, size, openFile, conflictPolicy, key)
}

// uploadBounded is the size-independent core of the bounded upload: one
// digest pass, then the batch contracts, then one streaming pass in parts
// of the server-declared size. open must yield a fresh reader over the same
// logical source each call; the scale entry feeds it a synthetic source
// while production feeds the guarded local file.
func (r *RemoteSurfaces) uploadBounded(ctx context.Context, space, relative string, size int64, open func() (io.ReadCloser, error), conflictPolicy, key string) (map[string]any, error) {
	if r.Root == nil {
		return nil, localSourceExplanation()
	}
	if key == "" || len(key) > 200 {
		return nil, errors.New("invalid_request")
	}
	f, err := open()
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	if _, err = io.Copy(digest, f); err != nil {
		f.Close()
		return nil, errors.New("local_file_unavailable")
	}
	f.Close()
	sha := hex.EncodeToString(digest.Sum(nil))
	original := map[string]any{"space_id": space, "conflict_policy": conflictPolicy}
	row, err := r.journalIntent("upload_batch_create", key, remoteFingerprint("upload_batch_create", original), original)
	if err != nil {
		return nil, err
	}
	create, err := r.Run(ctx, "upload_batch_create", map[string]any{
		"space_id": space, "conflict_policy": conflictPolicy, "idempotency_key": key,
	})
	if err != nil {
		// The create request left; the verdict may be unknown. Same-key
		// lookup recovers the original batch instead of creating another,
		// and the recovered receipt settles the intent under the server's
		// original ID.
		lookup, lookupErr := r.LookupOperation(ctx, "upload_batch_create", key, original)
		if lookupErr != nil {
			if row != nil {
				r.settleIntent(row, remoteIntentUnknown, "batch", "", "unknown")
			}
			return nil, err
		}
		create = lookup
		if row != nil {
			if recovered, ok := lookup.(map[string]any); ok {
				if id, ok := recovered["batch_id"].(string); ok && id != "" {
					r.settleIntent(row, remoteIntentConfirmed, "batch", id, "confirmed")
				}
			}
		}
	}
	batch, _ := create.(map[string]any)
	id, _ := batch["batch_id"].(string)
	if id == "" {
		if row != nil {
			r.settleIntent(row, remoteIntentFailed, "batch", "", "unknown")
		}
		return nil, errors.New("invalid_response")
	}
	itemKey := relative + "\x00" + sha
	if _, err = r.Run(ctx, "upload_batch_add_items", map[string]any{
		"batch_id": id, "idempotency_key": key + ":items",
		"items": []map[string]any{{
			"relative_path": relative, "size_bytes": size, "sha256": sha, "item_key": itemKey,
		}},
	}); err != nil {
		return nil, err
	}
	seal, err := r.Run(ctx, "upload_batch_seal", map[string]any{"batch_id": id, "idempotency_key": key + ":seal"})
	if err != nil {
		return nil, err
	}
	sealDoc, _ := seal.(map[string]any)
	operationID, _ := sealDoc["operation_id"].(string)
	var partSize int64 = defaultRemotePartSize
	if v, ok := sealDoc["part_size"].(float64); ok && int64(v) > 0 {
		partSize = int64(v)
	}
	if r.Stream == nil {
		return nil, &RemoteError{Code: "unsupported", Hint: "This adapter has no byte transport configured; upload bytes through the service web/HTTP upload entry."}
	}
	if operationID != "" {
		src, openErr := open()
		if openErr != nil {
			return nil, openErr
		}
		sent, streamErr := r.streamParts(ctx, operationID, partSize, src, size)
		src.Close()
		if streamErr != nil {
			return nil, streamErr
		}
		if sent != size {
			return nil, errors.New("upload_interrupted")
		}
	}
	status, err := r.Run(ctx, "upload_batch_get", map[string]any{"batch_id": id, "limit": float64(remotePageDefault)})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"batch_id": id, "operation_id": operationID, "space_id": space,
		"relative_path": relative, "sha256": sha, "size_bytes": size,
		"bytes_streamed": size, "part_size": partSize,
		"recovery_owner": "server", "status": status,
	}, nil
}

// streamParts uploads total bytes as sequential parts through the public
// byte path using exactly one part-sized buffer. The source is consumed
// once, sequentially: no assembly, no whole-file buffering, so a 100 GiB
// source costs one part of memory regardless of size.
func (r *RemoteSurfaces) streamParts(ctx context.Context, operationID string, partSize int64, src io.Reader, total int64) (int64, error) {
	if partSize <= 0 {
		return 0, errors.New("invalid_request")
	}
	buf := make([]byte, partSize)
	var index, sent int64
	for sent < total {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			chunk := buf[:n]
			if putErr := r.Stream.PutPart(ctx, operationID, index, int64(n), bytes.NewReader(chunk)); putErr != nil {
				return sent, putErr
			}
			sent += int64(n)
			index++
		}
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if n == 0 {
					break
				}
				continue
			}
			return sent, errors.New("upload_interrupted")
		}
	}
	return sent, nil
}

// openRemoteSurfacesDB opens the additive journal store with the same
// pure-Go SQLite driver the owner state uses. The file holds adapter
// transfer state only; dropping it loses the journal, never server-side
// truth.
func openRemoteSurfacesDB(file string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("state_unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err = db.AutoMigrate(&RemoteIntent{}); err != nil {
		sqlDB.Close()
		return nil, errors.New("state_unavailable")
	}
	if err = os.Chmod(file, 0600); err != nil {
		return nil, errors.New("state_unavailable")
	}
	return db, nil
}

// ───────────────────────────── MCP tool surface ─────────────────────────────

// RemoteSurfaceTool is one MCP tool derived from the frozen catalog. The
// design names the client-facing aliases (files_query for analytics_query);
// every tool is registered only when the server actually advertises the
// action, so tools/list never claims a capability the server lacks.
type RemoteSurfaceTool struct {
	Action      string
	Name        string
	Description string
	Required    []string
	Optional    []string
	ReadOnly    bool
}

// remoteSurfaceToolCatalog is the agent-registerable projection of the
// frozen action catalog. Human-only management entries never reach it.
var remoteSurfaceToolCatalog = []RemoteSurfaceTool{
	{Action: "upload_batch_create", Name: "upload_batch_create", Required: []string{"space_id", "idempotency_key"}, Optional: []string{"conflict_policy"}, Description: "Create one remote upload batch on the server; the batch_id is the server's own identity."},
	{Action: "upload_batch_add_items", Name: "upload_batch_add_items", Required: []string{"batch_id", "idempotency_key", "items"}, Optional: []string{"cursor"}, Description: "Append item descriptors (relative path, size, sha256, item_key) to an open batch, at most 500 per page."},
	{Action: "upload_batch_seal", Name: "upload_batch_seal", Required: []string{"batch_id", "idempotency_key"}, Description: "Finish enumeration and start the server-side batch operation; the receipt carries the original operation_id and part_size."},
	{Action: "upload_batch_get", Name: "upload_batch_get", Required: []string{"batch_id"}, Optional: []string{"cursor", "limit"}, ReadOnly: true, Description: "Read one page of batch status with per-item results and the original operation IDs."},
	{Action: "upload_batch_pause", Name: "upload_batch_pause", Required: []string{"batch_id", "idempotency_key"}, Description: "Stop scheduling new items; already verified data is retained under the original batch."},
	{Action: "upload_batch_resume", Name: "upload_batch_resume", Required: []string{"batch_id", "idempotency_key"}, Description: "Resume scheduling the original batch; recovery reads the original operation, never a replacement."},
	{Action: "upload_batch_cancel", Name: "upload_batch_cancel", Required: []string{"batch_id", "idempotency_key"}, Description: "Cancel the batch; committed files are retained."},
	{Action: "upload_batch_file", Name: "upload_batch_file", Required: []string{"space_id", "local_path", "idempotency_key"}, Optional: []string{"conflict_policy"}, Description: "Adapter-local bounded upload of one file inside the configured local root: digest, batch, seal, then stream parts with one reused buffer. Requires this adapter's local byte source."},
	{Action: "operation_lookup", Name: "operation_lookup", Required: []string{"action", "idempotency_key", "original_request"}, ReadOnly: true, Description: "Recover the original server operation after a lost response: same key, same original request."},
	{Action: "analytics_query", Name: "files_query", Required: []string{"dataset", "idempotency_key"}, Optional: []string{"filters", "dimensions", "metrics", "sort", "limit", "cursor", "consistency"}, Description: "Typed analytics query (datasets: file_current, transfer_facts, audit_facts, capacity_daily); no free-form SQL."},
	{Action: "analytics_query_get", Name: "analytics_query_get", Required: []string{"query_id"}, ReadOnly: true, Description: "Read one analytics query result or its async export operation by its original ID."},
	{Action: "analytics_snapshot", Name: "analytics_snapshot", Required: []string{"space_id", "idempotency_key"}, Optional: []string{"scope"}, Description: "Request an explicitly authorized snapshot; the server generates it and decides authorization."},
	{Action: "analytics_snapshot_query", Name: "analytics_snapshot_query", Required: []string{"snapshot_id", "dataset"}, Optional: []string{"filters", "dimensions", "metrics", "sort", "limit", "execution"}, ReadOnly: true, Description: "Typed query against one authorized snapshot; execution=local uses the optional local backend when configured."},
	{Action: "organize_plan", Name: "organize_plan", Required: []string{"idempotency_key"}, Optional: []string{"refs", "actions", "policy_id", "snapshot"}, Description: "Create one organization plan over an exact file set; confirmation stays a human entry."},
	{Action: "organize_plan_get", Name: "organize_plan_get", Required: []string{"plan_id"}, ReadOnly: true, Description: "Read one organization plan: revision, per-item impact, risks and waiting reasons."},
	{Action: "organize_apply", Name: "organize_apply", Required: []string{"plan_id", "expected_revision", "idempotency_key"}, Description: "Execute a confirmed plan under its expected revision; the server revalidates authorization."},
	{Action: "organize_compensation_plan", Name: "organize_compensation_plan", Required: []string{"plan_id", "idempotency_key"}, Description: "Generate the reverse compensation plan; nothing is undone until that plan executes."},
	{Action: "cloud_entries", Name: "cloud_entries", Required: []string{"connection_id"}, Optional: []string{"cursor", "limit", "parent_ref"}, ReadOnly: true, Description: "Page one cloud connection's observed entries by their provider identities."},
	{Action: "cloud_scan", Name: "cloud_scan", Required: []string{"connection_id", "idempotency_key"}, Description: "Start a bounded scan of one already-enabled connection scope; the server owns budget and roots."},
	{Action: "changes_v2", Name: "changes_v2", Optional: []string{"cursor", "space_id", "limit"}, ReadOnly: true, Description: "Read the opaque-cursor change feed; expired cursors report cursor_expired and need an authorized rebuild."},
	{Action: "change_subscription_create", Name: "change_subscription_create", Required: []string{"consumer_id"}, Optional: []string{"space_id"}, Description: "Create one consumer's change subscription inside the authorized scope."},
	{Action: "change_subscription_ack", Name: "change_subscription_ack", Required: []string{"subscription_id", "checkpoint"}, Description: "Acknowledge delivered pages by monotonic checkpoint; ack confirms delivery only."},
	{Action: "automation_intent_get", Name: "automation_intent_get", Required: []string{"intent_id"}, ReadOnly: true, Description: "Read one automation intent: original rule, event, operation and pause reason."},
}

// RemoteSurfaceToolCatalog returns the agent-registerable tool projection of
// the frozen catalog (stable order).
func RemoteSurfaceToolCatalog() []RemoteSurfaceTool {
	out := make([]RemoteSurfaceTool, len(remoteSurfaceToolCatalog))
	copy(out, remoteSurfaceToolCatalog)
	return out
}

// remoteSurfacesStatusToolName is the always-registered read-only discovery
// report for the attached surface: instance, advertised actions, and the
// concrete gap when a catalog action is missing. It never guesses aliases.
const remoteSurfacesStatusToolName = "remote_surfaces_status"

// DiscoverRemoteSurfaceTools filters the tool catalog against the actions the
// server currently advertises. Unadvertised tools are not registered — no
// fake availability — and the missing names are reported so the status tool
// can state the actual gap.
func (r *RemoteSurfaces) DiscoverRemoteSurfaceTools(ctx context.Context) (tools []RemoteSurfaceTool, missing []string, err error) {
	advertised, err := r.Actions(ctx)
	if err != nil {
		return nil, nil, err
	}
	have := make(map[string]bool, len(advertised))
	for _, a := range advertised {
		have[a] = true
	}
	// The adapter-local composite orchestrates the whole batch family; it
	// registers only when every member the flow needs is advertised, and a
	// missing member keeps it off tools/list and inside the status gap.
	batchFamily := map[string]bool{"upload_batch_create": false, "upload_batch_add_items": false, "upload_batch_seal": false, "upload_batch_get": false}
	for a := range have {
		if _, tracked := batchFamily[a]; tracked {
			batchFamily[a] = true
		}
	}
	familyComplete := true
	for _, ok := range batchFamily {
		familyComplete = familyComplete && ok
	}
	for _, tool := range remoteSurfaceToolCatalog {
		if tool.Action == "upload_batch_file" {
			if familyComplete {
				tools = append(tools, tool)
			} else {
				missing = append(missing, tool.Action)
			}
			continue
		}
		if have[tool.Action] {
			tools = append(tools, tool)
		} else {
			missing = append(missing, tool.Action)
		}
	}
	return tools, missing, nil
}

// Status is the read-only discovery report: the attached instance, the live
// advertisement and the concrete catalog gap. After a discovery failure it
// reports the error code instead of inventing an action list.
func (r *RemoteSurfaces) Status(ctx context.Context) map[string]any {
	out := map[string]any{"instance_id": r.Instance, "local_byte_source": r.Root != nil, "local_analytics": r.Analytics != nil}
	actions, err := r.Actions(ctx)
	if err != nil {
		out["discovery"] = "unavailable"
		out["error"] = safeErrorString(err.Error())
		out["catalog"] = RemoteSurfaceActionNames()
		return out
	}
	_, missing, _ := r.DiscoverRemoteSurfaceTools(ctx)
	out["discovery"] = "ok"
	out["actions"] = actions
	out["missing"] = missing
	return out
}

// ───────────────────────────── adapter-local orchestrations ─────────────────────────────

// runRemoteSurfaceLocalAction dispatches one adapter-local orchestration over
// catalog actions. It adds no server-side semantics; it only sequences the
// public contracts (guard → digest → batch → seal → stream) for one bounded
// local source. The second return reports whether the action was local.
func runRemoteSurfaceLocalAction(r *RemoteSurfaces, ctx context.Context, action string, args map[string]any) (any, bool, error) {
	if action != "upload_batch_file" {
		return nil, false, nil
	}
	space, _ := args["space_id"].(string)
	path, _ := args["local_path"].(string)
	policy, _ := args["conflict_policy"].(string)
	key, _ := args["idempotency_key"].(string)
	if policy == "" {
		policy = "fail"
	}
	out, err := r.UploadLocalFile(ctx, space, path, policy, key)
	return out, true, err
}
