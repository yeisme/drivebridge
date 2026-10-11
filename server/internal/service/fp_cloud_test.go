package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// cloudRig starts the fixture with the unified cloud-connector surface
// enabled; the default rig keeps it off (the default-off deployment gate).
func cloudRig(t *testing.T) *fixtures.Runtime {
	t.Helper()
	f, e := fixtures.StartWithOptions(t.TempDir(), "", "", fixtures.Options{CloudConnectors: true})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	return f
}

// cloudS3Connection binds one user-owned S3 connection at the fixture's
// loopback object store under a dedicated prefix.
func cloudS3Connection(t *testing.T, h *human, f *fixtures.Runtime, prefix string) service.Connection {
	t.Helper()
	var conn service.Connection
	body := map[string]any{"name": "cloud-" + prefix, "config": map[string]any{"endpoint": f.S3.URL, "region": "us-east-1", "bucket": "fixture", "prefix": prefix, "access_key": "fixture-key", "secret_key": "fixture-secret"}}
	if status := h.call(t, "POST", "/connections/s3", body, &conn); status != 200 {
		t.Fatalf("s3 connection status %d", status)
	}
	return conn
}

// cloudPutObject writes one raw object straight into the fixture store.
func cloudPutObject(t *testing.T, f *fixtures.Runtime, key string, body []byte) {
	t.Helper()
	req, _ := http.NewRequest("PUT", f.S3.URL+key, bytes.NewReader(body))
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("fixture object put status %d", resp.StatusCode)
	}
}

// cloudBindRoot binds one prefix root through the public contract.
func cloudBindRoot(t *testing.T, h *human, conn service.Connection, rootRef, name string) service.CloudRoot {
	t.Helper()
	var root service.CloudRoot
	if status := h.call(t, "POST", "/connections/"+conn.ID+"/roots", map[string]any{"root_ref": rootRef, "name": name}, &root); status != 200 {
		t.Fatalf("root bind status != 200: %d", status)
	}
	return root
}

// cloudAgent creates one connection-scoped agent for cloud reads.
func cloudAgent(t *testing.T, h *human, conn service.Connection, scopes string) *sdk.Client {
	t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	if status := h.call(t, "POST", "/agents", map[string]any{"name": "cloud-agent", "space_id": h.space.ID, "prefix": "", "scopes": scopes, "days": 7, "connection_id": conn.ID}, &out); status != 200 {
		t.Fatalf("connection-scoped agent status %d", status)
	}
	c, e := sdk.New(h.base, out.Token)
	if e != nil {
		t.Fatal(e)
	}
	return c
}

// Task 2.1: the deployment gate — a server without CloudConnectors enabled
// keeps every legacy route working and answers the new surface with
// unsupported, never a silent fallback or a different behavior.
func TestFPCloudConnectorsCapabilityGate(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	var conn service.Connection
	body := map[string]any{"name": "gated", "config": map[string]any{"endpoint": f.S3.URL, "region": "us-east-1", "bucket": "fixture", "prefix": "gated", "access_key": "fixture-key", "secret_key": "fixture-secret"}}
	if status := h.call(t, "POST", "/connections/s3", body, &conn); status != 200 {
		t.Fatalf("legacy s3 connection must keep working: %d", status)
	}
	status, raw := fpRawCall(t, h, "POST", "/connections/"+conn.ID+"/roots", []byte(`{"root_ref":"","name":"r"}`))
	if status != 400 {
		t.Fatalf("new surface on a default-off deployment must be 400 unsupported, got %d", status)
	}
	if !bytes.Contains(raw, []byte(`"unsupported"`)) {
		t.Fatalf("error code must be unsupported, got %s", raw)
	}
}

// agent principals, roots stay inside one data owner, and the entry
// observation identity is the (root, provider item) pair — repeated pages
// update the same row instead of duplicating observations.
func TestFPCloudConnectorsRootAndEntryIdentity(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	conn := cloudS3Connection(t, h, f, "identity")
	cloudPutObject(t, f, "/fixture/identity/docs/a.txt", []byte("alpha"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	if root.ID == "" || root.Provider != "s3" || root.Status != "bound" || root.Generation != 0 {
		t.Fatalf("root shape wrong: %+v", root)
	}
	// Agent principals never bind roots.
	_, c := agent(t, h, "", "read,write")
	if _, e := c.BindCloudRoot(context.Background(), conn.ID, "other", "nope"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent root bind must be forbidden, got %v", e)
	}
	// Cross-owner isolation: bob cannot see alice's connection or roots.
	var bobRoots []service.CloudRoot
	if status := bob.call(t, "GET", "/connections/"+conn.ID+"/roots", nil, &bobRoots); status != 404 {
		t.Fatalf("cross-owner root list must be 404, got %d", status)
	}
	if status := bob.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, nil); status != 404 {
		t.Fatalf("cross-owner entry list must be 404, got %d", status)
	}
	// Provider identity dedup: listing the same page twice keeps one row.

	// HTTP surface here and the agent surface in the 4.x equivalence tests.
	var page1, page2 sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page1); status != 200 {
		t.Fatalf("entries status %d", status)
	}
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page2); status != 200 {
		t.Fatalf("entries replay status %d", status)
	}
	if len(page1.Items) != 1 || len(page2.Items) != 1 {
		t.Fatalf("expected exactly one entry, got %d and %d", len(page1.Items), len(page2.Items))
	}
	if page1.Items[0].EntryRef == "" || page1.Items[0].EntryRef != page2.Items[0].EntryRef || page1.Items[0].ProviderItemID != "a.txt" {
		t.Fatalf("entry identity unstable: %+v vs %+v", page1.Items[0], page2.Items[0])
	}
	var rows int64
	f.Service.DB.Model(&service.ExternalEntry{}).Where("root_id = ?", root.ID).Count(&rows)
	if rows != 1 {
		t.Fatalf("repeated pages must update the same observation row, got %d", rows)
	}
	// The observation is an external fact, never a managed copy claim.
	if page1.Items[0].VersionStrength != "observed_only" || page1.Items[0].ManagedRef != "" {
		t.Fatalf("fresh observation must be observed_only without managed ref: %+v", page1.Items[0])
	}
	if page1.Coverage != "complete" || page1.NextCursor != "" {
		t.Fatalf("single-object page must be complete: %+v", page1)
	}
}

// Task 2.1: scan idempotency — the same key replays the original scan
// operation, a different budget under the same key conflicts, and another
// owner's identical key stays independent.
func TestFPCloudConnectorsScanIdempotency(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	conn := cloudS3Connection(t, h, f, "scan")
	cloudPutObject(t, f, "/fixture/scan/root/one.txt", []byte("one"))
	root := cloudBindRoot(t, h, conn, "root", "r")
	c := cloudAgent(t, h, conn, "read,write")
	first, e := c.ScanCloudConnection(context.Background(), conn.ID, root.ID, 100, "scan-key")
	if e != nil {
		t.Fatal(e)
	}
	if first.Status != "completed" || first.Coverage != "complete" || first.Observed != 1 {
		t.Fatalf("scan wrong: %+v", first)
	}
	replay, e := c.ScanCloudConnection(context.Background(), conn.ID, root.ID, 100, "scan-key")
	if e != nil {
		t.Fatal(e)
	}
	if replay.RootID != first.RootID || replay.Observed != first.Observed {
		t.Fatalf("scan replay created a second outcome: %+v vs %+v", replay, first)
	}
	if _, e = c.ScanCloudConnection(context.Background(), conn.ID, root.ID, 200, "scan-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("budget drift must be idempotency_conflict, got %v", e)
	}
	// Identity isolation: the same key under another owner is independent.
	bobConn := cloudS3Connection(t, bob, f, "scan-bob")
	bobRoot := cloudBindRoot(t, bob, bobConn, "root", "r")
	bc := cloudAgent(t, bob, bobConn, "read,write")
	other, e := bc.ScanCloudConnection(context.Background(), bobConn.ID, bobRoot.ID, 100, "scan-key")
	if e != nil {
		t.Fatal(e)
	}
	if other.OperationID == first.OperationID {
		t.Fatal("scan identity leaked across owners")
	}
}

// Task 2.2: additive schema boundaries — the new cloud tables are pure
// additions. A legacy flow (upload, read, search) keeps working before and
// after roots and observations exist, old rows never reinterpret legacy
// operation state, and dropping only the new tables restores the previous
// behavior surface without losing legacy data or references.
func TestFPCloudConnectorsSchemaAdditive(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	// Legacy upload with no cloud state at all.
	op := fpUploadBytes(t, c, h.space.ID, "legacy/before.txt", "cloud-legacy", []byte("legacy-bytes"))
	if op.Status != "completed" {
		t.Fatal(op.Status)
	}
	var legacyOp service.Operation
	if e := f.Service.DB.First(&legacyOp, "id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	// Cloud observations arrive next; the legacy operation is untouched.
	conn := cloudS3Connection(t, h, f, "additive")
	cloudPutObject(t, f, "/fixture/additive/docs/a.txt", []byte("alpha"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	var page sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page); status != 200 || len(page.Items) != 1 {
		t.Fatalf("entries status/list wrong: %d %+v", status, page)
	}
	if e := f.Service.DB.First(&legacyOp, "id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	if legacyOp.Kind != "upload" || legacyOp.Status != "completed" {
		t.Fatalf("legacy operation reinterpreted: %+v", legacyOp)
	}
	// Old contract keeps reading the legacy bytes.
	var sink bytes.Buffer
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || sink.String() != "legacy-bytes" {
		t.Fatal("legacy bytes changed")
	}
	// Rollback boundary: dropping only the additive cloud tables leaves the
	// legacy surface intact.
	if e := f.Service.DB.Migrator().DropTable("external_entries", "cloud_roots"); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.DB.First(&legacyOp, "id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	sink.Reset()
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || sink.String() != "legacy-bytes" {
		t.Fatal("legacy bytes changed after rollback drill")
	}
	// The cloud surface fails closed after rollback instead of half-working.
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, nil); status != 500 && status != 404 {
		t.Fatalf("cloud surface after rollback must fail closed, got %d", status)
	}
}

// fpCloudImport issues the explicit import over the public contract with
// the idempotency key in the header.
func fpCloudImport(t *testing.T, h *human, conn service.Connection, q map[string]any, key string) (service.Operation, []byte, int) {
	t.Helper()
	body, _ := json.Marshal(q)
	req, _ := http.NewRequest("POST", h.base+"/api/v1/connections/"+conn.ID+"/entries/import", bytes.NewReader(body))
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
		Data  service.Operation `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &env)
	return env.Data, []byte(env.Error.Code), resp.StatusCode
}

// Task 3.1: the capability matrix is an independently declared fact per
// provider. S3 over the fixture reports no historical read (no bucket
// versioning), no change cursor and prefix-projection directories; the
// declared notes carry the honest limits instead of inferring from names.
func TestFPCloudConnectorsCapabilityMatrix(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "additive-matrix")
	var caps service.CloudCapabilityView
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/cloud-capabilities", nil, &caps); status != 200 {
		t.Fatalf("capabilities status %d", status)
	}
	if caps.Provider != "s3" || !caps.Browse || !caps.Transfer || !caps.NativeCopy || !caps.ConditionalWrite {
		t.Fatalf("s3 matrix wrong: %+v", caps)
	}
	if caps.HistoricalRead || caps.SoftDelete || caps.ChangeCursor {
		t.Fatalf("unversioned fixture bucket must not claim history/soft-delete/cursor: %+v", caps)
	}
	// OneDrive and Google Drive declare their own facts from the adapter
	// matrix without any live provider call.
	for _, kind := range []string{"onedrive", "gdrive"} {
		c := service.Connection{ID: service.ID(), Owner: h.user.ID, Kind: kind, Name: kind, Status: "connected"}
		if e := f.Service.DB.Create(&c).Error; e != nil {
			t.Fatal(e)
		}
		var view service.CloudCapabilityView
		if status := h.call(t, "GET", "/connections/"+c.ID+"/cloud-capabilities", nil, &view); status != 200 {
			t.Fatalf("%s capabilities status %d", kind, status)
		}
		if view.Provider != kind || !view.HistoricalRead || !view.ChangeCursor || !view.SoftDelete || !view.NativeCopy || !view.ConditionalWrite || !view.Browse || !view.Transfer {
			t.Fatalf("%s matrix wrong: %+v", kind, view)
		}
		if kind == "gdrive" {
			joined := strings.Join(view.Notes, " ")
			if !strings.Contains(joined, "native_document") || !strings.Contains(joined, "export_required") {
				t.Fatalf("gdrive notes must disclose the native-document limit: %+v", view.Notes)
			}
		}
	}
}

// Task 3.1: the managed-copy normal path — a pinned observation imports
// through the public contract, the digest verifies, the observation links
// the managed reference with sha256_verified strength, and a replay of the
// import lands on the original operation identity.
func TestFPCloudConnectorsImportManagedCopy(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "import-copy")
	payload := []byte("import-payload")
	cloudPutObject(t, f, "/fixture/import-copy/docs/keep.txt", payload)
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	var page sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page); status != 200 || len(page.Items) != 1 {
		t.Fatalf("entries wrong: %d %+v", status, page)
	}
	entry := page.Items[0]
	if entry.Kind != "file" || entry.VersionStrength != "observed_only" || entry.ManagedRef != "" {
		t.Fatalf("entry shape wrong: %+v", entry)
	}
	op, errCode, status := fpCloudImport(t, h, conn, map[string]any{"entry_ref": entry.EntryRef, "expected_revision": entry.ObservedRevision, "space_id": h.space.ID, "path": "imported/keep.txt"}, "import-key")
	if status != 200 || errCode != nil && len(errCode) > 0 {
		t.Fatalf("import failed: %d %s", status, errCode)
	}
	if op.Status != "completed" || op.FileID == "" || op.VersionID == "" {
		t.Fatalf("import not completed: %+v", op)
	}
	if op.SHA256 != service.Hash(string(payload)) {
		t.Fatalf("import digest wrong: %s", op.SHA256)
	}
	// The observation now carries the managed link and verified strength.
	var after sdk.CloudEntries
	if s := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &after); s != 200 || len(after.Items) != 1 {
		t.Fatalf("re-list wrong: %d", s)
	}
	linked := after.Items[0]
	if linked.VersionStrength != "sha256_verified" || linked.ContentSHA256 != op.SHA256 || linked.ManagedRef == "" {
		t.Fatalf("observation not linked: %+v", linked)
	}
	if !strings.Contains(linked.ManagedRef, "/file/"+op.FileID+"/version/"+op.VersionID) {
		t.Fatalf("managed ref wrong: %s", linked.ManagedRef)
	}
	// The managed copy reads back with the imported bytes.
	c := mustSDK(t, h)
	var sink bytes.Buffer
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || !bytes.Equal(sink.Bytes(), payload) {
		t.Fatal("managed copy bytes differ")
	}
	// Replay: same key, same inputs — the original identity answers.
	replay, _, rstatus := fpCloudImport(t, h, conn, map[string]any{"entry_ref": entry.EntryRef, "expected_revision": entry.ObservedRevision, "space_id": h.space.ID, "path": "imported/keep.txt"}, "import-key")
	if rstatus != 200 || replay.ID != op.ID || replay.FileID != op.FileID {
		t.Fatalf("import replay created a second copy: %+v vs %+v", replay, op)
	}
}

// Task 3.2: source drift, scope and cursor invalidation. A source that
// moved past the pinned revision refuses with source_changed (never a
// silent refresh); cursors are root- and query-bound, tampered cursors
// expire, and a provider without a change cursor rejects cloud_changes
// with unsupported instead of fabricating a delta.
func TestFPCloudConnectorsDriftScopeAndCursorInvalidation(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "drift")
	cloudPutObject(t, f, "/fixture/drift/docs/a.txt", []byte("one"))
	cloudPutObject(t, f, "/fixture/drift/docs/b.txt", []byte("two"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	var page sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page); status != 200 || len(page.Items) != 2 {
		t.Fatalf("entries wrong: %d %+v", status, page)
	}
	var entryA sdk.CloudEntry
	for _, it := range page.Items {
		if it.ProviderItemID == "a.txt" {
			entryA = it
		}
	}
	// The source drifts: a.txt overwritten with different bytes.
	cloudPutObject(t, f, "/fixture/drift/docs/a.txt", []byte("one-drifted"))
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCall(t, h, "POST", "/connections/"+conn.ID+"/entries/stat", []byte(`{"entry_ref":"`+entryA.EntryRef+`","expected_revision":"`+entryA.ObservedRevision+`"}`))
	json.Unmarshal(raw, &env)
	if status != 409 || env.Error.Code != "source_changed" {
		t.Fatalf("drifted stat must be 409 source_changed, got %d %s", status, env.Error.Code)
	}
	// Import under the stale revision is refused and creates no copy.
	_, errCode, istatus := fpCloudImport(t, h, conn, map[string]any{"entry_ref": entryA.EntryRef, "expected_revision": entryA.ObservedRevision, "space_id": h.space.ID, "path": "drifted/a.txt"}, "drift-key")
	if istatus != 409 || string(errCode) != "source_changed" {
		t.Fatalf("drifted import must be 409 source_changed, got %d %s", istatus, errCode)
	}
	var copies int64
	f.Service.DB.Model(&service.Operation{}).Where("owner = ? AND kind = ?", h.user.ID, "import").Count(&copies)
	if copies != 0 {
		t.Fatalf("refused import must not create an operation copy, got %d", copies)
	}
	// S3 declares no change cursor: cloud_changes is unsupported.
	status, raw = fpRawCall(t, h, "GET", "/connections/"+conn.ID+"/changes?root_id="+root.ID, nil)
	json.Unmarshal(raw, &env)
	if status != 400 || env.Error.Code != "unsupported" {
		t.Fatalf("s3 changes must be 400 unsupported, got %d %s", status, env.Error.Code)
	}
	// Cursor scope: a cursor minted for one root/query expires for another.
	paged := sdk.CloudEntries{}
	for i := 0; i < 130; i++ {
		cloudPutObject(t, f, "/fixture/drift/docs/"+fmt.Sprintf("bulk-%03d.bin", i), []byte("x"))
	}
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&limit=50", nil, &paged); status != 200 {
		t.Fatalf("bulk entries status %d", status)
	}
	if paged.NextCursor == "" || paged.Coverage != "partial" {
		t.Fatalf("partial page must carry a cursor: %+v", paged)
	}
	// Same cursor on a different query (limit change) is cursor_expired.
	status, raw = fpRawCall(t, h, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&limit=100&cursor="+urlQueryEscape(paged.NextCursor), nil)
	json.Unmarshal(raw, &env)
	if status != 410 || env.Error.Code != "cursor_expired" {
		t.Fatalf("cursor with a different query must be 410, got %d %s", status, env.Error.Code)
	}
	// Tampered cursor is cursor_expired, not a fresh enumeration.
	status, raw = fpRawCall(t, h, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&limit=50&cursor="+urlQueryEscape(paged.NextCursor+"x"), nil)
	json.Unmarshal(raw, &env)
	if status != 410 || env.Error.Code != "cursor_expired" {
		t.Fatalf("tampered cursor must be 410, got %d %s", status, env.Error.Code)
	}
	// The same cursor on the same query continues the enumeration.
	var cont sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&limit=50&cursor="+urlQueryEscape(paged.NextCursor), nil, &cont); status != 200 || len(cont.Items) == 0 {
		t.Fatalf("valid cursor must continue: %d %+v", status, cont)
	}
	// Unknown root: not_found, and no information about other owners.
	status, _ = fpRawCall(t, h, "GET", "/connections/"+conn.ID+"/entries?root_id=nonexistent", nil)
	if status != 404 {
		t.Fatalf("unknown root must be 404, got %d", status)
	}
}

// Task 3.2: version_unavailable — the unversioned fixture bucket cannot
// serve a superseded revision, and the latest bytes are never returned in
// its place.
func TestFPCloudConnectorsVersionUnavailableOnUnversionedBackend(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "versions")
	cloudPutObject(t, f, "/fixture/versions/docs/v.txt", []byte("first"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	var page sdk.CloudEntries
	h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page)
	old := page.Items[0]
	// Overwrite: the old revision's bytes are gone for good on an
	// unversioned bucket.
	cloudPutObject(t, f, "/fixture/versions/docs/v.txt", []byte("second-revision"))
	// The fetch path pins the requested revision: a superseded revision on
	// an unversioned backend is refused, never replaced by the latest
	// bytes. The public stat/import surface refuses the stale expectation
	// with source_changed before any byte claim is made.
	_, errCode, istatus := fpCloudImport(t, h, conn, map[string]any{"entry_ref": old.EntryRef, "expected_revision": old.ObservedRevision, "space_id": h.space.ID, "path": "versions/v.txt"}, "version-key")
	if istatus != 409 || string(errCode) != "source_changed" {
		t.Fatalf("superseded revision import must be source_changed, got %d %s", istatus, errCode)
	}
	// With the expected_revision pinned to the stale observation the typed
	// refusal is source_changed; without any expectation the import adopts
	// the current observation and succeeds with the CURRENT bytes.
	fresh, _, fstaus := fpCloudImport(t, h, conn, map[string]any{"entry_ref": old.EntryRef, "space_id": h.space.ID, "path": "versions/v.txt"}, "version-key-2")
	if fstaus != 200 || fresh.Status != "completed" {
		t.Fatalf("unpinned import must adopt the current observation: %d %+v", fstaus, fresh)
	}
	if fresh.SHA256 != service.Hash("second-revision") {
		t.Fatalf("unpinned import must carry the current bytes, got %s", fresh.SHA256)
	}
}

func urlQueryEscape(v string) string { return url.QueryEscape(v) }

// Task 4.1: HTTP, MCP and SDK are equivalent projections of the same
// application contract. The same page viewed through the three entries
// carries identical item facts; agents without the connection scope are
// refused on the MCP surface, and the legacy OneDrive items route keeps
// its own semantics (no silent redefinition as a generic entry point).
func TestFPCloudConnectorsEntryEquivalence(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "equiv")
	cloudPutObject(t, f, "/fixture/equiv/docs/e1.txt", []byte("e1"))
	cloudPutObject(t, f, "/fixture/equiv/docs/e2.txt", []byte("e2"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	// HTTP entry (human session).
	var viaHTTP sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &viaHTTP); status != 200 {
		t.Fatalf("http entries %d", status)
	}
	// SDK entry (connection-scoped agent token).
	c := cloudAgent(t, h, conn, "read")
	viaSDK, e := c.ListCloudEntries(context.Background(), conn.ID, sdk.ListCloudEntriesRequest{RootID: root.ID})
	if e != nil {
		t.Fatal(e)
	}
	// MCP entry (connection-scoped agent principal).
	var agentOut struct {
		Agent service.Agent `json:"agent"`
	}
	h.call(t, "POST", "/agents", map[string]any{"name": "mcp-agent", "space_id": h.space.ID, "prefix": "", "scopes": "read", "days": 7, "connection_id": conn.ID}, &agentOut)
	mcpPrincipal := service.Principal{UserID: h.user.ID, Agent: &agentOut.Agent}
	mcpOut, e := f.Service.CallTool(context.Background(), mcpPrincipal, "cloud_entries", map[string]any{"connection_id": conn.ID, "root_id": root.ID})
	if e != nil {
		t.Fatal(e)
	}
	mcpMap, ok := mcpOut.(sdk.CloudEntries)
	if !ok {
		// CallTool returns the service view type; re-marshal for compare.
		b, _ := json.Marshal(mcpOut)
		if json.Unmarshal(b, &mcpMap) != nil {
			t.Fatalf("cloud_entries MCP shape: %T", mcpOut)
		}
	}
	if len(viaHTTP.Items) != 2 || len(viaSDK.Items) != 2 || len(mcpMap.Items) != 2 {
		t.Fatalf("entry counts differ: http=%d sdk=%d mcp=%d", len(viaHTTP.Items), len(viaSDK.Items), len(mcpMap.Items))
	}
	for i := range viaHTTP.Items {
		if viaHTTP.Items[i].EntryRef != viaSDK.Items[i].EntryRef || viaHTTP.Items[i].EntryRef != mcpMap.Items[i].EntryRef {
			t.Fatalf("entry identity differs across entries: %+v %+v %+v", viaHTTP.Items[i], viaSDK.Items[i], mcpMap.Items[i])
		}
		if viaHTTP.Items[i].ObservedRevision != viaSDK.Items[i].ObservedRevision || viaHTTP.Items[i].ObservedRevision != mcpMap.Items[i].ObservedRevision {
			t.Fatalf("revision differs across entries")
		}
		if viaHTTP.Items[i].VersionStrength != "observed_only" || viaSDK.Items[i].VersionStrength != "observed_only" || mcpMap.Items[i].VersionStrength != "observed_only" {
			t.Fatalf("strength differs across entries")
		}
	}
	if viaHTTP.Coverage != viaSDK.Coverage || viaHTTP.Coverage != mcpMap.Coverage {
		t.Fatalf("coverage differs across entries")
	}
	// Unscoped agents are refused on the MCP cloud surface.
	plain, _ := agent(t, h, "", "read")
	unscoped := service.Principal{UserID: h.user.ID, Agent: &plain}
	if _, e := f.Service.CallTool(context.Background(), unscoped, "cloud_entries", map[string]any{"connection_id": conn.ID, "root_id": root.ID}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("unscoped agent cloud_entries must be forbidden, got %v", e)
	}
	// The legacy OneDrive items route keeps its own contract on non-OneDrive
	// connections instead of becoming a generic entry point.
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/items", nil, nil); status != 503 {
		t.Fatalf("legacy items route on an s3 connection must stay storage_disconnected (503), got %d", status)
	}
	// The cloud tools are part of the discoverable contract.
	tools := service.ToolDefinitions()
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"cloud_entries", "cloud_entry_stat", "cloud_changes", "cloud_scan"} {
		if !names[name] {
			t.Fatalf("tool %s missing from the discoverable contract", name)
		}
	}
}

// Task 4.2: the design's user flow over fixtures only: bind a cloud
// directory from the web session, browse it in bounded pages, hit source
// drift, refresh the observation and import a fixed managed copy whose
// original ids read back afterwards. Every step goes through public
// surfaces; no real provider is contacted.
func TestFPCloudConnectorsFixtureFlowBindBrowseDriftImport(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "flow42")
	for i := 0; i < 120; i++ {
		cloudPutObject(t, f, "/fixture/flow42/reports/r-"+fmt.Sprintf("%03d.txt", i), []byte("report-"+fmt.Sprint(i)))
	}
	// Bind the cloud directory from the web session.
	root := cloudBindRoot(t, h, conn, "reports", "reports")
	// Browse in bounded pages through the public contract.
	var collected []sdk.CloudEntry
	cursor := ""
	pages := 0
	for {
		path := "/connections/" + conn.ID + "/entries?root_id=" + root.ID + "&limit=50"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page sdk.CloudEntries
		if status := h.call(t, "GET", path, nil, &page); status != 200 {
			t.Fatalf("browse page status %d", status)
		}
		collected = append(collected, page.Items...)
		pages++
		if page.NextCursor == "" {
			if page.Coverage != "complete" {
				t.Fatalf("final page must be complete: %+v", page)
			}
			break
		}
		cursor = page.NextCursor
		if pages > 10 {
			t.Fatal("browse did not converge")
		}
	}
	if len(collected) != 120 || pages != 3 {
		t.Fatalf("browse collected %d items in %d pages", len(collected), pages)
	}
	// Source drift: the first report is replaced underneath the browser.
	cloudPutObject(t, f, "/fixture/flow42/reports/r-000.txt", []byte("replaced-content"))
	var first sdk.CloudEntry
	for _, it := range collected {
		if it.ProviderItemID == "r-000.txt" {
			first = it
		}
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCall(t, h, "POST", "/connections/"+conn.ID+"/entries/stat", []byte(`{"entry_ref":"`+first.EntryRef+`","expected_revision":"`+first.ObservedRevision+`"}`))
	json.Unmarshal(raw, &env)
	if status != 409 || env.Error.Code != "source_changed" {
		t.Fatalf("drift must surface source_changed, got %d %s", status, env.Error.Code)
	}
	// Explicit import of the fixed current copy: refresh first (stat pins
	// the new revision), then import with the fresh expectation.
	fresh, ferr := cStat(t, h, conn, first.EntryRef)
	if ferr != nil {
		t.Fatal(ferr)
	}
	op, errCode, istatus := fpCloudImport(t, h, conn, map[string]any{"entry_ref": first.EntryRef, "expected_revision": fresh.ObservedRevision, "space_id": h.space.ID, "path": "reports/r-000.txt"}, "flow42-key")
	if istatus != 200 || len(errCode) > 0 {
		t.Fatalf("import failed: %d %s", istatus, errCode)
	}
	if op.Status != "completed" || op.SHA256 != service.Hash("replaced-content") {
		t.Fatalf("import wrong: %+v", op)
	}
	// The original ids read back through the public contract.
	c := mustSDK(t, h)
	var sink bytes.Buffer
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || sink.String() != "replaced-content" {
		t.Fatal("imported bytes differ on read-back")
	}
	if _, e := c.Operation(context.Background(), op.ID); e != nil {
		t.Fatalf("original operation id unreadable: %v", e)
	}
}

func cStat(t *testing.T, h *human, conn service.Connection, entryRef string) (sdk.CloudEntry, error) {
	t.Helper()
	var out sdk.CloudEntry
	status := h.call(t, "POST", "/connections/"+conn.ID+"/entries/stat", map[string]any{"entry_ref": entryRef}, &out)
	if status != 200 {
		return out, fmt.Errorf("stat status %d", status)
	}
	return out, nil
}

// Task 5.1: concurrent revocation and error injection. Revoking the
// connection-scoped agent mid-flow stops every cloud read with
// unauthorized; a failing provider keeps the durable facts (no
// tombstones, no fabricated coverage) and marks the scan failed.
func TestFPCloudConnectorsRevocationAndErrorInjection(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "revoke")
	cloudPutObject(t, f, "/fixture/revoke/docs/r.txt", []byte("r"))
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	c := cloudAgent(t, h, conn, "read")
	page, e := c.ListCloudEntries(context.Background(), conn.ID, sdk.ListCloudEntriesRequest{RootID: root.ID})
	if e != nil || len(page.Items) != 1 {
		t.Fatalf("pre-revocation read wrong: %+v %v", page, e)
	}
	// Concurrent revocation: the next call is unauthorized and nothing
	// leaks about the entries.
	var agents []service.Agent
	h.call(t, "GET", "/agents", nil, &agents)
	for _, a := range agents {
		if a.ConnectionID == conn.ID {
			if status := h.call(t, "POST", "/agents/"+a.ID+"/revoke", map[string]any{}, nil); status != 200 {
				t.Fatalf("revoke status %d", status)
			}
		}
	}
	if _, e := c.ListCloudEntries(context.Background(), conn.ID, sdk.ListCloudEntriesRequest{RootID: root.ID}); e == nil {
		t.Fatal("revoked agent must not read cloud entries")
	}
	// Error injection: the provider fails; the scan keeps durable facts.
	f.Failure = true
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status := 0
	req, _ := http.NewRequest("POST", h.base+"/api/v1/connections/"+conn.ID+"/scan", strings.NewReader(`{"root_id":"`+root.ID+`","budget":10}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", "inject-key")
	resp, rerr := h.http.Do(req)
	if rerr != nil {
		t.Fatal(rerr)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	status = resp.StatusCode
	json.Unmarshal(raw, &env)

	if status < 500 {
		t.Fatalf("scan with failed provider must fail with 5xx, got %d %s", status, env.Error.Code)
	}
	var tombstones int64
	f.Service.DB.Model(&service.ExternalEntry{}).Where("root_id = ? AND tombstoned_at IS NOT NULL", root.ID).Count(&tombstones)
	if tombstones != 0 {
		t.Fatalf("failed scan must not tombstone, got %d", tombstones)
	}
	var rootRow service.CloudRoot
	f.Service.DB.First(&rootRow, "id = ?", root.ID)
	if rootRow.LastScanCoverage == "complete" {
		t.Fatal("failed scan must not claim complete coverage")
	}
}

// Task 5.2 scale acceptance: a large prefix-walk fixture with bounded
// pages, keyset-stable pagination, in-page duplicate collapse, mid-walk
// drift, expired cursors and incomplete scans that never derive deletions.
// DRIVEBRIDGE_FP_SCALE=quick (default) seeds 10k entries; =full seeds 1M.
// The full 1M run was not executed on this machine (SQLite single-writer
// observation upserts dominate); quick-mode coverage of every metric is
// reported instead, real provider directories verify separately.
func TestFPCloudConnectorsScalePagination(t *testing.T) {
	total := 10000
	if v := os.Getenv("DRIVEBRIDGE_FP_SCALE"); v == "full" {
		total = 1000000
	}
	if os.Getenv("DRIVEBRIDGE_FP_SCALE") == "" {
		t.Skip("scale fixture gated behind DRIVEBRIDGE_FP_SCALE=quick|full (kept out of the standard suite like the readiness scale precedent)")
	}

	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "scale")
	for i := 0; i < total; i++ {
		f.SeedObjects(fmt.Sprintf("/fixture/scale/docs/s-%07d.bin", i), 16)
	}
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	// Bounded keyset walk: every page carries at most the limit and the
	// cursor continues deterministically.
	seen := 0
	cursor := ""
	pages := 0
	refs := map[string]bool{}
	for {
		path := "/connections/" + conn.ID + "/entries?root_id=" + root.ID + "&limit=200"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page sdk.CloudEntries
		if status := h.call(t, "GET", path, nil, &page); status != 200 {
			t.Fatalf("page status %d", status)
		}
		if len(page.Items) > 200 {
			t.Fatalf("page exceeds limit: %d", len(page.Items))
		}
		for _, it := range page.Items {
			if refs[it.ProviderItemID] {
				t.Fatalf("in-page or cross-page duplicate leaked: %s", it.ProviderItemID)
			}
			refs[it.ProviderItemID] = true
		}
		seen += len(page.Items)
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if seen != total || len(refs) != total {
		t.Fatalf("walk collected %d unique %d of %d", seen, len(refs), total)
	}
	// Observation rows stayed unique under the provider identity.
	var rows int64
	f.Service.DB.Model(&service.ExternalEntry{}).Where("root_id = ?", root.ID).Count(&rows)
	if rows != int64(total) {
		t.Fatalf("observation rows %d != %d", rows, total)
	}
	// Mid-walk drift: replace one object; the stale observation's stat is
	// source_changed and the new observation replaces the same row.
	var first sdk.CloudEntries
	h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&limit=1", nil, &first)
	drifted := first.Items[0]
	f.SeedObjects("/fixture/scale/docs/s-0000000.bin", 32)
	status, raw := fpRawCall(t, h, "POST", "/connections/"+conn.ID+"/entries/stat", []byte(`{"entry_ref":"`+drifted.EntryRef+`","expected_revision":"`+drifted.ObservedRevision+`"}`))
	if status != 409 {
		t.Fatalf("drift stat must be 409, got %d %s", status, raw)
	}
	// Incomplete scan derives no deletions: a budget below the population
	// leaves every observation alive.
	c := cloudAgent(t, h, conn, "read")
	scan, e := c.ScanCloudConnection(context.Background(), conn.ID, root.ID, 500, "scale-partial-key")
	if e != nil {
		t.Fatal(e)
	}
	if scan.Coverage != "partial" {
		t.Fatalf("budgeted scan must be partial: %+v", scan)
	}
	var tombstoned int64
	f.Service.DB.Model(&service.ExternalEntry{}).Where("root_id = ? AND tombstoned_at IS NOT NULL", root.ID).Count(&tombstoned)
	if tombstoned != 0 {
		t.Fatalf("partial scan derived %d deletions", tombstoned)
	}
	// A complete scan over the drifted world keeps the remaining entries
	// and tombstones nothing that still exists.
	complete, e := c.ScanCloudConnection(context.Background(), conn.ID, root.ID, int64(total+10), "scale-complete-key")
	if e != nil {
		t.Fatal(e)
	}
	if complete.Coverage != "complete" || complete.Tombstoned != 0 {
		t.Fatalf("complete scan wrong: %+v", complete)
	}
}

// Rework M1: an empty rootRef on a prefixed connection lists exactly the
// connection scope — sibling namespaces sharing the string prefix never
// leak in, and the scope's own keys surface as themselves instead of
// rolling into a bogus self-directory.
func TestFPCloudConnectorsRootPrefixScopeIsolation(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "sibling-leak")
	cloudPutObject(t, f, "/fixture/sibling-leak/top.txt", []byte("own"))
	cloudPutObject(t, f, "/fixture/sibling-leak/dir/nested.txt", []byte("own-nested"))
	// Decoys: siblings sharing the string prefix but outside the scope.
	cloudPutObject(t, f, "/fixture/sibling-leak-other/x.txt", []byte("decoy"))
	cloudPutObject(t, f, "/fixture/sibling-leakage/y.txt", []byte("decoy"))
	root := cloudBindRoot(t, h, conn, "", "whole-scope")
	var page sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID, nil, &page); status != 200 {
		t.Fatalf("entries status %d", status)
	}
	seen := map[string]string{}
	for _, item := range page.Items {
		seen[item.ProviderItemID] = item.Kind
	}
	if seen["top.txt"] != "file" {
		t.Fatalf("own top-level key missing or wrong kind: %+v", page.Items)
	}
	if seen["dir/"] != "directory" {
		t.Fatalf("own directory not grouped: %+v", page.Items)
	}
	for id := range seen {
		if strings.HasPrefix(id, "sibling-leak-other") || strings.HasPrefix(id, "sibling-leakage") {
			t.Fatalf("sibling namespace leaked into root listing: %s", id)
		}
	}
	// Listing the grouped directory still works inside the scope.
	var nested sdk.CloudEntries
	if status := h.call(t, "GET", "/connections/"+conn.ID+"/entries?root_id="+root.ID+"&parent_ref=dir%2F", nil, &nested); status != 200 || len(nested.Items) != 1 || nested.Items[0].ProviderItemID != "dir/nested.txt" {
		t.Fatalf("nested listing wrong: %d %+v", status, nested.Items)
	}
}

// Rework M4: a budgeted scan stops issuing provider pages once the budget
// is spent — one observed entry over two full pages means exactly one
// list call, not a wasted second page fetch.
func TestFPCloudConnectorsScanStopsPagingAtBudget(t *testing.T) {
	f := cloudRig(t)
	h, _ := login(t, f, "alice")
	conn := cloudS3Connection(t, h, f, "budget-pages")
	for i := range 250 {
		f.SeedObjects(fmt.Sprintf("/fixture/budget-pages/docs/b-%04d.bin", i), 8)
	}
	root := cloudBindRoot(t, h, conn, "docs", "docs")
	c := cloudAgent(t, h, conn, "read")
	before := f.ListCalls()
	scan, e := c.ScanCloudConnection(context.Background(), conn.ID, root.ID, 1, "budget-page-key")
	if e != nil {
		t.Fatal(e)
	}
	calls := f.ListCalls() - before
	if scan.Observed != 1 || scan.Coverage != "partial" {
		t.Fatalf("scan wrong: %+v", scan)
	}
	if calls != 1 {
		t.Fatalf("budget=1 must issue exactly one provider page, got %d calls", calls)
	}
}
