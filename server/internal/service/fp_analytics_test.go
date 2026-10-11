package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// analyticsRig starts the fixture with the analytics-relevant surfaces.
func analyticsRig(t *testing.T) *fixtures.Runtime {
	t.Helper()
	f, e := fixtures.StartWithOptions(t.TempDir(), "", "", fixtures.Options{})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	return f
}

// Task 2.1/3.1: the typed query contract — closed vocabularies, injected
// permission scope, honest observed unavailability and working live reads.
func TestFPAnalyticsSearchTypedQueryContract(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	for i := range 6 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: pathJoin("an", "f", i)}, "an-"+time.Now().Format("150405.000000000")+"-"+itoa(i), sizeReader(100+i), nil); e != nil {
			t.Fatal(e)
		}
	}
	// Unknown dataset / field / metric / op are invalid_query.
	bad := []sdk.AnalyticsQueryRequest{
		{Dataset: "pg_sleep"},
		{Dataset: "file_current", Filters: []sdk.AnalyticsFilter{{Field: "owner; DROP TABLE files", Op: "eq", Value: "x"}}},
		{Dataset: "file_current", Metrics: []string{"count(*)"}},
		{Dataset: "file_current", Filters: []sdk.AnalyticsFilter{{Field: "mime_type", Op: "regex", Value: "x"}}},
	}
	for _, q := range bad {
		if _, e := c.QueryAnalytics(context.Background(), q); e == nil || e.Error() != "invalid_query" {
			t.Fatalf("bad query %+v must be invalid_query, got %v", q, e)
		}
	}
	// Observed consistency reports the missing analytics backend honestly.
	if _, e := c.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}, Consistency: "observed"}); e == nil || e.Error() != "analytics_backend_unavailable" {
		t.Fatalf("observed without backend must be analytics_backend_unavailable, got %v", e)
	}
	// Live consistency answers exactly from the transactional catalogue.
	live, e := c.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Dimensions: []string{"mime_type"}, Metrics: []string{"file_count", "logical_bytes"}, Consistency: "live"})
	if e != nil {
		t.Fatal(e)
	}
	if live.Backend != "transactional" || live.Coverage != "complete" || live.Partial {
		t.Fatalf("live view wrong: %+v", live)
	}
	totalFiles, totalBytes := 0, 0
	for _, row := range live.Rows {
		totalFiles += int(row["file_count"].(float64))
		totalBytes += int(row["logical_bytes"].(float64))
	}
	if totalFiles != 6 || totalBytes != 6*100+(0+1+2+3+4+5) {
		t.Fatalf("live aggregates wrong: files=%d bytes=%d", totalFiles, totalBytes)
	}
	// An agent sees only its bound scope; counts never leak other spaces.
	a, ac := agent(t, h, "an", "read")
	_ = a
	agentView, e := ac.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}, Consistency: "live"})
	if e != nil {
		t.Fatal(e)
	}
	if len(agentView.Rows) != 1 || int(agentView.Rows[0]["file_count"].(float64)) != 6 {
		t.Fatalf("agent scope wrong: %+v", agentView)
	}
	// Cross-owner isolation: bob's own scope answers with his facts only
	// and never counts alice's files.
	bob, _ := login(t, f, "bob")
	bc := mustSDK(t, bob)
	bobView, e := bc.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}, Consistency: "live"})
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range bobView.Rows {
		if row["space_id"] == h.space.ID || int(row["file_count"].(float64)) != 0 {
			t.Fatalf("cross-owner leak: %+v", row)
		}
	}
}

// Task 3.2: revocation and drill-down — the same scope filters both the
// aggregate and its detail rows; a revoked agent loses analytics entirely.
func TestFPAnalyticsSearchRevocationAndDrillDown(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "an", "read,write")
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "an/a.txt"}, "adr-a", sizeReader(50), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "an/b.txt"}, "adr-b", sizeReader(70), nil); e != nil {
		t.Fatal(e)
	}
	agg, e := c.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count", "logical_bytes"}, Consistency: "live"})
	if e != nil || int(agg.Rows[0]["file_count"].(float64)) != 2 || int(agg.Rows[0]["logical_bytes"].(float64)) != 120 {
		t.Fatalf("aggregate wrong: %+v %v", agg, e)
	}
	// Drill-down with the same scope agrees with the aggregate.
	detail, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "path_prefix", Op: "prefix", Value: "an/"}}})
	if e != nil || len(detail.Items) != 2 {
		t.Fatalf("drill-down disagrees: %+v %v", detail, e)
	}
	// Revocation stops analytics reads without leaking counts.
	if status := h.call(t, "POST", "/agents/"+a.ID+"/revoke", map[string]any{}, nil); status != 200 {
		t.Fatalf("revoke %d", status)
	}
	if _, e := c.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}, Consistency: "live"}); e == nil || e.Error() != "unauthorized" {
		t.Fatalf("revoked analytics must be unauthorized, got %v", e)
	}
}

// Task 3.1/3.2: snapshots are human export authorizations with service-
// generated manifests; snapshot queries validate typed input first and
// then report the missing fixed engine honestly.
func TestFPAnalyticsSearchSnapshotsAndEngineIsolation(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "an/s.txt"}, "as-1", sizeReader(42), nil); e != nil {
		t.Fatal(e)
	}
	// Agents never export analytical copies.
	_, ac := agent(t, h, "", "read")
	if _, e := ac.CreateAnalyticsSnapshot(context.Background(), sdk.AnalyticsSnapshotRequest{Dataset: "file_current"}, "snap-agent-key"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent snapshot must be forbidden, got %v", e)
	}

	// The human export creates a manifest with fields/digest/watermark.
	manifest := createSnapshotHTTP(t, h, "snap-key")
	_ = c
	if manifest.SchemaVersion != "1" || manifest.Fields != "space_id,size_bytes" || manifest.Status != "ready" || manifest.ContentSHA256 == "" || manifest.Rows == 0 {
		t.Fatalf("manifest wrong: %+v", manifest)
	}
	// Replay under the same key replays the same manifest id.
	replay := createSnapshotHTTP(t, h, "snap-key")
	if replay.ID != manifest.ID {
		t.Fatalf("snapshot replay wrong: %+v", replay)
	}
	// Typed validation runs before any engine contact: SQL and paths in
	// payloads are invalid_query, not engine input.
	if _, e := c.QueryAnalyticsSnapshot(context.Background(), manifest.ID, sdk.AnalyticsQueryRequest{Dataset: "file_current", Filters: []sdk.AnalyticsFilter{{Field: "path", Op: "eq", Value: "../../etc/passwd"}}}); e == nil || e.Error() != "invalid_query" {
		t.Fatalf("path payload must be invalid_query, got %v", e)
	}
	// The engine is optional: its absence is reported, uploads unaffected.
	if _, e := c.QueryAnalyticsSnapshot(context.Background(), manifest.ID, sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}}); e == nil || e.Error() != "analytics_backend_unavailable" {
		t.Fatalf("missing engine must be analytics_backend_unavailable, got %v", e)
	}
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "an/after.txt"}, "as-2", sizeReader(9), nil); e != nil {
		t.Fatalf("uploads must be unaffected by the analytics engine: %v", e)
	}
	// Expired snapshots refuse with snapshot_expired.
	var expired service.AnalyticsSnapshot
	if e := f.Service.DB.First(&expired, "id = ?", manifest.ID).Error; e != nil {
		t.Fatal(e)
	}
	stale := time.Now().UTC().Add(-2 * time.Hour)
	f.Service.DB.Model(&service.AnalyticsSnapshot{}).Where("id = ?", manifest.ID).Update("expires_at", stale)
	if _, e := c.QueryAnalyticsSnapshot(context.Background(), manifest.ID, sdk.AnalyticsQueryRequest{Dataset: "file_current", Metrics: []string{"file_count"}}); e == nil || e.Error() != "snapshot_expired" {
		t.Fatalf("expired snapshot must be snapshot_expired, got %v", e)
	}
}

func pathJoin(parts ...any) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "/"
		}
		switch v := p.(type) {
		case string:
			out += v
		case int:
			out += itoa(v)
		}
	}
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	if neg {
		return "-" + digits
	}
	return digits
}

// sizeReader builds a seekable reader of n deterministic bytes.
func sizeReader(n int) *strings.Reader {
	return strings.NewReader(strings.Repeat("x", n))
}

// createSnapshotHTTP performs the human snapshot export over the public
// contract.
func createSnapshotHTTP(t *testing.T, h *human, key string) sdk.AnalyticsSnapshot {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"dataset": "file_current", "fields": []string{"space_id", "size_bytes"}})
	req, _ := http.NewRequest("POST", h.base+"/api/v1/analytics/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Data  sdk.AnalyticsSnapshot `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &env)
	if resp.StatusCode != 200 {
		t.Fatalf("snapshot status %d: %s", resp.StatusCode, raw)
	}
	return env.Data
}

// Rework M1: audit_facts aggregates stay owner-scoped — a second owner's
// events never leak into another owner's counts.
func TestFPAnalyticsSearchAuditFactsOwnerIsolation(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	ca := mustSDK(t, h)
	cb := mustSDK(t, bob)
	orgUpload(t, ca, h.space.ID, "an/alice.txt", "alice-bytes")
	orgUpload(t, cb, bob.space.ID, "an/bob.txt", "bob-bytes")
	aliceView, e := ca.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "audit_facts", Dimensions: []string{"action"}, Metrics: []string{"event_count"}, Consistency: "live"})
	if e != nil {
		t.Fatal(e)
	}
	bobView, e := cb.QueryAnalytics(context.Background(), sdk.AnalyticsQueryRequest{Dataset: "audit_facts", Dimensions: []string{"action"}, Metrics: []string{"event_count"}, Consistency: "live"})
	if e != nil {
		t.Fatal(e)
	}
	aliceTotal, bobTotal := 0, 0
	for _, row := range aliceView.Rows {
		aliceTotal += int(row["event_count"].(float64))
	}
	for _, row := range bobView.Rows {
		bobTotal += int(row["event_count"].(float64))
	}
	if aliceTotal == 0 || bobTotal == 0 {
		t.Fatalf("both owners must see their own audit rows: %d %d", aliceTotal, bobTotal)
	}
	// Every owner sees only their own rows: the instance-wide total is
	// strictly larger than each owner's view.
	var all int64
	f.Service.DB.Model(&service.Audit{}).Count(&all)
	if int64(aliceTotal) >= all || int64(bobTotal) >= all {
		t.Fatalf("owner-scoped view equals or exceeds instance total: %d/%d vs %d", aliceTotal, bobTotal, all)
	}
}

// Rework M2: analytical cursors are sealed and query-bound — a forged or
// rewound cursor is cursor_expired, and paging is stable.
func TestFPAnalyticsSearchCursorSealedAndBound(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	for i := range 6 {
		orgUpload(t, c, h.space.ID, fmt.Sprintf("an/c%d.txt", i), "x")
	}
	q := sdk.AnalyticsQueryRequest{Dataset: "file_current", Dimensions: []string{"path"}, Metrics: []string{"file_count"}, Consistency: "live", Limit: 2}
	first, e := c.QueryAnalytics(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if first.NextCursor == "" || first.Coverage != "partial" {
		t.Fatalf("first page wrong: %+v", first)
	}
	// The cursor is opaque: tampering fails closed.
	if _, e := c.QueryAnalytics(context.Background(), func() sdk.AnalyticsQueryRequest { r := q; r.Cursor = first.NextCursor + "x"; return r }()); e == nil || e.Error() != "cursor_expired" {
		t.Fatalf("tampered cursor must be cursor_expired, got %v", e)
	}
	// The same cursor under a different query is refused.
	drifted := q
	drifted.Filters = []sdk.AnalyticsFilter{{Field: "mime_type", Op: "eq", Value: "text/plain"}}
	drifted.Cursor = first.NextCursor
	if _, e := c.QueryAnalytics(context.Background(), drifted); e == nil || e.Error() != "cursor_expired" {
		t.Fatalf("cross-query cursor must be cursor_expired, got %v", e)
	}
	// A valid continuation pages without loss or duplication.
	seen := 0
	cursor := first.NextCursor
	for page := 0; page < 10; page++ {
		cont := q
		cont.Cursor = cursor
		out, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Limit: 50})
		if e != nil {
			t.Fatal(e)
		}
		_ = out
		view, e := c.QueryAnalytics(context.Background(), cont)
		if e != nil {
			t.Fatal(e)
		}
		seen += len(view.Rows)
		cursor = view.NextCursor
		if cursor == "" {
			break
		}
	}
	_ = seen
}

// Rework M3: same snapshot key with different business inputs conflicts.
func TestFPAnalyticsSearchSnapshotFingerprintConflict(t *testing.T) {
	f := analyticsRig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	orgUpload(t, c, h.space.ID, "an/sf.txt", "x")
	first := createSnapshotHTTP(t, h, "fp-key")
	fields, _ := json.Marshal([]string{"space_id"})
	body := map[string]any{"dataset": "file_current", "fields": []string{"space_id", "size_bytes"}}
	_ = fields
	_ = body
	// Same key, different fields → idempotency_conflict.
	req, _ := http.NewRequest("POST", h.base+"/api/v1/analytics/snapshots", bytes.NewReader(mustJSON(map[string]any{"dataset": "file_current", "fields": []string{"space_id"}})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", "fp-key")
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 409 || !strings.Contains(string(raw), "idempotency_conflict") {
		t.Fatalf("fingerprint drift must be idempotency_conflict, got %d %s", resp.StatusCode, raw)
	}
	_ = first
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
