package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// The unified cloud-connector surface. Everything here is gated behind
// Config.CloudConnectors: a deployment that has not opted in keeps every
// legacy route and tool behavior and receives "unsupported" on the new
// surface, never a silent fallback.

const (
	cloudPageDefault = 50
	cloudPageMax     = 200
	cloudCursorTTL   = 30 * time.Minute
	// cloudScanMaxBudget bounds one scan's observed entries; a scan that
	// hits the budget reports partial coverage and never derives deletions.
	cloudScanMaxBudget = 100000
)

func cloudPageLimit(raw int) int {
	if raw <= 0 {
		return cloudPageDefault
	}
	if raw > cloudPageMax {
		return cloudPageMax
	}
	return raw
}

// cloudGate is the shared capability switch for the whole surface.
func (s *Service) cloudGate() error {
	if !s.Config.CloudConnectors {
		return code("unsupported")
	}
	return nil
}

// cloudCursorPayload is the sealed body of one opaque listing cursor. The
// cursor binds the root, the caller's data owner, the canonical query and
// the provider continuation token; any mismatch is cursor_expired rather
// than a fresh enumeration. Liveness is bounded by ExpiresAt alone (the
// TTL): a root rebind or permission narrowing must mint a new enumeration
// through the root's own lifecycle, and callers treat 410 as the signal
// to re-establish the baseline.
type cloudCursorPayload struct {
	Root      string `json:"root"`
	Owner     string `json:"owner"`
	Query     string `json:"query"`
	Token     string `json:"token,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	Kind      string `json:"kind"` // list|changes
}

func (s *Service) sealCloudCursor(payload cloudCursorPayload) (string, error) {
	body, e := json.Marshal(payload)
	if e != nil {
		return "", code("invalid_request")
	}
	nonce, e := secureRandomBytes(s.crypt.NonceSize())
	if e != nil {
		return "", code("encryption_unavailable")
	}
	sealed := s.crypt.Seal(nonce, nonce, body, []byte("cloud-cursor"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) openCloudCursor(cursor, kind, root, owner, query string) (cloudCursorPayload, error) {
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if e != nil || len(raw) < s.crypt.NonceSize() {
		return cloudCursorPayload{}, code("cursor_expired")
	}
	plain, e := s.crypt.Open(nil, raw[:s.crypt.NonceSize()], raw[s.crypt.NonceSize():], []byte("cloud-cursor"))
	if e != nil {
		return cloudCursorPayload{}, code("cursor_expired")
	}
	var payload cloudCursorPayload
	if json.Unmarshal(plain, &payload) != nil {
		return cloudCursorPayload{}, code("cursor_expired")
	}
	if payload.Kind != kind || payload.Root != root || payload.Owner != owner || payload.Query != query {
		return cloudCursorPayload{}, code("cursor_expired")
	}
	if time.Now().UTC().Unix() > payload.ExpiresAt {
		return cloudCursorPayload{}, code("cursor_expired")
	}
	return payload, nil
}

// encodeEntryRef builds the self-contained entry reference from a root and
// provider item id (S3 keys may contain separators, so the pair is packed).
func encodeEntryRef(rootID, providerItemID string) string {
	body, _ := json.Marshal(map[string]string{"root": rootID, "item": providerItemID})
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeEntryRef(ref string) (string, string, error) {
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ref))
	if e != nil {
		return "", "", code("invalid_request")
	}
	var pair map[string]string
	if json.Unmarshal(raw, &pair) != nil || pair["root"] == "" || pair["item"] == "" {
		return "", "", code("invalid_request")
	}
	return pair["root"], pair["item"], nil
}

// gdriveConnection loads and unseals one Google Drive connection in the
// caller's scope. Agents may only touch the connection they are bound to.
func (s *Service) gdriveConnection(p Principal, id string) (Connection, gdriveCredential, error) {
	var c Connection
	var v gdriveCredential
	if s.DB.First(&c, "id = ? AND owner = ? AND kind = ? AND disabled = ?", id, p.dataOwner(), "gdrive", false).Error != nil {
		return c, v, code("storage_disconnected")
	}
	if p.Agent != nil && p.Agent.ConnectionID != id {
		return c, v, code("forbidden")
	}
	e := s.unseal(p.dataOwner(), c.Secret, &v)
	return c, v, e
}

// BeginGoogleDriveConnection starts the human OAuth flow. Agents can never
// begin, extend or re-identity a connection.
func (s *Service) BeginGoogleDriveConnection(w http.ResponseWriter, r *http.Request, p Principal) error {
	if e := s.cloudGate(); e != nil {
		return e
	}
	if p.Agent != nil {
		return code("forbidden")
	}
	if s.Config.GoogleClientID == "" {
		return code("gdrive_not_configured")
	}
	state, verifier := ID(), oauth2.GenerateVerifier()
	secret, _ := s.seal(p.dataOwner(), flowSecret{Verifier: verifier})
	flow := Flow{Hash: Hash(state), Owner: p.dataOwner(), Purpose: "gdrive", Secret: secret, Expires: time.Now().Add(10 * time.Minute)}
	if s.DB.Create(&flow).Error != nil {
		return code("database_unavailable")
	}
	s.cookie(w, "db_flow", state, 600)
	oauth := s.gdriveOAuth()
	respond(w, map[string]string{"authorization_url": oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))})
	return nil
}

// CloudRootView is the root read model shared by HTTP, MCP and SDK.
type CloudRootView struct {
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

func cloudRootView(r CloudRoot) CloudRootView {
	return CloudRootView{RootID: r.ID, ConnectionID: r.ConnectionID, Provider: r.Provider, RootRef: r.RootRef, Name: r.Name, Status: r.Status, Generation: r.Generation, LastScanAt: r.LastScanAt, LastScanCoverage: r.LastScanCoverage, LastScanObserved: r.LastScanObserved, LastScanTombstoned: r.LastScanTombstoned}
}

// BindCloudRoot binds one provider-native root reference (drive item id,
// folder id or validated object prefix) to the connection. Root binding is
// a human action: agents consume roots, they never create or widen them.
func (s *Service) BindCloudRoot(p Principal, connectionID, rootRef, name string) (CloudRootView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudRootView{}, e
	}
	if p.Agent != nil {
		return CloudRootView{}, code("forbidden")
	}
	if len(rootRef) > 512 || len(name) > 200 || strings.ContainsRune(rootRef, 0) {
		return CloudRootView{}, code("invalid_request")
	}
	var conn Connection
	if s.DB.First(&conn, "id = ? AND owner = ? AND disabled = ?", connectionID, p.dataOwner(), false).Error != nil {
		return CloudRootView{}, code("not_found")
	}
	adapter, e := s.cloudAdapterFor(conn.Kind)
	if e != nil {
		return CloudRootView{}, e
	}
	if conn.Kind == "s3" {
		// Prefix roots are validated against path rules; arbitrary bucket
		// names or URLs never reach the provider.
		if rootRef != "" {
			cleaned, e := cleanPath(rootRef)
			if e != nil || cleaned != rootRef {
				return CloudRootView{}, code("invalid_request")
			}
		}
	}
	now := time.Now().UTC()
	root := CloudRoot{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, ConnectionID: conn.ID, Provider: conn.Kind, RootRef: rootRef, Name: name, Status: "bound", CreatedAt: now, UpdatedAt: now}
	if s.DB.Create(&root).Error != nil {
		return CloudRootView{}, code("database_unavailable")
	}
	_ = adapter
	return cloudRootView(root), nil
}

// ListCloudRoots returns the caller's roots for one connection.
func (s *Service) ListCloudRoots(p Principal, connectionID string) ([]CloudRootView, error) {
	if e := s.cloudGate(); e != nil {
		return nil, e
	}
	var conn Connection
	if s.DB.First(&conn, "id = ? AND owner = ?", connectionID, p.dataOwner()).Error != nil {
		return nil, code("not_found")
	}
	if p.Agent != nil {
		return nil, code("forbidden")
	}
	var roots []CloudRoot
	if s.DB.Where("connection_id = ? AND owner = ?", connectionID, p.dataOwner()).Order("created_at").Find(&roots).Error != nil {
		return nil, code("database_unavailable")
	}
	out := make([]CloudRootView, 0, len(roots))
	for _, r := range roots {
		out = append(out, cloudRootView(r))
	}
	return out, nil
}

// cloudContext loads connection, root and adapter with scope checks. Every
// page re-authorizes: the root must still belong to the caller's data owner
// and the agent (if any) must stay bound to this connection.
func (s *Service) cloudContext(p Principal, connectionID, rootID string) (Connection, CloudRoot, cloudAdapter, error) {
	var conn Connection
	if s.DB.First(&conn, "id = ? AND owner = ? AND disabled = ?", connectionID, p.dataOwner(), false).Error != nil {
		return conn, CloudRoot{}, nil, code("not_found")
	}
	if p.Agent != nil && p.Agent.ConnectionID != connectionID {
		return conn, CloudRoot{}, nil, code("forbidden")
	}
	var root CloudRoot
	if s.DB.First(&root, "id = ? AND connection_id = ? AND owner = ?", rootID, connectionID, p.dataOwner()).Error != nil {
		return conn, root, nil, code("not_found")
	}
	adapter, e := s.cloudAdapterFor(conn.Kind)
	if e != nil {
		return conn, root, nil, e
	}
	return conn, root, adapter, nil
}

// ExternalEntryView is the observed entry payload. Field names match the
// design contract exactly: they are the public surface consumed by the
// client and MCP mocks.
type ExternalEntryView struct {
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

type CloudEntriesView struct {
	Items      []ExternalEntryView `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Coverage   string              `json:"coverage"`
	ObservedAt time.Time           `json:"observed_at"`
}

func cloudEntryView(root CloudRoot, e ExternalEntry, instance string) ExternalEntryView {
	view := ExternalEntryView{EntryRef: encodeEntryRef(e.RootID, e.ProviderItemID), ProviderItemID: e.ProviderItemID, RootID: e.RootID, ParentRef: e.ParentRef, Path: e.Path, Kind: e.Kind, SizeBytes: e.Size, MIME: e.MIME, ModifiedAt: e.ModifiedAt, ObservedRevision: e.ObservedRevision, VersionStrength: e.VersionStrength, ContentSHA256: e.ContentDigest, Tombstoned: e.TombstonedAt != nil, ObservedAt: e.ObservedAt}
	if e.ManagedFileID != "" && e.ManagedVersionID != "" {
		view.ManagedRef = "drivebridge://" + instance + "/file/" + e.ManagedFileID + "/version/" + e.ManagedVersionID
	}
	return view
}

// observeEntry persists one provider observation. Provider identity is the
// (root, item) pair: a repeated item after drift updates the same row, and
// a tombstone only clears when the provider shows the item again.
func (s *Service) observeEntry(conn Connection, root CloudRoot, obs cloudObservation, now time.Time) (ExternalEntry, error) {
	var existing ExternalEntry
	e := s.DB.Where("root_id = ? AND provider_item_id = ?", root.ID, obs.ProviderItemID).First(&existing).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		entry := ExternalEntry{ID: ID(), RootID: root.ID, ProviderItemID: obs.ProviderItemID, Owner: root.Owner, ConnectionID: conn.ID, ParentRef: obs.ParentRef, Path: obs.Path, Kind: obs.Kind, Size: obs.Size, MIME: obs.MIME, ModifiedAt: obs.ModifiedAt, ObservedRevision: obs.Revision, VersionStrength: "observed_only", ObservedAt: now, CreatedAt: now, UpdatedAt: now}
		if err := s.DB.Create(&entry).Error; err != nil {
			return entry, code("database_unavailable")
		}
		return entry, nil
	}
	if e != nil {
		return existing, code("database_unavailable")
	}
	updates := map[string]any{
		"parent_ref":        obs.ParentRef,
		"path":              obs.Path,
		"kind":              obs.Kind,
		"size_bytes":        obs.Size,
		"mime_type":         obs.MIME,
		"observed_revision": obs.Revision,
		"observed_at":       now,
		"updated_at":        now,
	}
	if obs.ModifiedAt != nil {
		// A nil provider timestamp clears nothing: the column keeps the
		// last known fact instead of guessing an update time.
		updates["modified_at"] = obs.ModifiedAt
	}
	if existing.TombstonedAt != nil {
		// The provider shows the item again: clear the confirmed-deletion
		// tombstone and drop any stale managed link from the deleted era.
		updates["tombstoned_at"] = nil
		updates["managed_file_id"] = ""
		updates["managed_version_id"] = ""
		updates["version_strength"] = "observed_only"
		updates["content_sha256"] = ""
	}
	// A revision change invalidates any previously verified digest link:
	// verified bytes are facts about one revision only.
	if existing.ObservedRevision != obs.Revision {
		updates["managed_file_id"] = ""
		updates["managed_version_id"] = ""
		updates["version_strength"] = "observed_only"
		updates["content_sha256"] = ""
	}
	if err := s.DB.Model(&ExternalEntry{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return existing, code("database_unavailable")
	}
	if err := s.DB.First(&existing, "id = ?", existing.ID).Error; err != nil {
		return existing, code("database_unavailable")
	}
	return existing, nil
}

// ListCloudEntries returns one bounded page of provider observations under
// the bound root. Coverage is partial whenever the provider enumeration did
// not complete; absence from a page is never deletion evidence.
func (s *Service) ListCloudEntries(ctx context.Context, p Principal, connectionID, rootID, parentRef, cursor string, limit int) (CloudEntriesView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudEntriesView{}, e
	}
	conn, root, adapter, e := s.cloudContext(p, connectionID, rootID)
	if e != nil {
		return CloudEntriesView{}, e
	}
	limit = cloudPageLimit(limit)
	query := cloudQueryFingerprint(parentRef, limit)
	token := ""
	if cursor != "" {
		payload, e := s.openCloudCursor(cursor, "list", root.ID, root.Owner, query)
		if e != nil {
			return CloudEntriesView{}, e
		}
		token = payload.Token
	}
	page, e := adapter.ListChildren(ctx, p, conn, root, parentRef, token, limit)
	if e != nil {
		return CloudEntriesView{}, e
	}
	now := time.Now().UTC()
	items := make([]ExternalEntryView, 0, len(page.Items))
	seen := map[string]bool{}
	for _, obs := range page.Items {
		// Duplicate provider identities inside one page collapse to one
		// observation instead of duplicating rows.
		if seen[obs.ProviderItemID] {
			continue
		}
		seen[obs.ProviderItemID] = true
		entry, e := s.observeEntry(conn, root, obs, now)
		if e != nil {
			return CloudEntriesView{}, e
		}
		items = append(items, cloudEntryView(root, entry, s.Instance))
	}
	view := CloudEntriesView{Items: items, Coverage: "complete", ObservedAt: now}
	if page.NextToken != "" {
		payload := cloudCursorPayload{Root: root.ID, Owner: root.Owner, Query: query, Token: page.NextToken, ExpiresAt: now.Add(cloudCursorTTL).Unix(), Kind: "list"}
		next, e := s.sealCloudCursor(payload)
		if e != nil {
			return CloudEntriesView{}, e
		}
		view.NextCursor = next
		view.Coverage = "partial"
		s.touchRoot(root.ID, "observed", nil, "", 0, 0, false)
	} else {
		s.touchRoot(root.ID, "observed", nil, "", 0, 0, false)
	}
	return view, nil
}

// cloudQueryFingerprint canonicalizes the page query so a cursor can never
// be replayed against different conditions.
func cloudQueryFingerprint(parentRef string, limit int) string {
	return Hash(parentRef + "\x00" + strconv.Itoa(limit))
}

// touchRoot updates root observation state. Scan facts only land through a
// dedicated scan call; listing refresh keeps status and timestamps.
func (s *Service) touchRoot(rootID, status string, scanAt *time.Time, coverage string, observed, tombstoned int64, bumpGeneration bool) {
	updates := map[string]any{"status": status, "updated_at": time.Now().UTC()}
	if scanAt != nil {
		updates["last_scan_at"] = scanAt
		updates["last_scan_coverage"] = coverage
		updates["last_scan_observed"] = observed
		updates["last_scan_tombstoned"] = tombstoned
	}
	if bumpGeneration {
		updates["generation"] = gorm.Expr("generation + 1")
	}
	s.DB.Model(&CloudRoot{}).Where("id = ?", rootID).Updates(updates)
}

// StatCloudEntry checks one entry against the live provider under the
// current authorization. A supplied expected_revision that no longer
// matches is source_changed: the caller refreshes the observation and
// decides a new intent; the original one never switches sources.
func (s *Service) StatCloudEntry(ctx context.Context, p Principal, connectionID, entryRef, expectedRevision string) (ExternalEntryView, error) {
	if e := s.cloudGate(); e != nil {
		return ExternalEntryView{}, e
	}
	rootID, providerItemID, e := decodeEntryRef(entryRef)
	if e != nil {
		return ExternalEntryView{}, e
	}
	conn, root, adapter, e := s.cloudContext(p, connectionID, rootID)
	if e != nil {
		return ExternalEntryView{}, e
	}
	obs, e := adapter.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil {
		return ExternalEntryView{}, e
	}
	if expectedRevision != "" && expectedRevision != obs.Revision {
		return ExternalEntryView{}, code("source_changed")
	}
	now := time.Now().UTC()
	entry, e := s.observeEntry(conn, root, obs, now)
	if e != nil {
		return ExternalEntryView{}, e
	}
	return cloudEntryView(root, entry, s.Instance), nil
}

// CloudChangesView is one bounded change page. Removed lists provider-
// confirmed removals only; coverage stays partial while the feed has more
// changes pending.
type CloudChangesView struct {
	Items      []ExternalEntryView `json:"items"`
	Removed    []string            `json:"removed,omitempty"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Complete   bool                `json:"complete"`
	ObservedAt time.Time           `json:"observed_at"`
}

// ReadCloudChanges advances one root's provider change feed. Providers
// without a native cursor (S3) reject the call with unsupported and change
// detection falls back to a budgeted scan on the original root.
func (s *Service) ReadCloudChanges(ctx context.Context, p Principal, connectionID, rootID, cursor string, limit int) (CloudChangesView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudChangesView{}, e
	}
	conn, root, adapter, e := s.cloudContext(p, connectionID, rootID)
	if e != nil {
		return CloudChangesView{}, e
	}
	limit = cloudPageLimit(limit)
	query := "changes"
	token := ""
	if cursor != "" {
		payload, e := s.openCloudCursor(cursor, "changes", root.ID, root.Owner, query)
		if e != nil {
			return CloudChangesView{}, e
		}
		token = payload.Token
	}
	page, e := adapter.ReadChanges(ctx, p, conn, root, token, limit)
	if e != nil {
		return CloudChangesView{}, e
	}
	now := time.Now().UTC()
	view := CloudChangesView{Items: []ExternalEntryView{}, Removed: page.Removed, Complete: !page.Delta, ObservedAt: now}
	for _, obs := range page.Items {
		entry, e := s.observeEntry(conn, root, obs, now)
		if e != nil {
			return CloudChangesView{}, e
		}
		view.Items = append(view.Items, cloudEntryView(root, entry, s.Instance))
	}
	for _, removed := range page.Removed {
		// Provider-confirmed removal: tombstone and invalidate the old
		// observation, including any managed link from the deleted era.
		s.DB.Model(&ExternalEntry{}).Where("root_id = ? AND provider_item_id = ?", root.ID, removed).Updates(map[string]any{"tombstoned_at": now, "managed_file_id": "", "managed_version_id": "", "version_strength": "observed_only", "content_sha256": "", "updated_at": now})
	}
	if page.NextCursor != "" {
		payload := cloudCursorPayload{Root: root.ID, Owner: root.Owner, Query: query, Token: page.NextCursor, ExpiresAt: now.Add(24 * time.Hour).Unix(), Kind: "changes"}
		next, e := s.sealCloudCursor(payload)
		if e != nil {
			return CloudChangesView{}, e
		}
		view.NextCursor = next
		s.DB.Model(&CloudRoot{}).Where("id = ?", root.ID).Updates(map[string]any{"delta_cursor": page.NextCursor, "updated_at": now})
	}
	return view, nil
}

// CloudScanRequest is one bounded scan intent.
type CloudScanRequest struct {
	RootID string `json:"root_id"`
	Budget int64  `json:"budget,omitempty"`
}

// CloudScanView reports the scan outcome on the original Operation.
type CloudScanView struct {
	OperationID string    `json:"operation_id"`
	RootID      string    `json:"root_id"`
	Status      string    `json:"status"`
	Coverage    string    `json:"coverage"`
	Observed    int64     `json:"observed"`
	Tombstoned  int64     `json:"tombstoned"`
	Generation  uint64    `json:"generation"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScanCloudConnection runs one bounded enumeration of the root. Scans are
// budgeted by owner policy: hitting the budget ends with partial coverage,
// and a partial or failed scan never derives deletions. Only a scan that
// enumerated the complete scope may tombstone entries the provider no
// longer shows.
func (s *Service) ScanCloudConnection(ctx context.Context, p Principal, connectionID string, q CloudScanRequest, key string) (CloudScanView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudScanView{}, e
	}
	if q.Budget <= 0 {
		q.Budget = 1000
	}
	if q.Budget > cloudScanMaxBudget {
		return CloudScanView{}, code("invalid_request")
	}
	conn, root, adapter, e := s.cloudContext(p, connectionID, q.RootID)
	if e != nil {
		return CloudScanView{}, e
	}
	if key == "" || len(key) > 200 {
		return CloudScanView{}, code("invalid_request")
	}
	if p.Agent != nil {
		// Agents may only rescan a scope their connection already covers;
		// the budget stays owner-bounded either way.
		if e := s.allow(p, p.Agent.SpaceID, p.Agent.Prefix, "read"); e != nil {
			return CloudScanView{}, e
		}
	}
	// The fingerprint normalization lives in one helper shared with
	// LookupOperation: recovery recomputes exactly what creation computed.
	fingerprint := cloudScanFingerprint(conn.ID, root.ID, q.Budget)
	// The idempotency transaction serializes on s.mu; the enumeration
	// itself runs outside the critical section so a long bounded scan
	// never blocks every other writer on this instance (the upload
	// export path follows the same release-after-reserve pattern).
	s.mu.Lock()
	var op Operation
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&op, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			if op.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		agent := ""
		if p.Agent != nil {
			agent = p.Agent.ID
		}
		op = Operation{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), AgentID: agent, Key: key, Fingerprint: fingerprint, Kind: "cloud_scan", SpaceID: "", Path: "", Status: "running", Updated: time.Now().UTC()}
		if err := tx.Create(&op).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	s.mu.Unlock()
	if e != nil {
		return CloudScanView{}, e
	}
	if op.Status == "completed" {
		return s.cloudScanView(op, root)
	}
	// Run the bounded enumeration. Failures keep partial results and mark
	// the operation failed without tombstoning anything.
	view, e := s.runCloudScan(ctx, p, conn, root, adapter, op, q.Budget)
	if e != nil {
		s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "failed", "error": e.Error(), "updated_at": time.Now().UTC()})
		return CloudScanView{}, e
	}
	return view, nil
}

// cloudScanView rebuilds the durable scan outcome for a replayed scan
// operation: the facts live on the root, the operation stays the anchor.
func (s *Service) cloudScanView(op Operation, root CloudRoot) (CloudScanView, error) {
	var fresh CloudRoot
	if s.DB.First(&fresh, "id = ?", root.ID).Error != nil {
		return CloudScanView{}, code("not_found")
	}
	return CloudScanView{OperationID: op.ID, RootID: fresh.ID, Status: op.Status, Coverage: fresh.LastScanCoverage, Observed: fresh.LastScanObserved, Tombstoned: fresh.LastScanTombstoned, Generation: fresh.Generation, UpdatedAt: fresh.UpdatedAt}, nil
}

// cloudScanFingerprint is the canonical normalization of one bounded scan
// intent: the single definition shared by ScanCloudConnection and
// LookupOperation so a lost creation response replays onto the same scan.
func cloudScanFingerprint(connectionID, rootID string, budget int64) string {
	fingerprintSource := struct {
		Connection string `json:"connection"`
		Root       string `json:"root"`
		Budget     int64  `json:"budget"`
	}{connectionID, rootID, budget}
	body, _ := json.Marshal(fingerprintSource)
	return Hash(string(body))
}

// runCloudScan enumerates the whole root tree within the budget. The walk
// is breadth-first over provider directories and memory-bounded: pages
// stream into observations and only counters plus the pending directory
// queue stay in memory. Coverage is complete only when the queue drained
// without hitting the budget; anything else is partial and derives no
// deletions.
func (s *Service) runCloudScan(ctx context.Context, p Principal, conn Connection, root CloudRoot, adapter cloudAdapter, op Operation, budget int64) (CloudScanView, error) {
	scanStart := time.Now().UTC()
	queue := []string{""}
	observed := int64(0)
	complete := true
	for len(queue) > 0 && observed < budget {
		parent := queue[0]
		queue = queue[1:]
		token := ""
		for {
			page, e := adapter.ListChildren(ctx, p, conn, root, parent, token, cloudPageMax)
			if e != nil {
				return CloudScanView{}, e
			}
			for _, obs := range page.Items {
				if observed >= budget {
					complete = false
					break
				}
				if _, e := s.observeEntry(conn, root, obs, scanStart); e != nil {
					return CloudScanView{}, e
				}
				observed++
				if obs.Kind == "directory" {
					queue = append(queue, obs.ProviderItemID)
				}
			}
			s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"progress_files": int(observed), "total_files": int(budget), "updated_at": time.Now().UTC()})
			if observed >= budget {
				// The budget is spent: stop issuing provider pages too,
				// not just stop consuming their items.
				complete = false
				break
			}
			if page.NextToken == "" {
				break
			}
			token = page.NextToken
		}
	}
	if observed >= budget && len(queue) > 0 {
		complete = false
	}
	coverage := "partial"
	if complete {
		coverage = "complete"
	}
	tombstoned := int64(0)
	if complete {
		// Only a complete enumeration is deletion evidence: entries not
		// observed during this scan era are confirmed absent at the
		// provider and tombstoned now, invalidating their observations.
		res := s.DB.Model(&ExternalEntry{}).Where("root_id = ? AND tombstoned_at IS NULL AND observed_at < ?", root.ID, scanStart).Updates(map[string]any{"tombstoned_at": scanStart, "managed_file_id": "", "managed_version_id": "", "version_strength": "observed_only", "content_sha256": "", "updated_at": scanStart})
		if res.Error != nil {
			return CloudScanView{}, code("database_unavailable")
		}
		tombstoned = res.RowsAffected
	}
	s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "completed", "updated_at": time.Now().UTC()})
	s.touchRoot(root.ID, "observed", &scanStart, coverage, observed, tombstoned, false)
	return CloudScanView{OperationID: op.ID, RootID: root.ID, Status: "completed", Coverage: coverage, Observed: observed, Tombstoned: tombstoned, Generation: root.Generation, UpdatedAt: scanStart}, nil
}

// ImportCloudEntryRequest is the explicit import intent: a fixed
// observation becomes a managed copy under the caller's write scope.
type ImportCloudEntryRequest struct {
	EntryRef         string `json:"entry_ref"`
	ExpectedRevision string `json:"expected_revision,omitempty"`
	SpaceID          string `json:"space_id"`
	Path             string `json:"path"`
}

// ImportCloudEntry downloads one pinned observation, verifies its digest
// and publishes a managed file version linked to the external source. The
// observation is re-checked before and after the byte transfer: a source
// that drifted mid-read fails with source_changed and the old observation
// is never presented as the saved version.
func (s *Service) ImportCloudEntry(ctx context.Context, p Principal, connectionID string, q ImportCloudEntryRequest, key string) (Operation, error) {
	if e := s.cloudGate(); e != nil {
		return Operation{}, e
	}
	rootID, providerItemID, e := decodeEntryRef(q.EntryRef)
	if e != nil {
		return Operation{}, e
	}
	conn, root, adapter, e := s.cloudContext(p, connectionID, rootID)
	if e != nil {
		return Operation{}, e
	}
	if e := s.allow(p, q.SpaceID, q.Path, "import"); e != nil {
		return Operation{}, e
	}
	obs, e := adapter.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil {
		return Operation{}, e
	}
	if obs.Kind != "file" {
		if obs.Kind == "native_document" {
			return Operation{}, code("export_required")
		}
		return Operation{}, code("invalid_request")
	}
	if q.ExpectedRevision != "" && q.ExpectedRevision != obs.Revision {
		return Operation{}, code("source_changed")
	}
	pinned := obs.Revision
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: q.SpaceID, Path: q.Path, Size: obs.Size, SHA256: Hash(conn.ID + providerItemID + pinned)}, key)
	if e != nil {
		return op, e
	}
	if op.Status == "completed" {
		return op, nil
	}
	updates := map[string]any{"kind": "import", "source_id": providerItemID, "source_version": pinned, "external_connection_id": conn.ID}
	if e := s.DB.Model(&Operation{}).Where("id = ? AND status <> ?", op.ID, "completed").Updates(updates).Error; e != nil {
		return op, code("database_unavailable")
	}
	op.Kind = "import"
	op.SourceID = providerItemID
	op.SourceVersion = pinned
	op.ExternalConnectionID = conn.ID
	return s.runCloudImport(ctx, p, conn, root, adapter, op, providerItemID, pinned)
}

// runCloudImport streams the pinned external bytes through the existing
// part staging and completion machinery. Replays land on the original
// operation identity; a mid-read drift keeps the managed copy unpublished.
func (s *Service) runCloudImport(ctx context.Context, p Principal, conn Connection, root CloudRoot, adapter cloudAdapter, op Operation, providerItemID, pinned string) (result Operation, failure error) {
	defer func() { s.failedOperation(op.ID, failure) }()
	hash := sha256.Sum256([]byte(op.ID))
	lock := &s.sourceLocks[int(hash[0])%len(s.sourceLocks)]
	lock.Lock()
	defer lock.Unlock()
	ctx, finish := s.operationContext(ctx, op.ID)
	defer finish()
	current, e := s.operation(p, op.ID, "write")
	if e != nil {
		return op, e
	}
	if current.Status == "completed" || current.Status == "cancelled" {
		return current, nil
	}
	if e := s.allow(p, op.SpaceID, op.Path, "import"); e != nil {
		return op, e
	}
	fetch, e := adapter.FetchItem(ctx, p, conn, root, providerItemID, pinned)
	if e != nil {
		return op, e
	}
	defer fetch.Body.Close()
	h := sha256.New()
	for i := 0; int64(i)*PartSize < op.Size; i++ {
		size := min(PartSize, op.Size-int64(i)*PartSize)
		part := make([]byte, int(size))
		if _, e = io.ReadFull(fetch.Body, part); e != nil {
			return op, code("incomplete_part")
		}
		h.Write(part)
		if _, e = s.PutPart(p, op.ID, i, bytes.NewReader(part)); e != nil {
			return op, e
		}
	}
	one := make([]byte, 1)
	if n, _ := fetch.Body.Read(one); n != 0 {
		return op, code("content_mismatch")
	}
	// Post-read revision check: the source must still sit on the pinned
	// revision, otherwise the bytes just read are not the observation the
	// caller imported.
	after, e := adapter.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil || after.Revision != pinned {
		return op, code("source_changed")
	}
	digest := hex.EncodeToString(h.Sum(nil))
	s.DB.Model(&Operation{}).Where("id = ?", op.ID).Update("sha256", digest)
	completed, e := s.Complete(ctx, p, op.ID)
	if e != nil {
		return completed, e
	}
	if completed.Status == "completed" {
		now := time.Now().UTC()
		// Link the observation to the managed copy and record the verified
		// digest for this exact revision.
		s.DB.Model(&ExternalEntry{}).Where("root_id = ? AND provider_item_id = ? AND observed_revision = ?", root.ID, providerItemID, pinned).Updates(map[string]any{"managed_file_id": completed.FileID, "managed_version_id": completed.VersionID, "version_strength": "sha256_verified", "content_sha256": digest, "updated_at": now})
	}
	return completed, nil
}

// CloudCapabilities reports the per-connection provider matrix. The values
// are independent facts, never inferred from the provider's name.
func (s *Service) CloudCapabilities(ctx context.Context, p Principal, connectionID string) (CloudCapabilityView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudCapabilityView{}, e
	}
	var conn Connection
	if s.DB.First(&conn, "id = ? AND owner = ? AND disabled = ?", connectionID, p.dataOwner(), false).Error != nil {
		return CloudCapabilityView{}, code("not_found")
	}
	if p.Agent != nil && p.Agent.ConnectionID != connectionID {
		return CloudCapabilityView{}, code("forbidden")
	}
	adapter, e := s.cloudAdapterFor(conn.Kind)
	if e != nil {
		return CloudCapabilityView{}, e
	}
	return adapter.Capabilities(ctx, p, conn), nil
}

// DisconnectCloudRoot marks a root disconnected and invalidates every
// outstanding cursor by bumping the generation. Human-only.
func (s *Service) DisconnectCloudRoot(p Principal, connectionID, rootID string) (CloudRootView, error) {
	if e := s.cloudGate(); e != nil {
		return CloudRootView{}, e
	}
	if p.Agent != nil {
		return CloudRootView{}, code("forbidden")
	}
	_, root, _, e := s.cloudContext(p, connectionID, rootID)
	if e != nil {
		return CloudRootView{}, e
	}
	s.touchRoot(root.ID, "disconnected", nil, "", 0, 0, true)
	var fresh CloudRoot
	if s.DB.First(&fresh, "id = ?", root.ID).Error != nil {
		return CloudRootView{}, code("not_found")
	}
	return cloudRootView(fresh), nil
}
