package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

// remoteStubTransport records every JSON call and answers from a scripted
// route table. It is the contract fixture for adapter-level tests; the HTTP
// shape of the real contract is covered by the httptest fixture server in
// the consumption tests.
type remoteStubTransport struct {
	mu     sync.Mutex
	calls  []remoteStubCall
	routes map[string]remoteStubHandler
}

type remoteStubCall struct {
	Method string
	Path   string
	Key    string
	Body   map[string]any
}

type remoteStubHandler func(call remoteStubCall) (map[string]any, error)

func (s *remoteStubTransport) JSON(ctx context.Context, method, path, key string, body, out any) error {
	s.mu.Lock()
	decoded := map[string]any{}
	if body != nil {
		if b, err := json.Marshal(body); err == nil {
			json.Unmarshal(b, &decoded)
		}
	}
	s.calls = append(s.calls, remoteStubCall{Method: method, Path: path, Key: key, Body: decoded})
	handler, ok := s.routes[method+" "+strings.SplitN(path, "?", 2)[0]]
	s.mu.Unlock()
	if !ok {
		return errors.New("service_unavailable")
	}
	doc, err := handler(remoteStubCall{Method: method, Path: path, Key: key, Body: decoded})
	if err != nil {
		return err
	}
	if out != nil {
		b, _ := json.Marshal(doc)
		return json.Unmarshal(b, out)
	}
	return nil
}

func (s *remoteStubTransport) recorded() []remoteStubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]remoteStubCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// surfacesFixture builds a RemoteSurfaces bound to one stub server instance
// and the passed journal database (nil keeps the thin no-journal mode).
func surfacesFixture(t *testing.T, db *gorm.DB, routes map[string]remoteStubHandler) (*RemoteSurfaces, *remoteStubTransport) {
	t.Helper()
	stub := &remoteStubTransport{routes: routes}
	return NewRemoteSurfaces("https://server.example", stub, nil, db, nil), stub
}

// TestFPRemoteSurfacesIntentJournalKeepsIdentityAndIdempotency pins the
// pre-request journal: the intent row is created before the request leaves,
// a same-key same-fingerprint replay reuses the row, and a same-key
// different-fingerprint attempt is refused locally without issuing anything.
func TestFPRemoteSurfacesIntentJournalKeepsIdentityAndIdempotency(t *testing.T) {
	db := surfacesDB(t)
	created := map[string]string{}
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			if id, ok := created[call.Key]; ok {
				return map[string]any{"batch_id": id, "reused": true}, nil
			}
			id := "batch-" + call.Key
			created[call.Key] = id
			return map[string]any{"batch_id": id}, nil
		},
	}
	s, stub := surfacesFixture(t, db, routes)
	first, err := s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "conflict_policy": "fail", "idempotency_key": "k1"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.(map[string]any)["batch_id"] != "batch-k1" {
		t.Fatalf("first create receipt: %v", first)
	}
	var rows []RemoteIntent
	if err := db.Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("journal rows: %d err=%v", len(rows), err)
	}
	row := rows[0]
	if row.Instance != "https://server.example" || row.Action != "upload_batch_create" || row.Key != "k1" {
		t.Fatalf("journal identity: %+v", row)
	}
	if row.RemoteRef != "batch-k1" || row.RefKind != "batch" || row.Phase != remoteIntentConfirmed {
		t.Fatalf("journal settle: %+v", row)
	}
	// Same key, same fingerprint replays through the same row.
	if _, err = s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "conflict_policy": "fail", "idempotency_key": "k1"}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if err := db.Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("replay must not add rows: %d err=%v", len(rows), err)
	}
	// Same key, different fingerprint is refused before any request leaves.
	before := len(stub.recorded())
	_, err = s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "other", "conflict_policy": "fail", "idempotency_key": "k1"})
	if err == nil || err.Error() != "idempotency_conflict" {
		t.Fatalf("divergent fingerprint: %v", err)
	}
	if got := len(stub.recorded()); got != before {
		t.Fatalf("divergent fingerprint must not issue: %d calls (was %d)", got, before)
	}
	// The database-level uniqueness backs the application check.
	dup := RemoteIntent{Instance: row.Instance, Action: row.Action, Key: row.Key, Fingerprint: row.Fingerprint, Request: "{}", Phase: remoteIntentIssued}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("duplicate scope must fail the unique index")
	}
}

// TestFPRemoteSurfacesIntentIsolationAcrossInstances pins instance scoping:
// two servers with the same action and key keep separate intents and
// verbatim server IDs.
func TestFPRemoteSurfacesIntentIsolationAcrossInstances(t *testing.T) {
	db := surfacesDB(t)
	counter := 0
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			counter++
			return map[string]any{"batch_id": fmt.Sprintf("srv%d-batch", counter)}, nil
		},
	}
	stub := &remoteStubTransport{routes: routes}
	a := NewRemoteSurfaces("https://a.example", stub, nil, db, nil)
	b := NewRemoteSurfaces("https://b.example", stub, nil, db, nil)
	if _, err := a.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "shared"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "shared"}); err != nil {
		t.Fatal(err)
	}
	var rows []RemoteIntent
	if err := db.Find(&rows).Error; err != nil || len(rows) != 2 {
		t.Fatalf("instance isolation rows: %d err=%v", len(rows), err)
	}
	refs := []string{rows[0].RemoteRef, rows[1].RemoteRef}
	sort.Strings(refs)
	if refs[0] != "srv1-batch" || refs[1] != "srv2-batch" {
		t.Fatalf("server IDs must stay verbatim and distinct: %v", refs)
	}
	if rows[0].Instance == rows[1].Instance {
		t.Fatalf("instances collapsed: %+v", rows)
	}
}

// TestFPRemoteSurfacesCatalogMatchesFrozenContract pins the frozen action
// table: every catalog entry carries its route, write flag and ref kind, and
// the human-only management entries never entered the catalog.
func TestFPRemoteSurfacesCatalogMatchesFrozenContract(t *testing.T) {
	want := map[string]remoteSurfaceActionDef{
		"upload_batch_create":        {Method: "POST", Path: "/api/v1/upload-batches", Write: true, RefKind: "batch"},
		"upload_batch_add_items":     {Method: "POST", Path: "/api/v1/upload-batches/%s/items", PathArg: "batch_id", Write: true, RefKind: "batch"},
		"upload_batch_seal":          {Method: "POST", Path: "/api/v1/upload-batches/%s/seal", PathArg: "batch_id", Write: true, RefKind: "operation"},
		"upload_batch_get":           {Method: "GET", Path: "/api/v1/upload-batches/%s/items", PathArg: "batch_id"},
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
		"cloud_entries":              {Method: "GET", Path: "/api/v1/connections/%s/entries", PathArg: "connection_id"},
		"cloud_scan":                 {Method: "POST", Path: "/api/v1/connections/%s/scan", PathArg: "connection_id", Write: true, RefKind: "operation"},
		"changes_v2":                 {Method: "GET", Path: "/api/v1/changes"},
		"change_subscription_create": {Method: "POST", Path: "/api/v1/change-subscriptions", Write: true, RefKind: "subscription"},
		"change_subscription_ack":    {Method: "POST", Path: "/api/v1/change-subscriptions/%s/ack", PathArg: "subscription_id", Write: true},
		"automation_intent_get":      {Method: "GET", Path: "/api/v1/automation-intents/%s", PathArg: "intent_id"},
	}
	if len(remoteSurfaceCatalog) != len(want) {
		names := RemoteSurfaceActionNames()
		t.Fatalf("catalog size %d want %d: %v", len(remoteSurfaceCatalog), len(want), names)
	}
	for name, def := range want {
		got, ok := remoteSurfaceCatalog[name]
		if !ok {
			t.Fatalf("catalog missing %s", name)
		}
		if got.Method != def.Method || got.Path != def.Path || got.PathArg != def.PathArg || got.Write != def.Write || got.RefKind != def.RefKind {
			t.Fatalf("catalog %s = %+v want %+v", name, got, def)
		}
	}
	// Human-only management surfaces must never be agent-callable here.
	for _, banned := range []string{"connection_begin", "organize_confirm", "organization_policy_set", "automation_rules_set", "upload_part"} {
		if _, ok := remoteSurfaceCatalog[banned]; ok {
			t.Fatalf("human-only action %s must not be in the agent catalog", banned)
		}
	}
	if sort.SearchStrings(RemoteSurfaceActionNames(), "upload_batch_create") == len(RemoteSurfaceActionNames()) {
		t.Fatal("action names must be sorted for stable discovery reporting")
	}
	// Unknown actions report unsupported with the actual gap, never a guess.
	s, _ := surfacesFixture(t, nil, map[string]remoteStubHandler{})
	_, err := s.Run(context.Background(), "upload_batch_magic", nil)
	if err == nil || err.Error() != "unsupported" {
		t.Fatalf("unknown action: %v", err)
	}
	var detail *RemoteError
	if !errors.As(err, &detail) || len(detail.Missing) == 0 || !strings.Contains(strings.Join(detail.Missing, ","), "upload_batch_create") {
		t.Fatalf("unsupported must carry the actual gap: %+v", detail)
	}
	if detail.RecoveryHint() == "" || !strings.Contains(detail.RecoveryHint(), "capabilities") {
		t.Fatalf("unsupported hint must point at rediscovery: %q", detail.RecoveryHint())
	}
}

// surfacesDB opens the owner journal table on a scratch database.
func surfacesDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := openRemoteSurfacesTestDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// openRemoteSurfacesTestDB opens the additive journal table on a scratch
// state directory.
func openRemoteSurfacesTestDB(state string) (*gorm.DB, error) {
	if err := os.MkdirAll(state, 0700); err != nil {
		return nil, errors.New("state_unavailable")
	}
	return openRemoteSurfacesDB(filepath.Join(state, "remote-surfaces.sqlite"))
}

// TestFPRemoteSurfacesLegacyStateStaysReadable pins the additive migration:
// an owner state written before this package opens unchanged, legacy rows
// keep their IDs and URI shapes, and only the new journal table is added.
func TestFPRemoteSurfacesLegacyStateStaysReadable(t *testing.T) {
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	// Phase one: a pre-package state with one legacy space and file row.
	first, err := New(state, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.AddSpace(Space{ID: "legacy", Remote: "onedrive-main", DriveID: "d1", RootID: "r1", LocalRoot: t.TempDir(), MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	legacyFile := File{Ref: "drivebridge://file/legacy-ref", SpaceID: "legacy", DriveID: "d1", ItemID: "i1", Name: "old.txt", Size: 3}
	if err = first.DB.Create(&legacyFile).Error; err != nil {
		t.Fatal(err)
	}
	first.Close()
	// Phase two: the same state after the additive package lands.
	second, err := New(state, nil, false)
	if err != nil {
		t.Fatalf("legacy state no longer opens: %v", err)
	}
	defer second.Close()
	if !second.DB.Migrator().HasTable(&RemoteIntent{}) {
		t.Fatal("additive journal table missing after migration")
	}
	var probe File
	if err = second.DB.First(&probe, "ref = ?", "drivebridge://file/legacy-ref").Error; err != nil || probe.SpaceID != "legacy" || probe.Name != "old.txt" {
		t.Fatalf("legacy row degraded: %+v err=%v", probe, err)
	}
	if p, err := second.Space("legacy"); err != nil || p.Remote != "onedrive-main" {
		t.Fatalf("legacy space degraded: %+v err=%v", p, err)
	}
	// Rollback boundary: dropping the additive journal loses only adapter
	// transfer notes; every legacy table and row survives untouched.
	if err = second.DB.Migrator().DropTable(&RemoteIntent{}); err != nil {
		t.Fatal(err)
	}
	if err = second.DB.First(&probe, "ref = ?", "drivebridge://file/legacy-ref").Error; err != nil {
		t.Fatalf("legacy row lost after journal rollback: %v", err)
	}
	third, err := New(state, nil, false)
	if err != nil {
		t.Fatalf("re-open after rollback failed: %v", err)
	}
	defer third.Close()
	if !third.DB.Migrator().HasTable(&RemoteIntent{}) {
		t.Fatal("journal table must rebuild additively on next open")
	}
}

// TestFPRemoteSurfacesJournalIsRebuildableProjection pins that the journal
// never becomes a source of truth: server answers stay authoritative after
// the local projection is wiped, and a rebuilt journal accepts fresh intents
// under the same identity rules.
func TestFPRemoteSurfacesJournalIsRebuildableProjection(t *testing.T) {
	db := surfacesDB(t)
	calls := 0
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			calls++
			return map[string]any{"batch_id": "authoritative-batch"}, nil
		},
	}
	s, _ := surfacesFixture(t, db, routes)
	if _, err := s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "k"}); err != nil {
		t.Fatal(err)
	}
	// The projection is disposable: clearing it does not invent or destroy
	// server-side truth, and the same identity rules re-apply afterwards.
	if err := db.Where("1 = 1").Delete(&RemoteIntent{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "k2"}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("server stays the execution authority: %d calls", calls)
	}
	var rows []RemoteIntent
	if err := db.Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].RemoteRef != "authoritative-batch" {
		t.Fatalf("rebuilt journal rows: %+v err=%v", rows, err)
	}
}

// TestFPRemoteSurfacesNormalPathsFollowFrozenContract walks the design's
// normal scenarios through the shared application contract: discovery, the
// batch lifecycle, typed analytics queries, organization plans, snapshots
// and change feeds. Every request is asserted against the frozen route,
// method, paging window and verbatim server IDs.
func TestFPRemoteSurfacesNormalPathsFollowFrozenContract(t *testing.T) {
	db := surfacesDB(t)
	routes := map[string]remoteStubHandler{
		"GET /api/v1/capabilities": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"actions": []string{"upload_batch_create", "upload_batch_add_items", "upload_batch_seal", "upload_batch_get", "analytics_query", "analytics_snapshot", "analytics_snapshot_query", "organize_plan", "organize_plan_get", "organize_apply", "changes_v2"}}, nil
		},
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "SRV-BATCH-1", "batch_revision": 1, "phase": "enumerating"}, nil
		},
		"POST /api/v1/upload-batches/SRV-BATCH-1/items": func(call remoteStubCall) (map[string]any, error) {
			items, _ := call.Body["items"].([]any)
			return map[string]any{"accepted": len(items), "next_cursor": ""}, nil
		},
		"POST /api/v1/upload-batches/SRV-BATCH-1/seal": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"operation_id": "SRV-OP-9", "part_size": 8388608}, nil
		},
		"GET /api/v1/upload-batches/SRV-BATCH-1/items": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "SRV-BATCH-1", "phase": "sealed", "items": []any{map[string]any{"item_key": "a", "operation_id": "SRV-OP-ITEM-1", "result_certainty": "confirmed"}}}, nil
		},
		"POST /api/v1/analytics/queries": func(call remoteStubCall) (map[string]any, error) {
			if call.Body["dataset"] != "file_current" {
				return nil, errors.New("invalid_request")
			}
			return map[string]any{"query_id": "SRV-Q-1", "rows": []any{map[string]any{"mime_type": "image/png", "files": 4}}, "next_cursor": ""}, nil
		},
		"POST /api/v1/analytics/snapshots": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"snapshot_id": "SRV-SNAP-1", "generation": "gen-7"}, nil
		},
		"POST /api/v1/analytics/snapshots/SRV-SNAP-1/queries": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"rows": []any{map[string]any{"bytes": 2048}}, "generation": "gen-7"}, nil
		},
		"POST /api/v1/organization-plans": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"plan_id": "SRV-PLAN-1", "revision": 3, "items_total": 2}, nil
		},
		"GET /api/v1/organization-plans/SRV-PLAN-1": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"plan_id": "SRV-PLAN-1", "revision": 3, "phase": "confirmed"}, nil
		},
		"POST /api/v1/organization-plans/SRV-PLAN-1/execute": func(call remoteStubCall) (map[string]any, error) {
			if call.Body["expected_revision"] != float64(3) {
				return nil, errors.New("version_conflict")
			}
			return map[string]any{"operation_id": "SRV-OP-ORG-1", "items_applied": 2}, nil
		},
		"GET /api/v1/changes": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"events": []any{}, "cursor": "opaque-1"}, nil
		},
	}
	s, stub := surfacesFixture(t, db, routes)
	ctx := context.Background()
	// Discovery reads the live capability document.
	actions, err := s.Actions(ctx)
	found := false
	for _, a := range actions {
		if a == "analytics_query" {
			found = true
		}
	}
	if err != nil || len(actions) == 0 || !found {
		t.Fatalf("discovery: %v err=%v", actions, err)
	}
	// Batch lifecycle: create → add → seal → get, server IDs verbatim.
	create, err := s.Run(ctx, "upload_batch_create", map[string]any{"space_id": "sp", "conflict_policy": "fail", "idempotency_key": "k-create"})
	if err != nil {
		t.Fatal(err)
	}
	if create.(map[string]any)["batch_id"] != "SRV-BATCH-1" {
		t.Fatalf("create receipt: %v", create)
	}
	add, err := s.Run(ctx, "upload_batch_add_items", map[string]any{"batch_id": "SRV-BATCH-1", "idempotency_key": "k-items", "items": []map[string]any{{"relative_path": "a/b.txt", "size_bytes": 3, "sha256": "x", "item_key": "a"}}})
	if err != nil || add.(map[string]any)["accepted"] != float64(1) {
		t.Fatalf("add items: %v err=%v", add, err)
	}
	seal, err := s.Run(ctx, "upload_batch_seal", map[string]any{"batch_id": "SRV-BATCH-1", "idempotency_key": "k-seal"})
	if err != nil || seal.(map[string]any)["operation_id"] != "SRV-OP-9" {
		t.Fatalf("seal: %v err=%v", seal, err)
	}
	status, err := s.Run(ctx, "upload_batch_get", map[string]any{"batch_id": "SRV-BATCH-1", "limit": 500})
	if err != nil || status.(map[string]any)["phase"] != "sealed" {
		t.Fatalf("batch get: %v err=%v", status, err)
	}
	for _, call := range stub.recorded() {
		if call.Path == "/api/v1/upload-batches/SRV-BATCH-1/items?limit=200" {
			goto pagingOK
		}
	}
	t.Fatal("page limit 500 must clamp to the frozen maximum 200")
pagingOK:
	// Typed analytics query with the fixed dataset enum.
	query, err := s.Run(ctx, "analytics_query", map[string]any{"dataset": "file_current", "idempotency_key": "k-q", "dimensions": []string{"mime_type"}, "limit": 500})
	if err != nil || query.(map[string]any)["query_id"] != "SRV-Q-1" {
		t.Fatalf("analytics query: %v err=%v", query, err)
	}
	if _, err = s.Run(ctx, "analytics_query", map[string]any{"dataset": "free_form_sql", "idempotency_key": "k-q2"}); err == nil || err.Error() != "invalid_request" {
		t.Fatalf("free-form dataset must be rejected locally: %v", err)
	}
	// Snapshot lifecycle: create then server-side typed query.
	snap, err := s.Run(ctx, "analytics_snapshot", map[string]any{"space_id": "sp", "idempotency_key": "k-snap"})
	if err != nil || snap.(map[string]any)["snapshot_id"] != "SRV-SNAP-1" {
		t.Fatalf("snapshot: %v err=%v", snap, err)
	}
	snapQuery, err := s.Run(ctx, "analytics_snapshot_query", map[string]any{"snapshot_id": "SRV-SNAP-1", "dataset": "file_current"})
	if err != nil || snapQuery.(map[string]any)["generation"] != "gen-7" {
		t.Fatalf("snapshot query: %v err=%v", snapQuery, err)
	}
	// Organization plan lifecycle with the expected-revision guard.
	plan, err := s.Run(ctx, "organize_plan", map[string]any{"refs": []string{"drivebridge://file/x"}, "actions": []string{"move"}, "policy_id": "pol", "idempotency_key": "k-plan"})
	if err != nil || plan.(map[string]any)["plan_id"] != "SRV-PLAN-1" {
		t.Fatalf("organize plan: %v err=%v", plan, err)
	}
	if _, err = s.Run(ctx, "organize_apply", map[string]any{"plan_id": "SRV-PLAN-1", "expected_revision": 2, "idempotency_key": "k-apply"}); err == nil || err.Error() != "version_conflict" {
		t.Fatalf("stale revision must be refused by the server contract: %v", err)
	}
	applied, err := s.Run(ctx, "organize_apply", map[string]any{"plan_id": "SRV-PLAN-1", "expected_revision": 3, "idempotency_key": "k-apply2"})
	if err != nil || applied.(map[string]any)["operation_id"] != "SRV-OP-ORG-1" {
		t.Fatalf("organize apply: %v err=%v", applied, err)
	}
	// Change feed with the frozen paging default.
	feed, err := s.Run(ctx, "changes_v2", map[string]any{"space_id": "sp"})
	if err != nil || feed.(map[string]any)["cursor"] != "opaque-1" {
		t.Fatalf("changes feed: %v err=%v", feed, err)
	}
	for _, call := range stub.recorded() {
		if call.Path == "/api/v1/changes?limit=50&space_id=sp" {
			return
		}
	}
	t.Fatal("changes must apply the default page size 50")
}

// partRecorder is the byte-path fixture: it counts parts and bytes without
// keeping any payload, mirroring the bounded server side.
type partRecorder struct {
	parts map[string]int64
	bytes int64
	fail  error
}

func (p *partRecorder) PutPart(ctx context.Context, operationID string, index, size int64, body io.Reader) error {
	if p.fail != nil {
		return p.fail
	}
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	if p.parts == nil {
		p.parts = map[string]int64{}
	}
	p.parts[operationID+"/"+strconv.FormatInt(index, 10)] = size
	p.bytes += size
	return nil
}

// TestFPRemoteSurfacesResponseLossRecoversOriginalID pins spec scenario 1:
// a lost upload response recovers the original server operation through the
// same key plus the same original request; no re-keying, no replacement.
func TestFPRemoteSurfacesResponseLossRecoversOriginalID(t *testing.T) {
	db := surfacesDB(t)
	dropped := false
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			_ = dropped
			// The request is accepted; only the response is lost, every time.
			return nil, errors.New("service_unavailable")
		},
		"POST /api/v1/upload-batches/SRV-BATCH-recover-key-2/items": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"accepted": 1}, nil
		},
		"POST /api/v1/upload-batches/SRV-BATCH-recover-key-2/seal": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"operation_id": "SRV-OP-recover-key-2", "part_size": 4}, nil
		},
		"GET /api/v1/upload-batches/SRV-BATCH-recover-key-2/items": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "SRV-BATCH-recover-key-2", "phase": "sealed"}, nil
		},
		"POST /api/v1/operations/lookup": func(call remoteStubCall) (map[string]any, error) {
			if call.Body["action"] != "upload_batch_create" {
				return nil, errors.New("invalid_request")
			}
			original, _ := call.Body["original_request"].(map[string]any)
			if original["space_id"] != "sp" || original["conflict_policy"] != "fail" {
				return nil, errors.New("invalid_request")
			}
			key, _ := call.Body["idempotency_key"].(string)
			return map[string]any{"found": true, "batch_id": "SRV-BATCH-" + key, "operation_id": "SRV-OP-" + key, "result_certainty": "confirmed", "recovery_action": "read_original_operation"}, nil
		},
	}
	s, _ := surfacesFixture(t, db, routes)
	ctx := context.Background()
	// Manual recovery through the public lookup action keeps the original
	// server identity visible to the caller.
	recovered, err := s.Run(ctx, "operation_lookup", map[string]any{
		"action": "upload_batch_create", "idempotency_key": "recover-key",
		"original_request": map[string]any{"space_id": "sp", "conflict_policy": "fail"},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := recovered.(map[string]any)
	if doc["batch_id"] != "SRV-BATCH-recover-key" || doc["found"] != true {
		t.Fatalf("lookup receipt: %v", doc)
	}
	// The composite flow recovers by itself: the create transport drops,
	// lookup restores the original batch, and the upload continues under
	// the server's IDs.
	parts := &partRecorder{}
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "payload.bin"), []byte("recoverable-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	full := NewRemoteSurfaces(s.Instance, s.Call, parts, db, root)
	receipt, err := full.UploadLocalFile(ctx, "sp", "payload.bin", "fail", "recover-key-2")
	if err != nil {
		t.Fatalf("upload recovery: %v", err)
	}
	if receipt["batch_id"] != "SRV-BATCH-recover-key-2" || receipt["operation_id"] != "SRV-OP-recover-key-2" {
		t.Fatalf("recovered upload must continue under the original server IDs: %v", receipt)
	}
	if parts.bytes != 17 || len(parts.parts) != 5 {
		t.Fatalf("bounded parts after recovery: bytes=%d parts=%v", parts.bytes, parts.parts)
	}
	// The journal keeps the unknown verdict until lookup resolves it.
	var rows []RemoteIntent
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Key == "recover-key-2" && row.Action == "upload_batch_create" && row.Phase == remoteIntentConfirmed && row.RemoteRef == "SRV-BATCH-recover-key-2" {
			return
		}
	}
	t.Fatalf("recovered intent must settle under the original server ID: %+v", rows)
}

// TestFPRemoteSurfacesPermissionAndSnapshotBoundaries pins spec scenarios
// 2 and 3: the pure-MCP answer for local paths, the root guard, revoked
// permissions without escalation, and the optional snapshot analysis path.
func TestFPRemoteSurfacesPermissionAndSnapshotBoundaries(t *testing.T) {
	db := surfacesDB(t)
	calls := map[string]int{}
	count := func(name string) int { return calls[name] }
	routes := map[string]remoteStubHandler{
		"POST /api/v1/analytics/snapshots": func(call remoteStubCall) (map[string]any, error) {
			calls["snapshot"]++
			return nil, errors.New("forbidden")
		},
		"POST /api/v1/analytics/snapshots/SRV-SNAP-1/queries": func(call remoteStubCall) (map[string]any, error) {
			calls["snapshot_query"]++
			return nil, errors.New("forbidden")
		},
	}
	record := func(name string, h remoteStubHandler) remoteStubHandler {
		return func(call remoteStubCall) (map[string]any, error) {
			calls[name]++
			return h(call)
		}
	}
	routes["POST /api/v1/upload-batches"] = record("create", func(call remoteStubCall) (map[string]any, error) {
		return map[string]any{"batch_id": "B"}, nil
	})
	ctx := context.Background()
	pure, _ := surfacesFixture(t, db, routes)
	// Scenario 2a: a remote agent without a local adapter asks to upload a
	// local path. The answer names the real byte entries and refuses to
	// pretend the remote control plane can read disks.
	_, err := pure.Run(ctx, "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "k", "local_path": "/home/user/secret.txt"})
	if err == nil || err.Error() != "unsupported" {
		t.Fatalf("pure MCP local path: %v", err)
	}
	var detail *RemoteError
	if errors.As(err, &detail) && detail.RecoveryHint() != "" {
		if !strings.Contains(detail.RecoveryHint(), "web/HTTP") {
			t.Fatalf("hint must name the web/HTTP entry: %q", detail.RecoveryHint())
		}
	}
	if count("create") != 0 {
		t.Fatal("no upload request may leave before the source question is settled")
	}
	// Scenario 3b: revoked export authorization. The refusal passes through
	// and the adapter never degrades to any wider analytics call.
	_, err = pure.Run(ctx, "analytics_snapshot", map[string]any{"space_id": "sp", "idempotency_key": "k-snap"})
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("revoked permission must surface forbidden: %v", err)
	}
	_, err = pure.Run(ctx, "analytics_snapshot_query", map[string]any{"snapshot_id": "SRV-SNAP-1", "dataset": "file_current"})
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("unauthorized snapshot query must surface forbidden: %v", err)
	}
	if count("snapshot") != 1 || count("snapshot_query") != 1 {
		t.Fatalf("no escalation retries allowed: %v", calls)
	}
	// Optional local snapshot execution without a runner: the optional path
	// reports its own availability; the code stays out of the shared set.
	_, err = pure.Run(ctx, "analytics_snapshot_query", map[string]any{"snapshot_id": "SRV-SNAP-1", "execution": "local"})
	if err == nil || err.Error() != "analytics_backend_unavailable" {
		t.Fatalf("missing optional backend: %v", err)
	}
	if count("snapshot_query") != 1 {
		t.Fatal("local execution must not hit the server route")
	}
	// Scenario 3a: with the local runner configured, the typed query runs
	// offline and declares the snapshot generation and offline copy.
	offline := &offlineAnalyticsRunner{}
	pure.Analytics = offline
	out, err := pure.Run(ctx, "analytics_snapshot_query", map[string]any{"snapshot_id": "SRV-SNAP-1", "execution": "local", "dataset": "file_current"})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := out.(map[string]any)
	if doc["generation"] != "gen-7" || doc["copy"] != "offline" {
		t.Fatalf("offline snapshot result: %v", doc)
	}
	if offline.lastSnapshot != "SRV-SNAP-1" || count("snapshot_query") != 1 {
		t.Fatalf("runner binding: snapshot=%q calls=%v", offline.lastSnapshot, calls)
	}
}

type offlineAnalyticsRunner struct{ lastSnapshot string }

func (o *offlineAnalyticsRunner) QuerySnapshot(ctx context.Context, snapshotID string, query map[string]any) (json.RawMessage, error) {
	o.lastSnapshot = snapshotID
	return json.RawMessage(`{"generation":"gen-7","copy":"offline","rows":[]}`), nil
}

// TestFPRemoteSurfacesRootGuardAndPartialResults pins the local root guard
// (scenario 2b) and partial batch receipts passing through with per-item
// certainty, plus cancel keeping the original identity.
func TestFPRemoteSurfacesRootGuardAndPartialResults(t *testing.T) {
	db := surfacesDB(t)
	routes := map[string]remoteStubHandler{
		"GET /api/v1/upload-batches/B/items": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "B", "phase": "sealed", "items": []any{
				map[string]any{"item_key": "done", "operation_id": "OP-1", "result_certainty": "confirmed"},
				map[string]any{"item_key": "pending", "result_certainty": "unknown"},
				map[string]any{"item_key": "failed", "error": "content_mismatch"},
			}}, nil
		},
		"POST /api/v1/upload-batches/B/cancel": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "B", "phase": "cancelled", "operation_id": "OP-9"}, nil
		},
		"GET /api/v1/changes": func(call remoteStubCall) (map[string]any, error) {
			return nil, errors.New("cursor_expired")
		},
	}
	s, stub := surfacesFixture(t, db, routes)
	ctx := context.Background()
	// Scenario 2b: a local source outside the configured root is refused
	// before any byte moves.
	rootDir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "escape.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	guarded := NewRemoteSurfaces(s.Instance, s.Call, &partRecorder{}, db, root)
	_, err = guarded.UploadLocalFile(ctx, "sp", "../"+filepath.Base(outside)+"/escape.txt", "fail", "k-escape")
	if err == nil || err.Error() != "path_not_allowed" {
		t.Fatalf("outside-root source: %v", err)
	}
	if abs, err := filepath.Abs(filepath.Join(outside, "escape.txt")); err == nil {
		if _, err = guarded.UploadLocalFile(ctx, "sp", abs, "fail", "k-escape-abs"); err == nil || err.Error() != "path_not_allowed" {
			t.Fatalf("absolute outside-root source: %v", err)
		}
	}
	if len(stub.recorded()) != 0 {
		t.Fatalf("guard must precede every request: %v", stub.recorded())
	}
	// Partial receipts pass through per-item certainty verbatim.
	partial, err := s.Run(ctx, "upload_batch_get", map[string]any{"batch_id": "B"})
	if err != nil {
		t.Fatal(err)
	}
	items := partial.(map[string]any)["items"].([]any)
	if len(items) != 3 || items[1].(map[string]any)["result_certainty"] != "unknown" {
		t.Fatalf("partial receipt: %v", partial)
	}
	// Cancel keeps the original server identity.
	cancelled, err := s.Run(ctx, "upload_batch_cancel", map[string]any{"batch_id": "B", "idempotency_key": "k-cancel"})
	if err != nil || cancelled.(map[string]any)["batch_id"] != "B" || cancelled.(map[string]any)["operation_id"] != "OP-9" {
		t.Fatalf("cancel receipt: %v err=%v", cancelled, err)
	}
	var row RemoteIntent
	if err := db.First(&row, "key = ?", "k-cancel").Error; err != nil || row.RemoteRef != "B" || row.Phase != remoteIntentConfirmed {
		t.Fatalf("cancel intent: %+v err=%v", row, err)
	}
	// Cross-scope cursors surface the server refusal verbatim.
	if _, err = s.Run(ctx, "changes_v2", map[string]any{"cursor": "other-tenant"}); err == nil || err.Error() != "cursor_expired" {
		t.Fatalf("cross-scope cursor: %v", err)
	}
}

// TestFPRemoteSurfacesConcurrentSameKeySingleRow pins the journal under
// concurrent identical intents: the unique scope index keeps one row and
// both callers see the same verdict. (The race detector itself is not run
// per repository policy; the invariant is asserted functionally.)
func TestFPRemoteSurfacesConcurrentSameKeySingleRow(t *testing.T) {
	db := surfacesDB(t)
	served := 0
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			served++
			return map[string]any{"batch_id": "SRV-CONCURRENT"}, nil
		},
	}
	s, _ := surfacesFixture(t, db, routes)
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.Run(context.Background(), "upload_batch_create", map[string]any{"space_id": "sp", "idempotency_key": "same-key"})
		}(i)
	}
	wg.Wait()
	var rows []RemoteIntent
	if err := db.Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("concurrent same-key intents must collapse to one row: %d err=%v", len(rows), err)
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	if rows[0].RemoteRef != "SRV-CONCURRENT" {
		t.Fatalf("concurrent verdict: %+v", rows[0])
	}
}

// loopReader endlessly re-yields one pattern block: the synthetic scale
// source produces real bytes on the fly without a 100 GiB file on disk.
type loopReader struct{ block []byte }

func (l *loopReader) Read(p []byte) (int, error) {
	n := copy(p, l.block[:min(len(l.block), len(p))])
	if n == 0 && len(p) > 0 {
		n = copy(p, l.block[:1])
	}
	return n, nil
}

func (l *loopReader) openN(total int64) io.ReadCloser {
	return io.NopCloser(io.LimitReader(l, total))
}

// scaleFixtureServer serves the public contract for the scale entries with
// real HTTP: keyset batch paging and the part byte path.
func scaleFixtureServer(t *testing.T) (*httptest.Server, *scaleState) {
	t.Helper()
	state := &scaleState{partSizes: map[int64]int64{}}
	mux := http.NewServeMux()
	encode := func(w http.ResponseWriter, doc map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": doc})
	}
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"actions": RemoteSurfaceActionNames()})
	})
	mux.HandleFunc("POST /api/v1/upload-batches", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"batch_id": "SCALE-BATCH"})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		encode(w, map[string]any{"accepted": 1})
	})
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/seal", func(w http.ResponseWriter, r *http.Request) {
		// The server-declared part size governs the client buffer; 256 MiB
		// mirrors the single-transfer buffer ceiling of the platform gate.
		encode(w, map[string]any{"operation_id": "SCALE-OP", "part_size": 256 << 20})
	})
	mux.HandleFunc("PUT /api/v1/operations/{id}/parts/{index}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		state.mu.Lock()
		index, _ := strconv.ParseInt(r.PathValue("index"), 10, 64)
		state.partSizes[index] = n
		state.partBytes += n
		state.maxPart = max(state.maxPart, n)
		state.mu.Unlock()
		encode(w, map[string]any{"received_bytes": n})
	})
	mux.HandleFunc("GET /api/v1/upload-batches/SCALE-BATCH/items", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		cursor, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
		state.mu.Lock()
		state.requestedLimits = append(state.requestedLimits, limit)
		state.mu.Unlock()
		if limit > remotePageMax {
			encode(w, map[string]any{"error": "invalid_request"})
			return
		}
		items := []map[string]any{}
		next := ""
		end := min(cursor+limit, 10000)
		for i := cursor; i < end; i++ {
			items = append(items, map[string]any{"item_key": fmt.Sprintf("scale-item-%06d", i), "operation_id": fmt.Sprintf("SCALE-ITEM-OP-%06d", i), "result_certainty": "confirmed"})
		}
		if end < 10000 {
			next = strconv.Itoa(end)
		}
		encode(w, map[string]any{"batch_id": "SCALE-BATCH", "phase": "sealed", "items": items, "next_cursor": next})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, state
}

type scaleState struct {
	mu              sync.Mutex
	partSizes       map[int64]int64
	partBytes       int64
	maxPart         int64
	requestedLimits []int
}

// TestFPRemoteSurfacesScaleTenThousandItemsPaging consumes the server's
// ten-thousand-item batch through the public paging contract: every page
// stays inside the frozen window, all items arrive with verbatim server
// IDs, and no page ever materializes the whole batch.
func TestFPRemoteSurfacesScaleTenThousandItemsPaging(t *testing.T) {
	server, state := scaleFixtureServer(t)
	client, err := sdk.New(server.URL, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	s := NewRemoteSurfaces(server.URL, client, nil, nil, nil)
	total, pages := 0, 0
	cursor := ""
	for {
		args := map[string]any{"batch_id": "SCALE-BATCH", "limit": 200}
		if cursor != "" {
			args["cursor"] = cursor
		}
		page, err := s.Run(context.Background(), "upload_batch_get", args)
		if err != nil {
			t.Fatal(err)
		}
		doc := page.(map[string]any)
		items := doc["items"].([]any)
		if len(items) > remotePageMax {
			t.Fatalf("page exceeded the frozen window: %d", len(items))
		}
		total += len(items)
		pages++
		next, _ := doc["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if total != 10000 || pages != 50 {
		t.Fatalf("ten-thousand-item batch: items=%d pages=%d", total, pages)
	}
	for _, limit := range state.requestedLimits {
		if limit < 1 || limit > remotePageMax {
			t.Fatalf("requested limit outside the frozen window: %d", limit)
		}
	}
}

// TestFPRemoteSurfacesScaleHundredGiBStreamingEntry is the platform's
// 100 GiB streaming test entry for this adapter: one logical source of
// 100 GiB streams through the real digest pass and the real HTTP part
// path with a single reused part buffer. Memory stays bounded regardless
// of size; the per-part ceiling and total byte count are asserted from
// the server side. (The backend owner's real-100 GiB-disk gate and the
// real-cloud run remain separately reported integration facts.)
func TestFPRemoteSurfacesScaleHundredGiBStreamingEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("100 GiB streaming entry moves the full logical volume through digest and HTTP")
	}
	server, state := scaleFixtureServer(t)
	client, err := sdk.New(server.URL, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	s := NewRemoteSurfaces(server.URL, client, &PartUploader{Base: server.URL, Token: "fixture-token"}, nil, root)
	block := make([]byte, 1<<20)
	for i := range block {
		block[i] = byte(i * 31)
	}
	source := &loopReader{block: block}
	const total = int64(100) << 30
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	openCount := 0
	receipt, err := s.uploadBounded(context.Background(), "sp", "scale/hundred-gib.bin", total, func() (io.ReadCloser, error) {
		openCount++
		return source.openN(total), nil
	}, "fail", "scale-hundred-gib")
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if receipt["bytes_streamed"] != total || receipt["size_bytes"] != total {
		t.Fatalf("streamed volume: %v", receipt)
	}
	state.mu.Lock()
	parts, bytes, maxPart := int64(len(state.partSizes)), state.partBytes, state.maxPart
	state.mu.Unlock()
	const partSize = int64(256 << 20)
	if parts != (total+partSize-1)/partSize || bytes != total {
		t.Fatalf("server observed parts=%d bytes=%d", parts, bytes)
	}
	if maxPart > partSize {
		t.Fatalf("part ceiling exceeded: %d > %d", maxPart, partSize)
	}
	if openCount != 2 {
		t.Fatalf("source must be read exactly twice (digest, stream): %d opens", openCount)
	}
	// Bounded memory: cumulative allocations stay far below the streamed
	// volume because the part buffer is reused, never grown.
	if delta := after.TotalAlloc - before.TotalAlloc; delta > 1<<30 {
		t.Fatalf("cumulative allocations %d MiB exceeded the bounded ceiling", delta>>20)
	}
}

// TestFPRemoteSurfacesFreshUploadWithJournalNeverSelfConflicts is the red
// regression for the fingerprint split: the pre-request journal and the Run
// dispatch must agree on the original-request shape (business inputs only,
// the idempotency key scopes the request), so a fresh journaled upload
// succeeds without ever consulting lookup.
func TestFPRemoteSurfacesFreshUploadWithJournalNeverSelfConflicts(t *testing.T) {
	db := surfacesDB(t)
	lookups := 0
	routes := map[string]remoteStubHandler{
		"POST /api/v1/upload-batches": func(call remoteStubCall) (map[string]any, error) {
			if _, ok := call.Body["idempotency_key"]; ok {
				return nil, errors.New("invalid_request")
			}
			return map[string]any{"batch_id": "FRESH-BATCH"}, nil
		},
		"POST /api/v1/upload-batches/FRESH-BATCH/items": func(call remoteStubCall) (map[string]any, error) {
			if _, ok := call.Body["idempotency_key"]; ok {
				return nil, errors.New("invalid_request")
			}
			return map[string]any{"accepted": 1}, nil
		},
		"POST /api/v1/upload-batches/FRESH-BATCH/seal": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"operation_id": "FRESH-OP", "part_size": 4}, nil
		},
		"GET /api/v1/upload-batches/FRESH-BATCH/items": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"batch_id": "FRESH-BATCH", "phase": "sealed"}, nil
		},
		"POST /api/v1/operations/lookup": func(call remoteStubCall) (map[string]any, error) {
			lookups++
			return map[string]any{"found": false}, nil
		},
	}
	s, _ := surfacesFixture(t, db, routes)
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "fresh.bin"), []byte("fresh-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	parts := &partRecorder{}
	full := NewRemoteSurfaces(s.Instance, s.Call, parts, db, root)
	receipt, err := full.UploadLocalFile(context.Background(), "sp", "fresh.bin", "fail", "fresh-key")
	if err != nil {
		t.Fatalf("fresh journaled upload must succeed: %v", err)
	}
	if receipt["batch_id"] != "FRESH-BATCH" {
		t.Fatalf("fresh receipt: %v", receipt)
	}
	if lookups != 0 {
		t.Fatalf("a successful fresh upload must never consult lookup: %d calls", lookups)
	}
	var rows []RemoteIntent
	if err := db.Find(&rows).Error; err != nil || len(rows) == 0 {
		t.Fatalf("journal rows: %d err=%v", len(rows), err)
	}
	for _, row := range rows {
		if row.Phase == remoteIntentUnknown || row.Certainty == "unknown" {
			t.Fatalf("journaled write must settle: %+v", row)
		}
	}
}

// TestFPRemoteSurfacesCompositeToolNeedsFullBatchFamily is the red
// regression for capability gating: the adapter-local upload_batch_file
// tool registers only when the server advertises the whole batch family it
// orchestrates; any missing member keeps it off tools/list and in the
// status gap.
func TestFPRemoteSurfacesCompositeToolNeedsFullBatchFamily(t *testing.T) {
	s, _ := surfacesFixture(t, nil, map[string]remoteStubHandler{
		"GET /api/v1/capabilities": func(call remoteStubCall) (map[string]any, error) {
			// seal is missing: the family is incomplete.
			return map[string]any{"actions": []string{"upload_batch_create", "upload_batch_add_items", "upload_batch_get", "organize_plan_get"}}, nil
		},
	})
	ctx := context.Background()
	tools, missing, err := s.DiscoverRemoteSurfaceTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Action == "upload_batch_file" {
			t.Fatal("composite upload tool registered with an incomplete batch family")
		}
	}
	joined := strings.Join(missing, ",")
	if !strings.Contains(joined, "upload_batch_file") || !strings.Contains(joined, "upload_batch_seal") {
		t.Fatalf("missing report must carry the composite and the absent member: %v", missing)
	}
	// With the complete family the composite registers.
	s2, _ := surfacesFixture(t, nil, map[string]remoteStubHandler{
		"GET /api/v1/capabilities": func(call remoteStubCall) (map[string]any, error) {
			return map[string]any{"actions": []string{"upload_batch_create", "upload_batch_add_items", "upload_batch_seal", "upload_batch_get"}}, nil
		},
	})
	tools2, _, err := s2.DiscoverRemoteSurfaceTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools2 {
		if tool.Action == "upload_batch_file" {
			found = true
		}
	}
	if !found {
		t.Fatal("composite upload tool missing with the complete batch family")
	}
}

// TestFPRemoteSurfacesVFSCacheSiblingRemotesStayIsolated is the red
// regression for cache attribution: rclone nests each remote's cache under
// its exact directory name, so nas and nas-backup never cross-account.
func TestFPRemoteSurfacesVFSCacheSiblingRemotesStayIsolated(t *testing.T) {
	root := t.TempDir()
	for remote, uploads := range map[string]int{"nas": 1, "nas-backup": 2} {
		dir := filepath.Join(root, remote, "upload")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for i := range uploads {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("pending-%d", i)), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	restore := vfsCacheRootForTest(t, root)
	defer restore()
	nas := vfsClasses("nas")
	if len(nas) != 1 || nas[0].Class != "vfs_pending_upload" || nas[0].Files != 1 {
		t.Fatalf("nas attribution: %+v", nas)
	}
	backup := vfsClasses("nas-backup")
	if len(backup) != 1 || backup[0].Files != 2 {
		t.Fatalf("nas-backup attribution: %+v", backup)
	}
	if other := vfsClasses("nas2"); len(other) != 0 {
		t.Fatalf("unrelated remote must not match: %+v", other)
	}
}

// vfsCacheRootForTest pins the VFS cache root for one test and returns the
// restore func.
func vfsCacheRootForTest(t *testing.T, root string) func() {
	t.Helper()
	previous := vfsCacheRootOverride
	vfsCacheRootOverride = root
	return func() { vfsCacheRootOverride = previous }
}
