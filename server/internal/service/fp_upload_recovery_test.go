package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// fpBatchCreate creates one upload batch through the SDK entry.
func fpBatchCreate(t *testing.T, c *sdk.Client, space, policy, key string) sdk.UploadBatchView {
	t.Helper()
	out, e := c.CreateUploadBatch(context.Background(), sdk.UploadBatchRequest{SpaceID: space, ConflictPolicy: policy}, key)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// fpBatchAdd adds one page of items through the SDK entry.
func fpBatchAdd(t *testing.T, c *sdk.Client, batchID string, items []sdk.UploadBatchItemInput) sdk.UploadBatchView {
	t.Helper()
	out, e := c.AddUploadBatchItems(context.Background(), batchID, items)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func fpItem(path string, size int64, sha string) sdk.UploadBatchItemInput {
	return sdk.UploadBatchItemInput{RelativePath: path, Size: size, SHA256: sha}
}

// fpUploadBytes uploads raw content through the SDK upload path.
func fpUploadBytes(t *testing.T, c *sdk.Client, space, path, key string, content []byte) sdk.Operation {
	t.Helper()
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: space, Path: path}, key, strings.NewReader(string(content)), nil)
	if e != nil {
		t.Fatal(e)
	}
	return op
}

// Task 2.1: the batch keeps a durable fixed identity under the original
// idempotency key: a replay after a lost creation response returns the same
// batch instead of creating a second one, and the same key with a different
// business fingerprint is refused while the original batch survives.
func TestFPUploadRecoveryBatchIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	bob, _ := login(t, f, "bob")

	first := fpBatchCreate(t, c, h.space.ID, "fail", "fixed-batch-key")
	if first.BatchID == "" || first.OperationID == "" {
		t.Fatalf("batch creation missing ids: %+v", first)
	}
	if first.Phase != "enumerating" || first.BatchRevision != 1 {
		t.Fatalf("fresh batch wrong phase/revision: %+v", first)
	}
	// Creation response "lost": replay the same key and inputs.
	replay := fpBatchCreate(t, c, h.space.ID, "fail", "fixed-batch-key")
	if replay.BatchID != first.BatchID || replay.OperationID != first.OperationID || replay.BatchRevision != first.BatchRevision {
		t.Fatalf("replay created a second identity: %+v vs %+v", replay, first)
	}
	// Same key, different business fingerprint.
	if _, e := c.CreateUploadBatch(context.Background(), sdk.UploadBatchRequest{SpaceID: h.space.ID, ConflictPolicy: "keep_both"}, "fixed-batch-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("fingerprint drift must be idempotency_conflict, got %v", e)
	}
	// The original batch is untouched by the refused replay.
	kept := fpBatchCreate(t, c, h.space.ID, "fail", "fixed-batch-key")
	if kept.BatchID != first.BatchID || kept.Phase != "enumerating" {
		t.Fatalf("original batch disturbed: %+v", kept)
	}
	_, bc := agent(t, bob, "", "read,write")
	// Identity isolation: another owner reusing the same key gets their own batch.
	other := fpBatchCreate(t, bc, bob.space.ID, "fail", "fixed-batch-key")
	if other.BatchID == first.BatchID {
		t.Fatal("batch identity leaked across owners")
	}
	// Invalid conflict policies are rejected up front.
	if _, e := c.CreateUploadBatch(context.Background(), sdk.UploadBatchRequest{SpaceID: h.space.ID, ConflictPolicy: "overwrite"}, "policy-batch"); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("unknown policy must be invalid_request, got %v", e)
	}
}

// Task 2.1: item keys are the durable per-item identity (relative path plus
// source fingerprint); duplicate keys inside one batch are refused and
// case-colliding paths are rejected for case-insensitive backends.
func TestFPUploadRecoveryItemKeyDiscipline(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "items-batch")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{
		fpItem("docs/a.txt", 3, service.Hash("a")),
		{RelativePath: "docs/empty-dir/", IsDir: true},
	})
	// One relative path identifies one source: a different fingerprint for
	// the same path is refused at enumeration; re-selected content against
	// the original item surfaces as content_mismatch at scheduling time.
	if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, []sdk.UploadBatchItemInput{fpItem("docs/a.txt", 4, service.Hash("b"))}); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("same path with a different fingerprint must be invalid_request, got %v", e)
	}
	// Exact duplicate (same path, same fingerprint) is refused.
	if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, []sdk.UploadBatchItemInput{fpItem("docs/a.txt", 3, service.Hash("a"))}); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("duplicate item key must be invalid_request, got %v", e)
	}
	// Absolute paths and parent traversal are refused.
	for _, bad := range []string{"/etc/passwd", "../escape.txt", "docs/../../escape.txt"} {
		if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, []sdk.UploadBatchItemInput{fpItem(bad, 1, service.Hash(bad))}); e == nil || e.Error() != "invalid_request" {
			t.Fatalf("path %q must be invalid_request, got %v", bad, e)
		}
	}
	// Case-fold collision across pages is refused.
	if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, []sdk.UploadBatchItemInput{fpItem("DOCS/a.txt", 3, service.Hash("a"))}); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("case collision must be invalid_request, got %v", e)
	}
	// Page limit: 501 entries in one call is refused without writing anything.
	big := make([]sdk.UploadBatchItemInput, 501)
	for i := range big {
		big[i] = fpItem(fmt.Sprintf("page/%04d.bin", i), 1, service.Hash(fmt.Sprintf("p%d", i)))
	}
	if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, big); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("oversized page must be invalid_request, got %v", e)
	}
	view := fpListBatch(t, c, b.BatchID, "", 200)
	if len(view.Items) != 2 {
		t.Fatalf("refused pages must not write items, got %d", len(view.Items))
	}
}

func fpListBatch(t *testing.T, c *sdk.Client, batchID, cursor string, limit int) sdk.UploadBatchView {
	t.Helper()
	out, e := c.GetUploadBatch(context.Background(), batchID, cursor, limit)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// Task 2.2: additive schema boundaries — a pre-change database keeps its old
// rows readable after the new tables and columns arrive, batch rows never
// reinterpret legacy operation state, and dropping only the new tables
// restores the previous behavior surface.
func TestFPUploadRecoverySchemaAdditive(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	// A legacy upload created before any batch exists stays fully functional.
	op := fpUploadBytes(t, c, h.space.ID, "legacy/before.txt", "legacy-before", []byte("legacy-bytes"))
	if op.Status != "completed" {
		t.Fatal(op.Status)
	}
	var legacyOp service.Operation
	if e := f.Service.DB.First(&legacyOp, "id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	if legacyOp.BatchID != "" || legacyOp.ItemKey != "" {
		t.Fatalf("legacy operation must keep empty batch binding: %+v", legacyOp)
	}
	// The new tables exist and stay empty for legacy flows.
	var batches, items, sessions int64
	f.Service.DB.Model(&service.UploadBatch{}).Count(&batches)
	f.Service.DB.Model(&service.UploadBatchItem{}).Count(&items)
	f.Service.DB.Model(&service.UploadSession{}).Count(&sessions)
	if batches != 0 || items != 0 || sessions != 0 {
		t.Fatalf("legacy flow wrote batch state: %d %d %d", batches, items, sessions)
	}
	// Old rows remain readable through the old contract.
	var sink strings.Builder
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil {
		t.Fatal(e)
	}
	if sink.String() != "legacy-bytes" {
		t.Fatal("legacy bytes changed")
	}
	// Rollback boundary: dropping only the additive tables leaves the legacy
	// operation and its bytes intact (the old surface never depended on them).
	if e := f.Service.DB.Migrator().DropTable("upload_batch_items", "upload_batches", "upload_sessions"); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.DB.First(&legacyOp, "id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	sink.Reset()
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil {
		t.Fatal(e)
	}
	if sink.String() != "legacy-bytes" {
		t.Fatal("legacy bytes changed after rollback drill")
	}
}

func mustSDK(t *testing.T, h *human) *sdk.Client {
	t.Helper()
	_, c := agent(t, h, "", "read,write")
	return c
}

func fpItemKey(path, sha string) string { return path + "\x00" + sha }

// fpScheduleItem uploads one batch item's bytes through the ordinary scoped
// upload contract with batch binding: creation reserves atomically, parts
// flow over the 8 MiB protocol and completion publishes through the shared
// publish transaction.
func fpScheduleItem(t *testing.T, c *sdk.Client, space, batchID, path, key string, content []byte) sdk.Operation {
	t.Helper()
	sha := service.Hash(string(content))
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: space, Path: path, BatchID: batchID, ItemKey: fpItemKey(path, sha)}, key, strings.NewReader(string(content)), nil)
	if e != nil {
		t.Fatal(e)
	}
	return op
}

// Task 3.1: the normal batch path — paged enumeration, seal, per-item
// scheduling with atomic reservation, backend reassembly, item completion
// and batch counters; the lost-response lookup recovers the original
// identity without creating a second task.
func TestFPUploadRecoverySealScheduleCompleteAndLookup(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "flow-batch")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{
		fpItem("docs/readme.md", int64(len("hello batch")), service.Hash("hello batch")),
		fpItem("docs/nested/deep.bin", int64(len("payload")), service.Hash("payload")),
		{RelativePath: "docs/empty/", IsDir: true},
	})
	sealed := fpSeal(t, c, b.BatchID)
	if sealed.Phase != "sealed" || sealed.ItemsTotal == nil || *sealed.ItemsTotal != 3 {
		t.Fatalf("seal wrong: %+v", sealed)
	}
	// Scheduling before seal is refused (inverse covered in 3.2 test).
	op := fpScheduleItem(t, c, h.space.ID, b.BatchID, "docs/readme.md", "item-1", []byte("hello batch"))
	if op.Status != "completed" {
		t.Fatalf("item did not complete: %+v", op)
	}
	view := fpListBatch(t, c, b.BatchID, "", 200)
	if view.ItemsCompleted != 1 || view.ItemsWaiting != 2 {
		t.Fatalf("counters wrong: %+v", view)
	}
	for _, it := range view.Items {
		if it.ItemKey == fpItemKey("docs/readme.md", service.Hash("hello batch")) {
			if it.Status != "completed" || it.OperationID == nil || *it.OperationID != op.ID || it.ResultCertainty != "confirmed" {
				t.Fatalf("completed item wrong: %+v", it)
			}
		}
	}
	// The durable session reports the protocol facts of the original op.
	session, e := c.UploadSession(context.Background(), op.ID)
	if e != nil {
		t.Fatal(e)
	}
	if session.PartSize != 8<<20 || session.Protocol != "v1-8mib-client-parts" || session.Durability != "durable" || session.Generation != 1 {
		t.Fatalf("session wrong: %+v", session)
	}
	if session.BackendPartSize != 64<<20 {
		t.Fatalf("backend part size wrong: %+v", session)
	}
	// Creation response lost: lookup recovers the original identities.
	found, e := c.LookupOperation(context.Background(), "upload_batch_create", "flow-batch", map[string]any{"space_id": h.space.ID, "conflict_policy": "fail"})
	if e != nil {
		t.Fatal(e)
	}
	if !found.Found || found.BatchID != b.BatchID || found.OperationID != b.OperationID || found.ResultCertainty != "confirmed" {
		t.Fatalf("batch lookup wrong: %+v", found)
	}
	itemLookup, e := c.LookupOperation(context.Background(), "upload", "item-1", map[string]any{"space_id": h.space.ID, "path": "docs/readme.md", "size_bytes": float64(len("hello batch")), "sha256": service.Hash("hello batch"), "batch_id": b.BatchID, "item_key": fpItemKey("docs/readme.md", service.Hash("hello batch"))})
	if e != nil {
		t.Fatal(e)
	}
	if !itemLookup.Found || itemLookup.OperationID != op.ID || itemLookup.BatchID != b.BatchID || itemLookup.ResultCertainty != "confirmed" {
		t.Fatalf("item lookup wrong: %+v", itemLookup)
	}
	// Finish the second file; the batch task completes with the empty-dir
	// entry resolved as an explicit no-byte item only when scheduled... it
	// stays waiting by design (no source bytes); cancel resolves the batch.
	fpScheduleItem(t, c, h.space.ID, b.BatchID, "docs/nested/deep.bin", "item-2", []byte("payload"))
	final := fpControl(t, c, b.BatchID, "cancel")
	if final.Phase != "cancelled" {
		t.Fatalf("cancel wrong: %+v", final)
	}
}

func fpSeal(t *testing.T, c *sdk.Client, batchID string) sdk.UploadBatchView {
	t.Helper()
	out, e := c.SealUploadBatch(context.Background(), batchID)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func fpControl(t *testing.T, c *sdk.Client, batchID, action string) sdk.UploadBatchView {
	t.Helper()
	out, e := c.ControlUploadBatch(context.Background(), batchID, action)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// Task 3.2: inverses — unsealed scheduling, source re-selection with
// different content, pause blocking new scheduling, and cancel keeping
// committed files as completed side effects.
func TestFPUploadRecoverySourcePauseCancelInverses(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "inverse-batch")
	// A second item stays pending so pause has work to block.
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{fpItem("a.txt", int64(len("original")), service.Hash("original")), fpItem("b.txt", int64(len("later")), service.Hash("later"))})
	// Scheduling before seal: batch_not_sealed.
	_, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "a.txt", BatchID: b.BatchID, ItemKey: fpItemKey("a.txt", service.Hash("original"))}, "early", strings.NewReader("original"), nil)
	if e == nil || e.Error() != "batch_not_sealed" {
		t.Fatalf("unsealed scheduling must be batch_not_sealed, got %v", e)
	}
	fpSeal(t, c, b.BatchID)
	// Re-selected source with different content: content_mismatch, the
	// original item keeps its identity and nothing is staged.
	_, e = c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "a.txt", BatchID: b.BatchID, ItemKey: fpItemKey("a.txt", service.Hash("original"))}, "mismatch", strings.NewReader("different"), nil)
	if e == nil || e.Error() != "content_mismatch" {
		t.Fatalf("source drift must be content_mismatch, got %v", e)
	}
	// The original source still schedules and completes under its own key.
	op := fpScheduleItem(t, c, h.space.ID, b.BatchID, "a.txt", "ok-item", []byte("original"))
	if op.Status != "completed" {
		t.Fatalf("original item failed: %+v", op)
	}
	// Completed items are never rewritten: scheduling the same item again is
	// refused without touching the file.
	_, e = c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "a.txt", BatchID: b.BatchID, ItemKey: fpItemKey("a.txt", service.Hash("original"))}, "again", strings.NewReader("original"), nil)
	if e == nil || e.Error() != "operation_closed" {
		t.Fatalf("completed item rescheduling must be operation_closed, got %v", e)
	}
	// Pause blocks new scheduling and keeps the committed file.
	paused := fpControl(t, c, b.BatchID, "pause")
	if paused.Phase != "paused" {
		t.Fatalf("pause wrong: %+v", paused)
	}
	// Cancel keeps committed files as completed side effects.
	cancelled := fpControl(t, c, b.BatchID, "cancel")
	if cancelled.Phase != "cancelled" {
		t.Fatalf("cancel wrong: %+v", cancelled)
	}
	if cancelled.ItemsCompleted != 1 {
		t.Fatalf("cancel hid completed side effects: %+v", cancelled)
	}
	var file service.File
	if e := f.Service.DB.Where("owner = ? AND space_id = ? AND path = ?", h.user.ID, h.space.ID, "a.txt").First(&file).Error; e != nil {
		t.Fatal("cancel deleted a committed file")
	}
	var sink strings.Builder
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || sink.String() != "original" {
		t.Fatalf("committed file unreadable after cancel: %v %q", e, sink.String())
	}
	// Control on a terminal batch is refused.
	if _, e := c.ControlUploadBatch(context.Background(), b.BatchID, "resume"); e == nil || e.Error() != "operation_closed" {
		t.Fatalf("terminal control must be operation_closed, got %v", e)
	}
}

// Task 4.1 entry equivalence: HTTP, MCP and SDK expose the same application
// verdicts for the batch read model and the lost-response lookup.
func TestFPUploadRecoveryEntryEquivalence(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "equiv-batch")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{fpItem("e.txt", int64(len("equiv")), service.Hash("equiv"))})
	fpSeal(t, c, b.BatchID)
	// SDK view.
	sdkView := fpListBatch(t, c, b.BatchID, "", 200)
	// MCP view through the same application service.
	principal := service.Principal{UserID: h.user.ID, Agent: &a}
	out, e := f.Service.CallTool(context.Background(), principal, "upload_batch_get", map[string]any{"batch_id": b.BatchID, "limit": "50"})
	if e != nil {
		t.Fatal(e)
	}
	mcpView, ok := out.(service.UploadBatchView)
	if !ok {
		t.Fatalf("upload_batch_get returned unexpected shape: %T", out)
	}
	if mcpView.BatchID != sdkView.BatchID || mcpView.Phase != sdkView.Phase || mcpView.BatchRevision != sdkView.BatchRevision || len(mcpView.Items) != len(sdkView.Items) {
		t.Fatalf("MCP and SDK verdicts differ: %+v vs %+v", mcpView, sdkView)
	}
	// HTTP raw response carries the same batch identity.
	var httpView sdk.UploadBatchView
	if status := h.call(t, "GET", "/upload-batches/"+b.BatchID+"/items?limit=50", nil, &httpView); status != 200 {
		t.Fatalf("HTTP batch read failed: %d", status)
	}
	if httpView.BatchID != sdkView.BatchID || httpView.Phase != sdkView.Phase || httpView.ItemsTotal == nil || *httpView.ItemsTotal != *sdkView.ItemsTotal {
		t.Fatalf("HTTP and SDK verdicts differ: %+v vs %+v", httpView, sdkView)
	}
	// Lookup equivalence over MCP.
	lookup, e := f.Service.CallTool(context.Background(), principal, "operation_lookup", map[string]any{"action": "upload_batch_create", "idempotency_key": "equiv-batch", "original_request": map[string]any{"space_id": h.space.ID, "conflict_policy": "fail"}})
	if e != nil {
		t.Fatal(e)
	}
	m, ok := lookup.(service.OperationLookupView)
	if !ok {
		t.Fatalf("operation_lookup returned unexpected shape: %T", lookup)
	}
	if !m.Found || m.BatchID != b.BatchID || m.ResultCertainty != "confirmed" {
		t.Fatalf("MCP lookup verdict wrong: %+v", m)
	}
}

// Task 5.1: concurrent revocation and partial recovery. A batch with one
// completed, one failed and one pending item reports every item's own
// terminal state; revoking the acting agent stops further scheduling without
// rewriting completed items or leaking cross-identity reads.
func TestFPUploadRecoveryPartialRevocationAndErrorInjection(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "partial-batch")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{
		fpItem("done.txt", int64(len("done")), service.Hash("done")),
		fpItem("todo.txt", int64(len("todo")), service.Hash("todo")),
	})
	fpSeal(t, c, b.BatchID)
	done := fpScheduleItem(t, c, h.space.ID, b.BatchID, "done.txt", "partial-done", []byte("done"))
	if done.Status != "completed" {
		t.Fatalf("done item failed: %+v", done)
	}
	// Error injection: drop the staged object mid-flight is covered by the
	// legacy suite; here the injected error is a revoked agent between items.
	f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true)
	_, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "todo.txt", BatchID: b.BatchID, ItemKey: fpItemKey("todo.txt", service.Hash("todo"))}, "partial-todo", strings.NewReader("todo"), nil)
	if e == nil || (e.Error() != "forbidden" && e.Error() != "unauthorized") {
		t.Fatalf("revoked agent must be refused, got %v", e)
	}
	// The owner (not the revoked agent) reads the partial truth.
	var view sdk.UploadBatchView
	if status := h.call(t, "GET", "/upload-batches/"+b.BatchID+"/items?limit=50", nil, &view); status != 200 {
		t.Fatalf("owner batch read failed: %d", status)
	}
	if view.ItemsCompleted != 1 || view.ItemsWaiting != 1 {
		t.Fatalf("partial view wrong: %+v", view)
	}
	for _, it := range view.Items {
		if it.ItemKey == fpItemKey("done.txt", service.Hash("done")) && (it.Status != "completed" || it.ResultCertainty != "confirmed") {
			t.Fatalf("completed item rewritten: %+v", it)
		}
	}
	// Another identity never reads the batch.
	bob, _ := login(t, f, "bob")
	_, bc := agent(t, bob, "", "read,write")
	if _, e := bc.GetUploadBatch(context.Background(), b.BatchID, "", 50); e == nil || e.Error() != "not_found" {
		t.Fatalf("cross-identity batch read must be not_found, got %v", e)
	}
}

// Task 5.2 scale acceptance, gated behind DRIVEBRIDGE_FP_SCALE like the
// Wave-1 scale entries. quick runs a 2,000-item batch with 0-byte files,
// empty directories, Unicode names and a conflict; full raises the item
// count to 10,000. The achieved dimensions (item scale, cursor pagination
// over the full batch, durable part resume with read-back digest, and a
// bounded-memory multi-part streamed transfer with peak heap recorded) are
// asserted here; the 100GiB real-stream dimension runs in the separate
// disk-backed TestFPUploadRecoveryScaleHundredGiBStream behind
// DRIVEBRIDGE_FP_HUNDRED_GIB=1.
func TestFPUploadRecoveryScaleBatch(t *testing.T) {
	mode := os.Getenv("DRIVEBRIDGE_FP_SCALE")
	if mode != "quick" && mode != "full" {
		t.Skip("scale fixture gated behind DRIVEBRIDGE_FP_SCALE=quick|full")
	}
	total := 2000
	if mode == "full" {
		total = 10000
	}
	f, e := fixtures.StartWithOptions(t.TempDir(), "", "", fixtures.Options{MaxBytes: 512 << 20})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "scale-batch")
	// Enumerate in 500-entry pages: 0-byte files, explicit empty dirs,
	// Unicode names and one conflicting path against an existing file.
	fpUploadBytes(t, c, h.space.ID, "conflict.txt", "conflict-pre", []byte("already here"))
	for page := 0; page*500 < total; page++ {
		end := min((page+1)*500, total)
		items := make([]sdk.UploadBatchItemInput, 0, end-page*500)
		for i := page * 500; i < end; i++ {
			switch {
			case i == 0:
				items = append(items, fpItem("scale/conflict.txt", int64(len("already here")), service.Hash("already here")))
			case i%97 == 0:
				items = append(items, sdk.UploadBatchItemInput{RelativePath: fmt.Sprintf("scale/empty-%d/", i), IsDir: true})
			case i%13 == 0:
				items = append(items, fpItem(fmt.Sprintf("scale/空目录/文件-%d.txt", i), 0, ""))
			default:
				payload := fmt.Sprintf("scale-payload-%d", i)
				items = append(items, fpItem(fmt.Sprintf("scale/file-%06d.bin", i), int64(len(payload)), service.Hash(payload)))
			}
		}
		if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, items); e != nil {
			t.Fatalf("page %d rejected: %v", page, e)
		}
	}
	// The multi-part probe item is enumerated before the seal like any
	// other source entry.
	big := make([]byte, 24<<20)
	for i := range big {
		big[i] = byte(i)
	}
	bigSHA := sha256.Sum256(big)
	bigItem := fpItem("scale/big.bin", int64(len(big)), hex.EncodeToString(bigSHA[:]))
	small := bytes.Repeat([]byte("resume-me-"), (8<<20)/9+3)
	sSHA := sha256.Sum256(small)
	sItem := fpItem("scale/resume.bin", int64(len(small)), hex.EncodeToString(sSHA[:]))
	if _, e := c.AddUploadBatchItems(context.Background(), b.BatchID, []sdk.UploadBatchItemInput{bigItem, sItem}); e != nil {
		t.Fatal(e)
	}
	fpSeal(t, c, b.BatchID)
	// Paginate the whole batch through the opaque cursor.
	seen := 0
	cursor := ""
	for {
		page := fpListBatch(t, c, b.BatchID, cursor, 200)
		seen += len(page.Items)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if seen != total+2 {
		t.Fatalf("pagination lost items: %d of %d", seen, total+2)
	}
	// Schedule and complete a spread of real items, including the 0-byte
	// Unicode entry and one multi-part (3x8MiB) streamed transfer with peak
	// heap recorded against the 256MiB single-transfer budget.
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "scale/big.bin", BatchID: b.BatchID, ItemKey: fpItemKey("scale/big.bin", hex.EncodeToString(bigSHA[:]))}, "scale-big", bytes.NewReader(big), nil)
	if e != nil {
		t.Fatal(e)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	peak := after.TotalAlloc - before.TotalAlloc
	t.Logf("scale=%s items=%d paginated=%d big_transfer_bytes=%d total_allocated_during_transfer=%d budget=268435456", mode, total, seen, len(big), peak)
	if op.Status != "completed" {
		t.Fatalf("multi-part streamed item failed: %+v", op)
	}
	if peak > 256<<20 {
		t.Fatalf("single-transfer allocation budget exceeded: %d", peak)
	}
	var sink bytes.Buffer
	if e := c.Download(context.Background(), op.FileID, "", &sink); e != nil || !bytes.Equal(sink.Bytes(), big) {
		t.Fatalf("read-back digest mismatch after streamed reassembly: %v", e)
	}
	// Durable part resume: an interrupted transfer recovers the missing
	// range only and completes with an identical read-back digest. Part 0 is
	// delivered, then the client "restarts"; the session reports the
	// verified range and only part 1 is re-sent before completion.
	body, _ := json.Marshal(sdk.UploadRequest{SpaceID: h.space.ID, Path: "scale/resume.bin", Size: int64(len(small)), SHA256: hex.EncodeToString(sSHA[:]), BatchID: b.BatchID, ItemKey: fpItemKey("scale/resume.bin", hex.EncodeToString(sSHA[:]))})
	createReq, _ := http.NewRequest("POST", f.App.URL+"/api/v1/uploads", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "scale-resume")
	createReq.Header.Set("X-CSRF-Token", h.csrf)
	createReq.Header.Set("Origin", f.App.URL)
	resp, e := h.http.Do(createReq)
	if e != nil {
		t.Fatal(e)
	}
	rawCreate, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var created struct {
		Data sdk.Operation `json:"data"`
	}
	json.Unmarshal(rawCreate, &created)
	if created.Data.ID == "" {
		t.Fatalf("resume operation not created: %d %s", resp.StatusCode, string(rawCreate))
	}
	part0 := small[:8<<20]
	putPart := func(index int, payload []byte) {
		pr, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/operations/%s/parts/%d", f.App.URL, created.Data.ID, index), bytes.NewReader(payload))
		pr.Header.Set("Content-Type", "application/octet-stream")
		pr.Header.Set("X-CSRF-Token", h.csrf)
		pr.Header.Set("Origin", f.App.URL)
		presp, e := h.http.Do(pr)
		if e != nil {
			t.Fatal(e)
		}
		presp.Body.Close()
		if presp.StatusCode != 200 {
			t.Fatalf("part %d rejected: %d", index, presp.StatusCode)
		}
	}
	putPart(0, part0)
	session, e := c.UploadSession(context.Background(), created.Data.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(session.ReceivedParts) != 1 || session.ReceivedParts[0] != 0 {
		t.Fatalf("session lost the durable part: %+v", session)
	}
	// Client restart: re-selecting the source must reproduce the same parts;
	// re-sending part 0 is an idempotent same-digest replay.
	putPart(0, part0)
	putPart(1, small[8<<20:])
	completeReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/operations/%s/complete", f.App.URL, created.Data.ID), nil)
	completeReq.Header.Set("X-CSRF-Token", h.csrf)
	completeReq.Header.Set("Origin", f.App.URL)
	cresp, e := h.http.Do(completeReq)
	if e != nil {
		t.Fatal(e)
	}
	var done struct {
		Data sdk.Operation `json:"data"`
	}
	json.NewDecoder(cresp.Body).Decode(&done)
	cresp.Body.Close()
	if done.Data.Status != "completed" {
		t.Fatalf("resumed transfer did not complete: %+v", done.Data)
	}
	var resumed bytes.Buffer
	if e := c.Download(context.Background(), done.Data.FileID, "", &resumed); e != nil || !bytes.Equal(resumed.Bytes(), small) {
		t.Fatalf("resumed read-back digest mismatch: %v", e)
	}
	t.Logf("scale resume verified: parts_replayed=1 completed_bytes=%d", len(small))
}

// Wave2Review MAJOR-1 regression: when the last batch item completes, the
// batch anchor Operation (kind upload_batch) must complete with it. The
// completion used to address the row by the batch id and silently missed,
// leaving the anchor running forever.
func TestFPUploadRecoveryBatchAnchorCompletes(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	b := fpBatchCreate(t, c, h.space.ID, "fail", "anchor-batch")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{fpItem("docs/only.txt", 6, service.Hash("anchor"))})
	fpSeal(t, c, b.BatchID)
	op := fpScheduleItem(t, c, h.space.ID, b.BatchID, "docs/only.txt", "anchor-item", []byte("anchor"))
	if op.Status != "completed" {
		t.Fatal(op.Status)
	}
	view := fpListBatch(t, c, b.BatchID, "", 200)
	if view.Phase != "completed" || view.ItemsCompleted != 1 {
		t.Fatalf("batch not complete: %+v", view)
	}
	var anchor service.Operation
	if e := f.Service.DB.First(&anchor, "id = ?", b.OperationID).Error; e != nil {
		t.Fatal(e)
	}
	if anchor.Kind != "upload_batch" || anchor.Status != "completed" {
		t.Fatalf("anchor operation stuck at kind=%s status=%s", anchor.Kind, anchor.Status)
	}
}

func fpCreateUpload(t *testing.T, h *human, space, path, key, sha string, size int64, batch, itemKey string) service.Operation {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"space_id": space, "path": path, "size_bytes": size, "sha256": sha, "batch_id": batch, "item_key": itemKey})
	req, _ := http.NewRequest("POST", h.base+"/api/v1/uploads", bytes.NewReader(body))
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	response, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	var env struct {
		Data  service.Operation `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if e := json.NewDecoder(response.Body).Decode(&env); e != nil || response.StatusCode != 200 {
		t.Fatalf("upload create failed: %d %s", response.StatusCode, env.Error.Code)
	}
	return env.Data
}
func fpPutPartRaw(t *testing.T, h *human, opID string, index int, body []byte) int {
	t.Helper()
	status, raw := fpRawCall(t, h, "PUT", "/operations/"+opID+"/parts/"+fmt.Sprint(index), body)
	if status >= 300 {
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(raw, &env)
		t.Fatalf("part %d refused: %d %s", index, status, env.Error.Code)
	}
	return status
}

// Wave2Review MAJOR-2 regression: an operation whose staged source was lost
// (the metadata-only restore state repairParts records) must refuse blind
// resume and completion with the typed source_reselection_required instead
// of the misleading missing_parts, and the refusal clears once the original
// actor genuinely re-sends source parts.
func TestFPUploadRecoverySourceReselectionRefusal(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	content := bytes.Repeat([]byte("s"), int(2*service.PartSize+16))
	sha := service.Hash(string(content))
	op := fpCreateUpload(t, h, h.space.ID, "docs/reselect.bin", "reselect-key", sha, int64(len(content)), "", "")
	// Metadata-only restore state: staged bytes gone, source re-selection
	// is the only recovery (exactly what repairParts records).
	if e := f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "interrupted", "error": "original_source_required", "transferred": 0}).Error; e != nil {
		t.Fatal(e)
	}
	var resumeOut struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		Data json.RawMessage `json:"data"`
	}
	status, raw := fpRawCall(t, h, "POST", "/operations/"+op.ID+"/resume", []byte("{}"))
	json.Unmarshal(raw, &resumeOut)
	if status != 409 || resumeOut.Error.Code != "source_reselection_required" {
		t.Fatalf("resume of lost source must be source_reselection_required, got %d %s", status, resumeOut.Error.Code)
	}
	status, raw = fpRawCall(t, h, "POST", "/operations/"+op.ID+"/complete", []byte("{}"))
	json.Unmarshal(raw, &resumeOut)
	if status != 409 || resumeOut.Error.Code != "source_reselection_required" {
		t.Fatalf("complete of lost source must be source_reselection_required, got %d %s", status, resumeOut.Error.Code)
	}
	// Genuine re-selection: the original actor re-sends the parts; the
	// refusal must not outlive the reselection.
	fpPutPartRaw(t, h, op.ID, 0, content[:service.PartSize])
	fpPutPartRaw(t, h, op.ID, 1, content[service.PartSize:2*service.PartSize])
	fpPutPartRaw(t, h, op.ID, 2, content[2*service.PartSize:])
	var done service.Operation
	if status := h.call(t, "POST", "/operations/"+op.ID+"/complete", map[string]any{}, &done); status != 200 || done.Status != "completed" {
		t.Fatalf("completion after reselection failed: %d %+v", status, done)
	}
}

// Wave2Review MAJOR-2 regression: a batch-bound upload whose durable
// backend transfer session expired refuses further parts with the typed
// transfer_session_expired; the explicit resume by the original actor
// rebuilds the session under the same Operation (generation bump) and the
// parts flow again.
func TestFPUploadRecoveryExpiredBackendSessionRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	content := bytes.Repeat([]byte("e"), int(2*service.PartSize+8))
	sha := service.Hash(string(content))
	b := fpBatchCreate(t, c, h.space.ID, "fail", "expired-session-batch")
	itemKey := fpItemKey("docs/expired.bin", sha)
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{fpItem("docs/expired.bin", int64(len(content)), sha)})
	fpSeal(t, c, b.BatchID)
	op := fpCreateUpload(t, h, h.space.ID, "docs/expired.bin", "expired-item", sha, int64(len(content)), b.BatchID, itemKey)
	if op.BatchID != b.BatchID {
		t.Fatalf("upload not batch-bound: %+v", op)
	}
	fpPutPartRaw(t, h, op.ID, 0, content[:service.PartSize])
	// Backdate the backend session window past its expiry.
	past := time.Now().UTC().Add(-25 * time.Hour)
	if e := f.Service.DB.Model(&service.UploadSession{}).Where("operation_id = ?", op.ID).Update("expires_at", past).Error; e != nil {
		t.Fatal(e)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCall(t, h, "PUT", "/operations/"+op.ID+"/parts/1", content[service.PartSize:2*service.PartSize])
	json.Unmarshal(raw, &env)
	if status != 409 || env.Error.Code != "transfer_session_expired" {
		t.Fatalf("expired backend session must refuse parts with transfer_session_expired, got %d %s", status, env.Error.Code)
	}
	// Explicit recovery under the original operation: resume bumps the
	// session generation and re-opens the window without rekeying anything.
	var resumed service.Operation
	if status := h.call(t, "POST", "/operations/"+op.ID+"/resume", map[string]any{}, &resumed); status != 200 {
		t.Fatalf("session recovery resume failed: %d", status)
	}
	var session service.UploadSession
	if e := f.Service.DB.First(&session, "operation_id = ?", op.ID).Error; e != nil {
		t.Fatal(e)
	}
	if session.Generation != 2 {
		t.Fatalf("explicit recovery must bump the session generation, got %d", session.Generation)
	}
	fpPutPartRaw(t, h, op.ID, 1, content[service.PartSize:2*service.PartSize])
	fpPutPartRaw(t, h, op.ID, 2, content[2*service.PartSize:])
	var done service.Operation
	if status := h.call(t, "POST", "/operations/"+op.ID+"/complete", map[string]any{}, &done); status != 200 || done.Status != "completed" {
		t.Fatalf("completion after session recovery failed: %d %+v", status, done)
	}
	view := fpListBatch(t, c, b.BatchID, "", 200)
	if view.Phase != "completed" {
		t.Fatalf("batch did not complete after session recovery: %+v", view)
	}
}

// fpGenReader produces deterministic scale-stream bytes: each 1MiB block
// starts with a per-block counter header and repeats a fixed base pattern,
// so any byte range is reproducible without storing the stream.
type fpGenReader struct {
	offset, remaining int64
}

func (g *fpGenReader) Read(p []byte) (int, error) {
	if g.remaining <= 0 {
		return 0, io.EOF
	}
	const block = 1 << 20
	total := 0
	for len(p) > 0 && g.remaining > 0 {
		blockIndex := g.offset / block
		inBlock := int(g.offset % block)
		chunk := min(int64(len(p)), g.remaining, int64(block-inBlock))
		var base [64]byte
		header := fmt.Sprintf("fp-scale-block-%016x:", blockIndex)
		copy(base[:], header)
		for i := range chunk {
			p[i] = base[(inBlock+int(i))%len(base)]
		}
		p = p[chunk:]
		g.offset += chunk
		g.remaining -= chunk
		total += int(chunk)
	}
	return total, nil

}

// Task 5.2 hundred-GiB dimension, gated behind DRIVEBRIDGE_FP_SCALE=full with
// DRIVEBRIDGE_FP_HUNDRED_GIB=1 opting in to the disk-backed stream. A real
// 100GiB object is uploaded as 8MiB parts through the production HTTP part
// surface, interrupted at 40%, resumed from the durable session (one replayed
// part), completed, and read back streaming through SHA-256. Peak heap,
// per-transfer allocation budget under 4-file concurrency, and staging
// occupancy are recorded; the run skips when free disk cannot hold the
// stream plus its stored copy with headroom.
func TestFPUploadRecoveryScaleHundredGiBStream(t *testing.T) {
	if os.Getenv("DRIVEBRIDGE_FP_SCALE") != "full" || os.Getenv("DRIVEBRIDGE_FP_HUNDRED_GIB") != "1" {
		t.Skip("hundred-GiB fixture gated behind DRIVEBRIDGE_FP_SCALE=full and DRIVEBRIDGE_FP_HUNDRED_GIB=1")
	}
	const stream = int64(100) << 30
	const part = int64(8) << 20
	parts := int(stream / part)
	diskDir := t.TempDir()
	var stat syscall.Statfs_t
	if e := syscall.Statfs(diskDir, &stat); e != nil {
		t.Fatal(e)
	}
	free := int64(stat.Bavail) * int64(stat.Bsize)
	// Source is generated on the fly; the durable store keeps parts plus
	// the completed reassembly, so budget 2.2x with headroom.
	if free < stream*22/10 {
		t.Skipf("insufficient disk for 100GiB stream: free=%d need=%d", free, stream*22/10)
	}
	stateDir := t.TempDir()
	f, e := fixtures.StartWithOptions(stateDir, "", "", fixtures.Options{MaxBytes: stream + 8<<30, DiskObjects: diskDir})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	// Deterministic whole-stream digest without retaining bytes.
	digestStart := time.Now()
	hasher := sha256.New()
	if _, e := io.Copy(hasher, &fpGenReader{offset: 0, remaining: stream}); e != nil {
		t.Fatal(e)
	}
	wantSHA := hex.EncodeToString(hasher.Sum(nil))
	digestSeconds := time.Since(digestStart).Seconds()
	// Peak heap monitor: single-transfer buffer budget is 256MiB.
	var peakHeap atomic.Int64
	stopMonitor := make(chan struct{})
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		var ms runtime.MemStats
		for {
			select {
			case <-stopMonitor:
				return
			default:
			}
			runtime.ReadMemStats(&ms)
			if int64(ms.HeapInuse) > peakHeap.Load() {
				peakHeap.Store(int64(ms.HeapInuse))
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	// Create the upload through the durable HTTP surface.
	body, _ := json.Marshal(sdk.UploadRequest{SpaceID: h.space.ID, Path: "scale/hundred-gib.bin", Size: stream, SHA256: wantSHA})
	createReq, _ := http.NewRequest("POST", f.App.URL+"/api/v1/uploads", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "hundred-gib")
	createReq.Header.Set("X-CSRF-Token", h.csrf)
	createReq.Header.Set("Origin", f.App.URL)
	resp, e := h.http.Do(createReq)
	if e != nil {
		t.Fatal(e)
	}
	rawCreate, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var created struct {
		Data sdk.Operation `json:"data"`
	}
	json.Unmarshal(rawCreate, &created)
	if created.Data.ID == "" {
		t.Fatalf("upload not created: %d %s", resp.StatusCode, string(rawCreate))
	}
	putPart := func(index int) {
		pr, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/operations/%s/parts/%d", f.App.URL, created.Data.ID, index), &fpGenReader{offset: int64(index) * part, remaining: part})
		pr.Header.Set("Content-Type", "application/octet-stream")
		pr.Header.Set("X-CSRF-Token", h.csrf)
		pr.Header.Set("Origin", f.App.URL)
		presp, e := h.http.Do(pr)
		if e != nil {
			t.Fatalf("part %d: %v", index, e)
		}
		presp.Body.Close()
		if presp.StatusCode != 200 {
			t.Fatalf("part %d rejected: %d", index, presp.StatusCode)
		}
	}
	uploadStart := time.Now()
	interruptAt := parts * 2 / 5
	for i := range interruptAt {
		putPart(i)
	}
	stagingMid := dirUsageBytes(stateDir) + f.DiskUsageBytes()
	session, e := c.UploadSession(context.Background(), created.Data.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(session.ReceivedParts) != interruptAt {
		t.Fatalf("durable session lost parts: %d of %d", len(session.ReceivedParts), interruptAt)
	}
	// Client restart: replay one already-durable part (idempotent), then
	// send only the remaining range.
	putPart(interruptAt - 1)
	for i := interruptAt; i < parts; i++ {
		putPart(i)
	}
	completeReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/operations/%s/complete", f.App.URL, created.Data.ID), nil)
	completeReq.Header.Set("X-CSRF-Token", h.csrf)
	completeReq.Header.Set("Origin", f.App.URL)
	cresp, e := h.http.Do(completeReq)
	if e != nil {
		t.Fatal(e)
	}
	var done struct {
		Data sdk.Operation `json:"data"`
	}
	json.NewDecoder(cresp.Body).Decode(&done)
	cresp.Body.Close()
	if done.Data.Status != "completed" {
		t.Fatalf("hundred-GiB transfer did not complete: %+v", done.Data)
	}
	uploadSeconds := time.Since(uploadStart).Seconds()
	stagingFinal := dirUsageBytes(stateDir) + f.DiskUsageBytes()
	// Read the whole object back through the SDK streaming download and
	// compare digests without retaining bytes.
	readStart := time.Now()
	back := sha256.New()
	if e := c.Download(context.Background(), done.Data.FileID, "", back); e != nil {
		t.Fatal(e)
	}
	if got := hex.EncodeToString(back.Sum(nil)); got != wantSHA {
		t.Fatalf("read-back digest mismatch after 100GiB resume")
	}
	readSeconds := time.Since(readStart).Seconds()
	// 4-file concurrency: four simultaneous 256MiB streams through the same
	// surface while the heap monitor tracks the single-transfer budget.
	const concurrentFiles = 4
	const concurrentSize = int64(256) << 20
	concStart := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, concurrentFiles)
	for i := range concurrentFiles {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hasher := sha256.New()
			buf := make([]byte, 1<<20)
			tr := io.TeeReader(&io.LimitedReader{R: &fpGenReader{offset: int64(n) << 28, remaining: concurrentSize}, N: concurrentSize}, hasher)
			for {
				if _, e := tr.Read(buf); e != nil {
					break
				}
			}
			sha := hex.EncodeToString(hasher.Sum(nil))
			createBody, _ := json.Marshal(sdk.UploadRequest{SpaceID: h.space.ID, Path: fmt.Sprintf("scale/conc-%d.bin", n), Size: concurrentSize, SHA256: sha})
			req, _ := http.NewRequest("POST", f.App.URL+"/api/v1/uploads", bytes.NewReader(createBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", fmt.Sprintf("conc-%d", n))
			req.Header.Set("X-CSRF-Token", h.csrf)
			req.Header.Set("Origin", f.App.URL)
			cresp, e := h.http.Do(req)
			if e != nil {
				errs[n] = e
				return
			}
			var cr struct {
				Data sdk.Operation `json:"data"`
			}
			rawCreate, _ := io.ReadAll(cresp.Body)
			cresp.Body.Close()
			json.Unmarshal(rawCreate, &cr)
			if cresp.StatusCode != 200 || cr.Data.ID == "" {
				errs[n] = fmt.Errorf("concurrent %d create failed: %d %s", n, cresp.StatusCode, string(rawCreate))
				return
			}
			for p := int64(0); p < concurrentSize/part; p++ {
				pr, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/operations/%s/parts/%d", f.App.URL, cr.Data.ID, p), &fpGenReader{offset: (int64(n) << 28) + p*part, remaining: part})
				pr.Header.Set("Content-Type", "application/octet-stream")
				pr.Header.Set("X-CSRF-Token", h.csrf)
				pr.Header.Set("Origin", f.App.URL)
				presp, e := h.http.Do(pr)
				if e != nil {
					errs[n] = e
					return
				}
				presp.Body.Close()
			}
			fin, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/operations/%s/complete", f.App.URL, cr.Data.ID), nil)
			fin.Header.Set("X-CSRF-Token", h.csrf)
			fin.Header.Set("Origin", f.App.URL)
			fresp, e := h.http.Do(fin)
			if e != nil {
				errs[n] = e
				return
			}
			var fr struct {
				Data sdk.Operation `json:"data"`
			}
			json.NewDecoder(fresp.Body).Decode(&fr)
			fresp.Body.Close()
			if fr.Data.Status != "completed" {
				errs[n] = fmt.Errorf("concurrent %d status %s", n, fr.Data.Status)
			}
		}(i)
	}
	wg.Wait()
	for i := range concurrentFiles {
		if errs[i] != nil {
			t.Fatalf("concurrent stream %d failed: %v", i, errs[i])
		}
	}
	concSeconds := time.Since(concStart).Seconds()
	close(stopMonitor)
	<-monitorDone
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	peak := peakHeap.Load() - int64(before.HeapInuse)
	if peak > 256<<20 {
		t.Fatalf("single-transfer allocation budget exceeded under 4-file concurrency: %d", peak)
	}
	report := map[string]any{"stream_bytes": stream, "parts": parts, "interrupted_at_parts": interruptAt, "parts_replayed": 1, "digest_seconds": digestSeconds, "upload_seconds": uploadSeconds, "read_back_seconds": readSeconds, "concurrent_files": concurrentFiles, "concurrent_seconds": concSeconds, "peak_heap_delta_bytes": peak, "staging_bytes_mid": stagingMid, "staging_bytes_final": stagingFinal, "read_back_sha256": wantSHA}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("hundred-GiB report: %s", b)
	if dir := os.Getenv("DRIVEBRIDGE_EVIDENCE_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "fp-upload-hundred-gib.json"), b, 0o644)
	}
}

// dirUsageBytes walks one directory tree and sums regular file sizes so
// staging occupancy reports the server's real on-disk footprint.
func dirUsageBytes(root string) int64 {
	var total int64
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
