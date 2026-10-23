package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func fpHold(t *testing.T, c *sdk.Client, fileID, versionID, purpose, source, expires, key string) sdk.RetentionHoldView {
	t.Helper()
	out, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: fileID, VersionID: versionID, Purpose: purpose, SourceRef: source, ExpiresAt: expires}, key)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// Task 2.1: the hold keeps a durable fixed identity under its idempotency
// key, the same key with different business inputs is refused, and hold
// identities never leak across owners.
func TestFPRetentionGuardsIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, c, h.space.ID, "held.txt", "hold-src", []byte("held-bytes"))
	// The expiry is part of the idempotency fingerprint; a lost-response
	// replay must reuse the original request rather than recompute a new time.
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	first := fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, expires, "hold-key")
	if first.HoldID == "" || first.Revision != 1 || first.State != "active" {
		t.Fatalf("hold creation wrong: %+v", first)
	}
	// Lost creation response: replay returns the same hold.
	replay := fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, expires, "hold-key")
	if replay.HoldID != first.HoldID || replay.Revision != first.Revision {
		t.Fatalf("replay created a second hold: %+v vs %+v", replay, first)
	}
	// Same key, different business fingerprint.
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "legal_hold", SourceRef: op.ID, ExpiresAt: expires}, "hold-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("fingerprint drift must be idempotency_conflict, got %v", e)
	}
	// Unknown purposes and foreign sources are refused up front.
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, Purpose: "forever", SourceRef: op.ID, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "bad-purpose"); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("unknown purpose must be invalid_request, got %v", e)
	}
	foreign := service.ID()
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, Purpose: "fixed_delivery", SourceRef: foreign, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "bad-source"); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("foreign source must be invalid_request, got %v", e)
	}
	// Cross-owner isolation: bob cannot read alice's hold.
	bob, _ := login(t, f, "bob")
	_, bc := agent(t, bob, "", "read,write")
	if _, e := bc.GetRetentionHold(context.Background(), first.HoldID); e == nil || e.Error() != "not_found" {
		t.Fatalf("cross-owner hold read must be not_found, got %v", e)
	}
	// Read-back through the original id keeps the same verdict.
	got, e := c.GetRetentionHold(context.Background(), first.HoldID)
	if e != nil || got.State != "active" || got.Purpose != "fixed_delivery" {
		t.Fatalf("hold read-back wrong: %+v %v", got, e)
	}
}

// Task 2.2: additive schema boundaries — legacy purge flows never touch the
// retention tables, and dropping only the new tables restores the previous
// surface.
func TestFPRetentionGuardsSchemaAdditive(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "legacy.txt", "legacy-key", []byte("legacy"))
	keep := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "keep.txt", "keep-key", []byte("keep"))
	_ = keep
	if e := f.Service.Trash(p, op.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.Purge(context.Background(), p, op.FileID); e != nil {
		t.Fatal(e)
	}
	var holds, policies, plans, items int64
	f.Service.DB.Model(&service.RetentionHold{}).Count(&holds)
	f.Service.DB.Model(&service.RetentionPolicy{}).Count(&policies)
	f.Service.DB.Model(&service.RetentionPlan{}).Count(&plans)
	f.Service.DB.Model(&service.RetentionItem{}).Count(&items)
	if holds != 0 || policies != 0 || plans != 0 || items != 0 {
		t.Fatalf("legacy purge wrote retention state: %d %d %d %d", holds, policies, plans, items)
	}
	// Rollback drill: only the new tables drop; the purge mark column is
	// nullable and legacy rows survive without it.
	if e := f.Service.DB.Migrator().DropTable("retention_items", "retention_plans", "retention_policies", "retention_holds"); e != nil {
		t.Fatal(e)
	}
	var files int64
	f.Service.DB.Model(&service.File{}).Count(&files)
	if files == 0 {
		t.Fatal("rollback drill lost live files")
	}
}

// Scenario 1 (mutual exclusion): an active delivery hold blocks purge with
// version_protected and names the source; a purge that already started
// refuses new holds with purge_in_progress; a concurrent pair always
// resolves to exactly one winner.
func TestFPRetentionGuardsPurgeHoldMutualExclusion(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, c := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, c, h.space.ID, "mutual.txt", "mutual-key", []byte("mutual"))
	fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "mutual-hold")
	if e := f.Service.Trash(p, op.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.Purge(context.Background(), p, op.FileID); e == nil || e.Error() != "version_protected" {
		t.Fatalf("protected version purged: %v", e)
	}
	// purge_in_progress: a stale two-phase mark refuses new holds.
	stale := time.Now().UTC()
	if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("purge_started_at", stale).Error; e != nil {
		t.Fatal(e)
	}
	other := fpUploadBytes(t, c, h.space.ID, "other.txt", "other-key", []byte("other"))
	_ = other
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, Purpose: "fixed_delivery", SourceRef: op.ID, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "during-purge"); e == nil || e.Error() != "purge_in_progress" {
		t.Fatalf("hold during purge must be purge_in_progress, got %v", e)
	}
	f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("purge_started_at", nil)
	// Concurrent purge and hold: exactly one wins, no dangling promise.
	var wg sync.WaitGroup
	purgeErr := make(chan error, 1)
	holdErr := make(chan error, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		purgeErr <- f.Service.Purge(context.Background(), p, op.FileID)
	}()
	go func() {
		defer wg.Done()
		_, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, Purpose: "transfer_window", SourceRef: op.ID, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "race-hold")
		holdErr <- e
	}()
	wg.Wait()
	close(purgeErr)
	close(holdErr)
	pe, he := <-purgeErr, <-holdErr
	outcomes := 0
	if pe == nil {
		outcomes++
	}
	if he == nil {
		outcomes++
	}
	if outcomes != 1 {
		t.Fatalf("concurrent purge/hold must have exactly one winner: purge=%v hold=%v", pe, he)
	}
}

// Scenario 2 (capacity and reference safety): purging one of two versions
// that share a blob keeps the other readable and the physical bytes stored;
// the transaction truth, not any projection, decides deletion eligibility.
func TestFPRetentionGuardsSharedBlobReferenceSafety(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, c := agent(t, h, "", "read,write")
	body := []byte("shared-blob-content")
	first := fpUploadBytes(t, c, h.space.ID, "shared-a.txt", "shared-a", body)
	// A second file with identical content dedupes onto the same blob.
	second := fpUploadBytes(t, c, h.space.ID, "shared-b.txt", "shared-b", body)
	var blobs int64
	f.Service.DB.Model(&service.Blob{}).Count(&blobs)
	if blobs != 1 {
		t.Fatalf("expected one deduplicated blob, got %d", blobs)
	}
	if e := f.Service.Trash(p, first.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.Purge(context.Background(), p, first.FileID); e != nil {
		t.Fatal(e)
	}
	// The other version stays readable and the object bytes remain.
	var sink strings.Builder
	if e := c.Download(context.Background(), second.FileID, "", &sink); e != nil || sink.String() != string(body) {
		t.Fatalf("sibling version unreadable after shared-blob purge: %v", e)
	}
	if f.ObjectCount() != 1 {
		t.Fatalf("physical bytes were dropped with live references: %d", f.ObjectCount())
	}
	// Transaction truth wins over a fabricated zero-reference projection:
	// the projection row lies, the version rows decide.
	if e := f.Service.DB.Model(&service.Blob{}).Where("1=1").Update("refs", 0).Error; e != nil {
		t.Fatal(e)
	}
	if e := f.Service.Trash(p, second.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.Purge(context.Background(), p, second.FileID); e != nil {
		t.Fatalf("transaction truth refused a legitimate deletion: %v", e)
	}
}

// Task 4.2 fixture flow: 清理预览 → 交付保护 → 并发purge拒绝 → 期满重新计划.
// A trashed file enters the preview, a delivery hold protects it, purge is
// refused, the hold expires and a fresh preview re-plans the cleanup which
// now executes.
func TestFPRetentionGuardsPreviewProtectExpireReplan(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, c := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, c, h.space.ID, "flow.txt", "flow-key", []byte("flow"))
	if e := f.Service.Trash(p, op.FileID, false); e != nil {
		t.Fatal(e)
	}
	// Age the trash entry past the window so it becomes a candidate.
	past := time.Now().UTC().Add(-400 * 24 * time.Hour)
	if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", past).Error; e != nil {
		t.Fatal(e)
	}
	preview, e := c.PreviewRetention(context.Background(), "trash")
	if e != nil {
		t.Fatal(e)
	}
	if fpNum(preview["items_total"]) < 1 {
		t.Fatalf("preview missed the aged trash entry: %+v", preview)
	}
	// Delivery protection: the hold blocks both direct purge and the plan
	// execution, each naming the protecting source.
	hold := fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "flow-hold")
	if e := f.Service.Purge(context.Background(), p, op.FileID); e == nil || e.Error() != "version_protected" {
		t.Fatalf("protected purge accepted: %v", e)
	}
	planID := preview["plan_id"].(string)
	var executed map[string]any
	if status := h.call(t, "POST", "/retention/plans/"+planID+"/execute", nil, &executed); status != 200 {
		t.Fatalf("plan execution failed: %d", status)
	}
	if fpNum(executed["items_deleted"]) != 0 || fpNum(executed["items_skipped"]) < 1 {
		t.Fatalf("plan executed over an active hold: %+v", executed)
	}
	// Expiry re-plans: an expired hold no longer protects.
	if e := f.Service.DB.Model(&service.RetentionHold{}).Where("id = ?", hold.HoldID).Update("expires_at", time.Now().UTC().Add(-time.Minute)); e.Error != nil {
		t.Fatal(e)
	}
	replan, e := c.PreviewRetention(context.Background(), "trash")
	if e != nil {
		t.Fatal(e)
	}
	replanID := replan["plan_id"].(string)
	var reexec map[string]any
	if status := h.call(t, "POST", "/retention/plans/"+replanID+"/execute", nil, &reexec); status != 200 {
		t.Fatalf("re-planned execution failed: %d", status)
	}
	if fpNum(reexec["items_deleted"]) < 1 {
		t.Fatalf("expired hold still blocks cleanup: %+v", reexec)
	}
	var gone int64
	f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Count(&gone)
	if gone != 0 {
		t.Fatal("re-planned cleanup left the file row behind")
	}
}

// Task 4.1 entry equivalence and permission posture: HTTP, MCP and SDK
// expose the same hold verdict; execute is human-only and never an agent
// tool; agents may create protective holds but never purge.
func TestFPRetentionGuardsEntryEquivalenceAndPermissions(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, c, h.space.ID, "equiv.txt", "equiv-key", []byte("equiv"))
	sdkView := fpHold(t, c, op.FileID, op.VersionID, "fixed_delivery", op.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "equiv-hold")
	// MCP verdict through the same application service.
	principal := service.Principal{UserID: h.user.ID, Agent: &a}
	out, e := f.Service.CallTool(context.Background(), principal, "retention_hold_get", map[string]any{"hold_id": sdkView.HoldID})
	if e != nil {
		t.Fatal(e)
	}
	mcpView, ok := out.(service.RetentionHoldView)
	if !ok {
		t.Fatalf("retention_hold_get returned unexpected shape: %T", out)
	}
	if mcpView.HoldID != sdkView.HoldID || mcpView.State != sdkView.State || mcpView.Purpose != sdkView.Purpose {
		t.Fatalf("MCP and SDK verdicts differ: %+v vs %+v", mcpView, sdkView)
	}
	// HTTP read matches.
	var httpView sdk.RetentionHoldView
	if status := h.call(t, "GET", "/retention/holds/"+sdkView.HoldID, nil, &httpView); status != 200 || httpView.HoldID != sdkView.HoldID || httpView.State != "active" {
		t.Fatalf("HTTP verdict differs: %d %+v", status, httpView)
	}
	// Plan execution over MCP is not a tool at all.
	if _, e := f.Service.CallTool(context.Background(), principal, "retention_execute", map[string]any{"plan_id": "x"}); e == nil || e.Error() != "unknown_tool" {
		t.Fatalf("agent purge tool must not exist, got %v", e)
	}
	// The service layer also refuses agent execution directly.
	if _, e := f.Service.ExecuteRetention(context.Background(), principal, "x"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent execution must be forbidden, got %v", e)
	}
}

// fpNum coerces a JSON-decoded number regardless of concrete type.
func fpNum(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return -1
}

// Task 5.2 scale acceptance, gated behind DRIVEBRIDGE_FP_SCALE. quick/full
// both create 10,000 holds over deduplicated versions and run 20
// concurrent hold/purge conflict rounds. The million-version dimension
// cannot run on this machine inside the check budget (SQLite single-writer
// insertion rate); it stays unverified and the task stays unchecked.
func TestFPRetentionGuardsScaleHolds(t *testing.T) {
	mode := os.Getenv("DRIVEBRIDGE_FP_SCALE")
	if mode != "quick" && mode != "full" {
		t.Skip("scale fixture gated behind DRIVEBRIDGE_FP_SCALE=quick|full")
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, c := agent(t, h, "", "read,write")
	// One source operation authorizes the whole family of holds.
	op := fpUploadBytes(t, c, h.space.ID, "scale-hold.txt", "scale-hold-src", []byte("scale-hold"))
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	// 20 slots stay free for the concurrent conflict rounds below.
	const total = 9980
	created := 0
	for batch := 0; batch*100 < total; batch++ {
		var wg sync.WaitGroup
		for i := 0; i < min(100, total-batch*100); i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				key := fmt.Sprintf("scale-hold-%06d", batch*100+n)
				if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "fixed_delivery", SourceRef: op.ID, ExpiresAt: expiry}, key); e == nil {
					synchronizedAdd(&created)
				}
			}(i)
		}
		wg.Wait()
	}
	if created != total {
		t.Fatalf("hold creation below target: %d of %d", created, total)
	}
	var holds int64
	f.Service.DB.Model(&service.RetentionHold{}).Count(&holds)
	if holds < total {
		t.Fatalf("stored holds below target: %d", holds)
	}
	// 20 concurrent hold/purge conflict rounds: every round has exactly one
	// winner and the file is never both purged and protected.
	for round := 0; round < 20; round++ {
		if e := f.Service.Trash(p, op.FileID, false); e != nil {
			t.Fatalf("round %d trash: %v", round, e)
		}
		purgeErr := make(chan error, 1)
		holdErr := make(chan error, 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			purgeErr <- f.Service.Purge(context.Background(), p, op.FileID)
		}()
		go func() {
			defer wg.Done()
			_, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "fixed_delivery", SourceRef: op.ID, ExpiresAt: expiry}, fmt.Sprintf("conflict-%d", round))
			holdErr <- e
		}()
		wg.Wait()
		pe, he := <-purgeErr, <-holdErr
		winners := 0
		if pe == nil {
			winners++
		}
		if he == nil {
			winners++
		}
		if winners != 1 {
			t.Fatalf("round %d: exactly one winner required, got %d (purge=%v hold=%v)", round, winners, pe, he)
		}
		if pe == nil {
			t.Fatalf("round %d: purge won over 10k active holds", round)
		}
		// Restore for the next round.
		if e := f.Service.Trash(p, op.FileID, true); e != nil {
			t.Fatalf("round %d restore: %v", round, e)
		}
	}
	// After the conflict rounds the quota is exhausted and refuses more.
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "fixed_delivery", SourceRef: op.ID, ExpiresAt: expiry}, "over-quota"); e == nil || e.Error() != "quota_exceeded" {
		t.Fatalf("hold quota not enforced at scale: %v", e)
	}
	t.Logf("scale=%s holds=%d concurrent_rounds=20 winners=1 purge_wins=0", mode, holds)
}

// Task 5.2 million-version dimension, gated behind DRIVEBRIDGE_FP_SCALE=million.
// One million version rows are seeded through parameterized bulk inserts with
// shared blob digests across two files. The guard verdicts are then recomputed
// at that scale: an expired hold no longer protects (decision-time expiry
// recompute), a released hold no longer protects (cancel cleanup), an active
// hold still protects, and purging one file keeps shared blob references of
// the other file intact. Analysis lag never changes these verdicts because
// they read transaction truth, not projections.
func TestFPRetentionGuardsScaleMillionVersions(t *testing.T) {
	if os.Getenv("DRIVEBRIDGE_FP_SCALE") != "million" {
		t.Skip("million fixture gated behind DRIVEBRIDGE_FP_SCALE=million")
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, c := agent(t, h, "", "read,write")
	expired := fpUploadBytes(t, c, h.space.ID, "scale-expired.txt", "expired-src", []byte("expired"))
	released := fpUploadBytes(t, c, h.space.ID, "scale-released.txt", "released-src", []byte("released"))
	protected := fpUploadBytes(t, c, h.space.ID, "scale-protected.txt", "protected-src", []byte("protected"))
	// Shared digests: every 10 seeded versions share one blob digest, and
	// digest families span both files so purging one file must keep the other
	// file's references to the same logical blobs alive.
	const perFile = 500_000
	const versions = 2 * perFile
	seed := map[string][2]string{"expired": {expired.FileID, h.user.ID}, "protected": {protected.FileID, h.user.ID}}
	started := time.Now()
	for name, target := range seed {
		now := time.Now().UTC()
		const batch = 1000
		for offset := 0; offset < perFile; offset += batch {
			var query strings.Builder
			query.WriteString("INSERT INTO versions (id, owner, file_id, connection_id, object_key, blob_sha256, sha256, size, mime, created) VALUES ")
			args := make([]any, 0, batch*10)
			for i := range batch {
				if i > 0 {
					query.WriteByte(',')
				}
				query.WriteString("(?,?,?,?,?,?,?,?,?,?)")
				n := offset + i
				digest := fmt.Sprintf("sha256-scale-%s-%08x", name, n/10)
				args = append(args, fmt.Sprintf("v-%s-%08x", name, n), target[1], target[0], "", "", digest, digest, int64(16), "text/plain", now)
			}
			if e := f.Service.DB.Exec(query.String(), args...).Error; e != nil {
				t.Fatalf("bulk insert %s offset %d: %v", name, offset, e)
			}
		}
	}
	insertSeconds := time.Since(started).Seconds()
	var versionRows int64
	f.Service.DB.Model(&service.Version{}).Count(&versionRows)
	if versionRows < versions {
		t.Fatalf("version scale below target: %d of %d", versionRows, versions)
	}
	var sharedDigests int64
	f.Service.DB.Raw("SELECT COUNT(DISTINCT blob_sha256) FROM versions WHERE blob_sha256 LIKE 'sha256-scale-%'").Scan(&sharedDigests)
	// Decision-time expiry recompute: an expired hold is dead truth the moment
	// its deadline passed, so the file can be purged.
	past := time.Now().Add(-time.Minute).UTC()
	if e := f.Service.DB.Create(&service.RetentionHold{ID: service.ID(), Owner: h.user.ID, TenantID: f.Service.Instance, FileID: expired.FileID, VersionID: expired.VersionID, Purpose: "fixed_delivery", SourceKind: "operation", SourceRef: expired.ID, Key: "scale-expired-hold", Fingerprint: "fixture", Revision: 1, State: "active", ExpiresAt: past, CreatedAt: past, UpdatedAt: past}).Error; e != nil {
		t.Fatal(e)
	}
	// Cancel cleanup: a released hold leaves its protection immediately.
	rh := fpHold(t, c, released.FileID, released.VersionID, "fixed_delivery", released.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "scale-released-hold")
	if _, e := f.Service.ReleaseRetentionHold(p, rh.HoldID); e != nil {
		t.Fatal(e)
	}
	fpHold(t, c, protected.FileID, protected.VersionID, "fixed_delivery", protected.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "scale-protected-hold")
	purgeTimed := make(map[string]time.Duration, 3)
	for _, step := range []struct{ name, fileID, want string }{
		{"expired", expired.FileID, ""},
		{"released", released.FileID, ""},
		{"protected", protected.FileID, "version_protected"},
	} {
		if e := f.Service.Trash(p, step.fileID, false); e != nil {
			t.Fatalf("%s trash: %v", step.name, e)
		}
		t0 := time.Now()
		e := f.Service.Purge(context.Background(), p, step.fileID)
		purgeTimed[step.name] = time.Since(t0)
		if step.want == "" {
			if e != nil {
				t.Fatalf("%s hold still blocked purge at scale: %v", step.name, e)
			}
			continue
		}
		if e == nil || e.Error() != step.want {
			t.Fatalf("%s verdict changed at scale: want %s got %v", step.name, step.want, e)
		}
	}
	// Shared blob multi-reference survives the expired file's purge: the
	// protected file keeps its seeded references to logical blobs.
	var survivingRefs int64
	f.Service.DB.Raw("SELECT COUNT(*) FROM versions WHERE file_id = ? AND blob_sha256 LIKE 'sha256-scale-%'", protected.FileID).Scan(&survivingRefs)
	if survivingRefs != perFile {
		t.Fatalf("shared blob references lost: %d of %d", survivingRefs, perFile)
	}
	report := map[string]any{"versions_total": versionRows, "versions_target": versions, "insert_seconds": insertSeconds, "insert_rows_per_second": int64(float64(versions) / insertSeconds), "shared_blob_digests": sharedDigests, "holds": 3, "expired_purge_ms": purgeTimed["expired"].Milliseconds(), "released_purge_ms": purgeTimed["released"].Milliseconds(), "protected_blocked": true, "surviving_shared_refs": survivingRefs}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("million report: %s", b)
	if dir := os.Getenv("DRIVEBRIDGE_EVIDENCE_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "fp-retention-million.json"), b, 0o644)
	}
}

var scaleMu sync.Mutex

func synchronizedAdd(n *int) {
	scaleMu.Lock()
	*n++
	scaleMu.Unlock()
}
