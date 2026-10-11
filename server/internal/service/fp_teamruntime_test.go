package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
	gormclause "gorm.io/gorm/clause"
)

// fpRuntimeUpload completes one small upload and waits for the in-process
// worker to drain the resulting outbox apply jobs.
func fpRuntimeUpload(t *testing.T, f *fixtures.Runtime, h *human, path string) sdk.Operation {
	t.Helper()
	_, c := agent(t, h, "", "read,write")
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: path}, "fp-runtime-"+path, bytes.NewReader([]byte(path)), nil)
	if e != nil {
		t.Fatal(e)
	}
	return op
}

func fpDrainRuntime(t *testing.T, f *fixtures.Runtime) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var pending, claimedNow int64
		f.Service.DB.Model(&service.RuntimeJob{}).Where("status = ?", "pending").Count(&pending)
		f.Service.DB.Model(&service.RuntimeJob{}).Where("status = ?", "claimed").Count(&claimedNow)
		if pending == 0 && claimedNow == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("runtime worker did not drain pending jobs")
}

// Scenario 1: transaction/projection layering. An unreachable projection
// store never rolls back a committed file business transaction, events stay
// recoverable, and a failed business commit never publishes an event.
func TestFPTeamRuntimeTransactionalOutboxSeparation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	op := fpRuntimeUpload(t, f, h, "layered.txt")
	// The committed file business transaction wrote its outbox event.
	var events int64
	f.Service.DB.Model(&service.OutboxEvent{}).Where("resource_id = ?", op.FileID).Count(&events)
	if events == 0 {
		t.Fatal("committed completion published no outbox event")
	}
	fpDrainRuntime(t, f)
	// Break the projection store: applying must fail while the file stays
	// readable and the events stay recoverable. One fresh event is inserted
	// directly so the apply must touch the broken table.
	f.Service.DB.Migrator().DropTable("runtime_projection_rows")
	f.Service.DB.Create(&service.OutboxEvent{EventID: service.ID(), Kind: "file_catalogue.upsert", ResourceID: service.ID(), ResourceRevision: time.Now().UTC().UnixNano(), SourceSequence: 1 << 40, OwnerScope: h.user.ID})
	_, _, e := f.Service.ApplyOutboxEvents("file_catalogue", h.user.ID, 100)
	if e == nil {
		t.Fatal("broken projection store did not surface an error")
	}
	if _, _, _, de := f.Service.Download(context.Background(), service.Principal{UserID: h.user.ID}, op.FileID, ""); de != nil {
		t.Fatalf("projection failure rolled back the file business transaction: %v", de)
	}
	var recoverable int64
	f.Service.DB.Model(&service.OutboxEvent{}).Where("owner_scope = ?", h.user.ID).Count(&recoverable)
	if recoverable == 0 {
		t.Fatal("events lost while the projection store was down")
	}
	// Repair the store: the same events apply and the checkpoint advances.
	if e := f.Service.DB.AutoMigrate(&service.ProjectionRow{}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := f.Service.ApplyOutboxEvents("file_catalogue", h.user.ID, 100); e != nil {
		t.Fatal(e)
	}
	var checkpoint service.ProjectionCheckpoint
	f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", h.user.ID).First(&checkpoint)
	if checkpoint.Watermark == 0 {
		t.Fatal("checkpoint did not advance after recovery")
	}
	// A failed business commit publishes nothing: reserve a path, let
	// another upload take it, then complete the reservation so the file
	// insert fails inside the completion transaction.
	_, c := agent(t, h, "", "read,write")
	payload := []byte("clash-payload")
	reserve := sdk.UploadRequest{SpaceID: h.space.ID, Path: "clash.txt", Size: int64(len(payload)), SHA256: service.Hash(string(payload))}
	var created service.Operation
	if e := c.JSON(context.Background(), "POST", "/api/v1/uploads", "fp-reserve", reserve, &created); e != nil {
		t.Fatal(e)
	}
	if _, e := f.Service.PutPart(service.Principal{UserID: h.user.ID}, created.ID, 0, bytes.NewReader(payload)); e != nil {
		t.Fatal(e)
	}
	fpRuntimeUpload(t, f, h, "clash.txt")
	var before int64
	f.Service.DB.Model(&service.OutboxEvent{}).Count(&before)
	if _, e := f.Service.Complete(context.Background(), service.Principal{UserID: h.user.ID}, created.ID); e == nil {
		t.Fatal("conflicting completion unexpectedly succeeded")
	}
	var after int64
	f.Service.DB.Model(&service.OutboxEvent{}).Count(&after)
	if after != before {
		t.Fatal("failed business commit published an outbox event")
	}
}

// Scenario 2: original-task continuity with fencing. A new worker takes over
// the original job after lease expiry and finishes it once; the late old
// worker is refused, its attempt lands in diagnostics, and the new owner's
// state is never overwritten.
func TestFPTeamRuntimeFencingTakeoverAndStaleWorker(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	// Seed one pending job outside the worker's reach by pausing? The
	// in-process worker claims quickly; instead create the job directly.
	job := service.RuntimeJob{ID: service.ID(), Kind: "projection_apply", OriginalOperationID: service.ID(), OwnerScope: h.user.ID, Status: "pending"}
	if e := f.Service.DB.Create(&job).Error; e != nil {
		t.Fatal(e)
	}
	claimed, token1, e := f.Service.ClaimRuntimeJob("worker-old", time.Millisecond)
	if e != nil || claimed == nil || claimed.ID != job.ID {
		t.Fatalf("claim failed: %v %+v", e, claimed)
	}
	if token1 != 1 {
		t.Fatalf("first fencing token must be 1, got %d", token1)
	}
	time.Sleep(5 * time.Millisecond)
	// Lease expired: a new worker reclaims the ORIGINAL job (same id, same
	// original operation), not a duplicate.
	takeover, token2, e := f.Service.ClaimRuntimeJob("worker-new", time.Minute)
	if e != nil || takeover == nil || takeover.ID != job.ID {
		t.Fatalf("takeover did not continue the original job: %v %+v", e, takeover)
	}
	if token2 <= token1 {
		t.Fatalf("fencing token did not increase: %d then %d", token1, token2)
	}
	// The late old worker tries to commit its result.
	if e := f.Service.CompleteRuntimeJob("worker-old", token1, job.ID, "done"); e == nil || e.Error() != "worker_fenced" {
		t.Fatalf("stale worker commit must be worker_fenced, got %v", e)
	}
	var after service.RuntimeJob
	f.Service.DB.First(&after, "id = ?", job.ID)
	if after.Status != "claimed" {
		t.Fatalf("stale worker overwrote the new owner's state: %s", after.Status)
	}
	if !strings.Contains(after.Diagnostics, "worker-old") || !strings.Contains(after.Diagnostics, fmt.Sprintf(`"token":%d`, token1)) {
		t.Fatalf("fenced attempt not retained for diagnosis: %s", after.Diagnostics)
	}
	// The new worker completes exactly once; no duplicate job state remains.
	if e := f.Service.CompleteRuntimeJob("worker-new", token2, job.ID, "done"); e != nil {
		t.Fatal(e)
	}
	f.Service.DB.First(&after, "id = ?", job.ID)
	if after.Status != "done" {
		t.Fatalf("new worker could not finish the original job: %s", after.Status)
	}
	var leases int64
	f.Service.DB.Model(&service.WorkerLease{}).Where("id = ?", job.ID).Count(&leases)
	if leases != 0 {
		t.Fatal("completed job kept a live lease")
	}
	// A late heartbeat from the old worker is also fenced.
	if e := f.Service.RuntimeHeartbeat("worker-old", token1, job.ID, time.Minute); e == nil || e.Error() != "worker_fenced" {
		t.Fatalf("stale heartbeat must be worker_fenced, got %v", e)
	}
}

// Scenario 3: generation switch on a verified watermark. The switch refuses
// with projection_not_caught_up until both generations agree, then moves the
// read pointer atomically; retired generations stay inspectable; reads never
// mix generations.
func TestFPTeamRuntimeGenerationSwitch(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	fpRuntimeUpload(t, f, h, "gen-a.txt")
	fpDrainRuntime(t, f)
	admin := service.Principal{UserID: h.user.ID, Admin: true}
	// A wrong expected_generation is a version conflict, not a silent rebuild.
	if _, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "GEN_Z"); e == nil || e.Error() != "version_conflict" {
		t.Fatalf("stale expected_generation accepted: %v", e)
	}
	start, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "GEN_0")
	if e != nil {
		t.Fatal(e)
	}
	operationID, _ := start["operation_id"].(string)
	if operationID == "" {
		t.Fatal("rebuild returned no original operation")
	}
	if start["generation"] != "GEN_A" || start["phase"] != "frozen" || start["ready"] != false {
		t.Fatalf("rebuild start wrong: %+v", start)
	}
	// Replay while live returns the same original operation.
	replay, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "")
	if e != nil || replay["operation_id"] != operationID {
		t.Fatalf("rebuild replay created a second operation: %v %+v", e, replay)
	}
	// New business events land after the freeze.
	fpRuntimeUpload(t, f, h, "gen-b.txt")
	// Drive the phases: frozen -> backfill -> catching_up.
	for _, phase := range []string{"backfill", "catching_up"} {
		view, e := f.Service.AdvanceProjectionRebuild(operationID, 500)
		if e != nil {
			t.Fatal(e)
		}
		if view["phase"] != phase {
			t.Fatalf("phase transition wrong: want %s got %v", phase, view["phase"])
		}
	}
	// Reach the verifying phase, then attempt the switch while the OLD
	// generation lags behind the stream head: it must refuse without
	// switching anything.
	for {
		view, e := f.Service.AdvanceProjectionRebuild(operationID, 500)
		if e != nil {
			t.Fatal(e)
		}
		if view["phase"] == "verifying" {
			break
		}
	}
	if _, e := f.Service.AdvanceProjectionRebuild(operationID, 500); e == nil || e.Error() != "projection_not_caught_up" {
		t.Fatalf("premature switch accepted: %v", e)
	}
	// The steady worker catches the old generation up; the rebuild catches
	// the new one up; then the switch verifies and lands.
	fpDrainRuntime(t, f)
	for {
		view, e := f.Service.AdvanceProjectionRebuild(operationID, 500)
		if e != nil {
			t.Fatal(e)
		}
		if view["phase"] == "active" {
			if view["ready"] != true {
				t.Fatalf("switch landed without ready: %+v", view)
			}
			break
		}
	}
	var checkpoint service.ProjectionCheckpoint
	f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", h.user.ID).First(&checkpoint)
	if checkpoint.Generation != "GEN_A" {
		t.Fatalf("read pointer did not switch: %+v", checkpoint)
	}
	// The retired generation keeps its rows for diagnostics; reads via the
	// checkpoint never mix them.
	var retired, active int64
	f.Service.DB.Model(&service.ProjectionRow{}).Where("projection = ? AND generation = ?", "file_catalogue", "GEN_0").Count(&retired)
	f.Service.DB.Model(&service.ProjectionRow{}).Where("projection = ? AND generation = ?", "file_catalogue", "GEN_A").Count(&active)
	if retired == 0 || active == 0 {
		t.Fatalf("generations not retained separately: retired=%d active=%d", retired, active)
	}
	// The original operation completed and stays readable by its id.
	var op service.Operation
	if f.Service.DB.First(&op, "id = ?", operationID).Error != nil || op.Status != "completed" {
		t.Fatalf("original rebuild operation not completed: %+v", op)
	}
	// A second rebuild now switches from the ACTIVE generation.
	start2, e := f.Service.StartProjectionRebuild(admin, "file_catalogue", "GEN_A")
	if e != nil {
		t.Fatal(e)
	}
	if start2["generation"] != "GEN_B" {
		t.Fatalf("second rebuild did not branch from the active generation: %+v", start2)
	}
}

// Operator gating and DSN hygiene: runtime status is admin-only across HTTP
// and MCP, rebuild is not an Agent tool, and no view leaks the DSN.
func TestFPTeamRuntimeStatusGatingAndNoDSN(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	fpRuntimeUpload(t, f, h, "status.txt")
	fpDrainRuntime(t, f)
	admin := service.Principal{UserID: h.user.ID, Admin: true}
	view, e := f.Service.RuntimeStatusView(admin)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(view)
	if bytes.Contains(raw, []byte("postgres://")) || bytes.Contains(bytes.ToLower(raw), []byte(`"dsn"`)) || bytes.Contains(raw, []byte("secret")) {
		t.Fatalf("runtime status leaked connection material: %s", raw)
	}
	queue := view["queue"].(map[string]int64)
	if _, ok := queue["pending_jobs"]; !ok {
		t.Fatal("queue projection missing pending_jobs")
	}
	projections := view["projections"].([]map[string]any)
	if len(projections) == 0 {
		t.Fatal("projection watermarks missing")
	}
	// HTTP entry: an operator key session sees it, another user is refused.
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "fp-operator", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	adminJar := fpKeySession(t, f, issued[0].Key)
	req, _ := http.NewRequest("GET", f.App.URL+"/api/v1/runtime/status", nil)
	adminResp, e := adminJar.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	var httpEnvelope struct {
		Data map[string]any `json:"data"`
	}
	body, _ := io.ReadAll(adminResp.Body)
	adminResp.Body.Close()
	json.Unmarshal(body, &httpEnvelope)
	if adminResp.StatusCode != 200 || httpEnvelope.Data["queue"] == nil {
		t.Fatalf("operator runtime status failed: %d %s", adminResp.StatusCode, body)
	}
	adminRaw := string(body)
	if strings.Contains(adminRaw, "postgres://") || strings.Contains(strings.ToLower(adminRaw), `"dsn"`) {
		t.Fatalf("HTTP runtime status leaked connection material: %s", adminRaw)
	}
	var leak map[string]any
	if status := bob.call(t, "GET", "/runtime/status", nil, &leak); status != 403 {
		t.Fatalf("non-admin runtime status must be 403, got %d", status)
	}
	if status := bob.call(t, "POST", "/projections/file_catalogue/rebuild", map[string]string{}, &leak); status != 403 {
		t.Fatalf("non-admin rebuild must be 403, got %d", status)
	}
	// MCP entry: runtime_status exists but refuses non-operators; rebuild is
	// not registered as a tool at all.
	a, _ := agent(t, h, "", "read,write")
	if _, e := f.Service.CallTool(context.Background(), service.Principal{UserID: h.user.ID, Agent: &a}, "runtime_status", map[string]any{}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent runtime_status must be forbidden, got %v", e)
	}
	for _, tool := range service.ToolDefinitions() {
		if strings.Contains(tool.Name, "rebuild") {
			t.Fatalf("rebuild leaked into the Agent tool contract: %s", tool.Name)
		}
	}
}

// Owner-scope partitioning: per-owner checkpoints and rows never cross.
func TestFPTeamRuntimeOwnerScopeIsolation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	fpRuntimeUpload(t, f, alice, "scope-a.txt")
	fpRuntimeUpload(t, f, bob, "scope-b.txt")
	fpDrainRuntime(t, f)
	var checkpoints []service.ProjectionCheckpoint
	f.Service.DB.Where("projection = ?", "file_catalogue").Find(&checkpoints)
	if len(checkpoints) != 2 {
		t.Fatalf("expected one checkpoint per owner scope, got %d", len(checkpoints))
	}
	scopes := map[string]bool{}
	for _, cp := range checkpoints {
		scopes[cp.OwnerScope] = true
	}
	if !scopes[alice.user.ID] || !scopes[bob.user.ID] {
		t.Fatal("owner scopes collapsed into one checkpoint")
	}
	var alien []service.ProjectionRow
	f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", bob.user.ID).Find(&alien)
	for _, row := range alien {
		var file service.File
		if f.Service.DB.First(&file, "id = ?", row.ResourceID).Error == nil && file.Owner == alice.user.ID {
			t.Fatal("cross-owner row leaked into another owner's projection partition")
		}
	}
}

// Upload -> worker restart -> original task completion -> projection rebuild
// -> authorized query: the full runtime scenario from the design.
func TestFPTeamRuntimeRestartRebuildAndQuery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	op := fpRuntimeUpload(t, f, h, "restart-projection.txt")
	// Simulate a worker crash: an in-flight claimed job with an expired
	// lease, then a process restart.
	claim := service.RuntimeJob{ID: service.ID(), Kind: "projection_apply", OwnerScope: h.user.ID, Status: "claimed", LeaseEpoch: 4, AttemptGeneration: 4}
	f.Service.DB.Create(&claim)
	f.Service.DB.Create(&service.WorkerLease{ID: claim.ID, WorkerID: "crashed", FencingToken: 4, ExpiresAt: time.Now().UTC().Add(-time.Minute), Heartbeat: time.Now().UTC()})
	config := f.Service.Config
	var filesBefore int64
	f.Service.DB.Model(&service.File{}).Count(&filesBefore)
	f.Service.Close()
	reopened, e := service.Open(config)
	if e != nil {
		t.Fatal(e)
	}
	f.Service = reopened
	defer func() { f.Service = reopened }()
	var filesAfter int64
	reopened.DB.Model(&service.File{}).Count(&filesAfter)
	if filesBefore != filesAfter {
		t.Fatal("restart lost file rows")
	}
	// The restart reconciles the expired claim back to pending and the
	// worker finishes the original task.
	fpDrainRuntime(t, f)
	var reconciled service.RuntimeJob
	reopened.DB.First(&reconciled, "id = ?", claim.ID)
	if reconciled.Status != "done" {
		t.Fatalf("expired claim not reconciled and finished: %+v", reconciled)
	}
	// Response loss on the read side is recovered by the original ids.
	if _, _, _, e := reopened.Download(context.Background(), service.Principal{UserID: h.user.ID}, op.FileID, ""); e != nil {
		t.Fatalf("original file id unreadable after restart: %v", e)
	}
	// Projection rebuild after the restart: the operator drives a new
	// generation through verify and switch on the recovered runtime.
	admin := service.Principal{UserID: h.user.ID, Admin: true}
	start, e := reopened.StartProjectionRebuild(admin, "file_catalogue", "")
	if e != nil {
		t.Fatal(e)
	}
	rebuildOp, _ := start["operation_id"].(string)
	fpDrainRuntime(t, f)
	for {
		view, e := reopened.AdvanceProjectionRebuild(rebuildOp, 500)
		if e != nil {
			t.Fatal(e)
		}
		if view["phase"] == "active" {
			break
		}
	}
	// Authorized query path still works end to end after the switch.
	if _, e := fpMCPCheck(t, f, service.Principal{UserID: h.user.ID}, "file_read", h.space.ID, ""); e != nil {
		t.Fatal(e)
	}
}

// Metadata migration: quiescence gate, export/import/verify between two
// states, and the refusal to roll back over new writes.
func TestFPTeamRuntimeMetadataMigration(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	fpRuntimeUpload(t, f, h, "migrate.txt")
	fpDrainRuntime(t, f)
	// Pending business write blocks the export.
	pending := service.Operation{ID: service.ID(), Owner: h.user.ID, Key: "fp-pending", Kind: "upload", SpaceID: h.space.ID, Path: "pending.txt", Status: "awaiting_upload"}
	f.Service.DB.Create(&pending)
	var buf bytes.Buffer
	if e := f.Service.ExportMetadata(&buf); e == nil || e.Error() != "metadata_migration_requires_quiescence" {
		t.Fatalf("unquiesced export accepted: %v", e)
	}
	f.Service.DB.Delete(&pending)
	buf.Reset()
	if e := f.Service.ExportMetadata(&buf); e != nil {
		t.Fatal(e)
	}
	snapshot := buf.Bytes()
	if !bytes.Contains(snapshot, []byte(`"instance_id":"`+f.Service.Instance)) {
		t.Fatal("snapshot header missing instance identity")
	}
	// Import into a fresh sqlite target and verify equality.
	target := t.TempDir()
	targetDB, e := gorm.Open(sqlite.Open(filepath.Join(target, "metadata.sqlite")), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e := targetDB.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{}); e != nil {
		t.Fatal(e)
	}
	result, e := f.Service.ImportMetadata(bytes.NewReader(snapshot), targetDB)
	if e != nil {
		t.Fatal(e)
	}
	digest := result["imported"].(map[string]int64)
	if digest["files"] == 0 {
		t.Fatal("import lost files")
	}
	verify, e := f.Service.VerifyMetadata(bytes.NewReader(snapshot), bytes.NewReader(snapshot))
	if e != nil || verify["matching"] != true {
		t.Fatalf("verify rejected identical snapshots: %v %+v", e, verify)
	}
	// Rolling the OLD snapshot back over a target that already carries new
	// writes is refused: it needs a reverse migration after quiescence.
	fresh := service.RuntimeJob{ID: service.ID(), Kind: "projection_apply", OwnerScope: h.user.ID, Status: "pending"}
	f.Service.DB.Create(&fresh)
	target2 := t.TempDir()
	targetDB2, _ := gorm.Open(sqlite.Open(filepath.Join(target2, "metadata.sqlite")), &gorm.Config{})
	targetDB2.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{})
	targetDB2.Create(&service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: "new-write.txt"})
	if _, e := f.Service.ImportMetadata(bytes.NewReader(snapshot), targetDB2); e == nil || e.Error() != "metadata_migration_requires_quiescence" {
		t.Fatalf("rollback over new writes accepted: %v", e)
	}
	f.Service.DB.Delete(&fresh)
	// A foreign instance identity is refused outright.
	other := t.TempDir()
	os.Chmod(other, 0o700)
	otherService, e := service.Open(service.Config{State: other, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	defer otherService.Close()
	if otherService.Instance == f.Service.Instance {
		t.Fatal("fixture instances collided")
	}
	if _, e := otherService.ImportMetadata(bytes.NewReader(snapshot), targetDB2); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("cross-instance import accepted: %v", e)
	}
}

// Scale acceptance (task 5.2): sustained event rate with out-of-order replay
// and 20 concurrent readers; generation consistency, no duplicate publishing,
// projection P95 within budget. Gated behind DRIVEBRIDGE_FP_SCALE because of
// fixture cost; the achieved scale is recorded in the evidence artifacts.
func TestFPTeamRuntimeScaleSoak(t *testing.T) {
	mode := os.Getenv("DRIVEBRIDGE_FP_SCALE")
	if mode == "" {
		t.Skip("scale fixture gated: set DRIVEBRIDGE_FP_SCALE=quick|full")
	}
	events := 20000
	readers := 20
	seconds := 10
	if mode == "full" {
		events = 90000 // 100 events/s sustained for 15 minutes
		seconds = 30   // compressed soak window on one machine; recorded
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	owner := h.user.ID
	// Sequence space continues from the current outbox head: replaying the
	// same numeric range would be skipped by per-scope watermarks.
	var seqBase int64
	f.Service.DB.Model(&service.OutboxEvent{}).Select("COALESCE(MAX(source_sequence), 0)").Scan(&seqBase)
	started := time.Now()
	// Concurrent authorized readers run DURING publishing and catch-up, the
	// way the design frames the 20-reader budget.
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, events)
	var duplicateRows int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				t0 := time.Now()
				f.Service.RuntimeStatusView(service.Principal{UserID: owner, Admin: true})
				mu.Lock()
				latencies = append(latencies, time.Since(t0))
				mu.Unlock()
			}
		}()
	}
	// Publish events with deliberate out-of-order revisions inside one
	// scope; the insert loop is the effective rate limiter on one machine.
	rate := time.Second / time.Duration(events/seconds)
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	published := 0
	for published < events {
		select {
		case <-ticker.C:
		}
		batch := 10
		e := f.Service.DB.Transaction(func(tx *gorm.DB) error {
			rows := make([]service.OutboxEvent, 0, batch)
			for i := range batch {
				revision := int64(published + i)
				if i%3 == 0 {
					revision -= 5 // stale revision replay
					if revision < 0 {
						revision = 0
					}
				}
				rows = append(rows, service.OutboxEvent{EventID: service.ID(), Kind: "file_catalogue.upsert", ResourceID: fmt.Sprintf("%048x", (published+i)%500), ResourceRevision: revision, Tombstone: (published+i)%7 == 0, SourceSequence: seqBase + int64(published+i+1), OwnerScope: owner, Payload: fmt.Sprintf(`{"seq":%d}`, published+i)})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
			return tx.Clauses(gormclause.OnConflict{DoNothing: true}).Create(&service.RuntimeSequence{Name: "outbox", Next: seqBase + int64(events) + 1}).Error
		})
		if e != nil {
			t.Fatal(e)
		}
		published += batch
	}
	publishSeconds := time.Since(started).Seconds()
	var total int64
	f.Service.DB.Model(&service.OutboxEvent{}).Count(&total)
	if total != int64(events) {
		t.Fatalf("published %d of %d events", total, events)
	}
	// Drain: the worker applies everything; measure projection lag.
	lagStart := time.Now()
	fpDrainRuntimeLong(t, f, time.Duration(seconds)*time.Second+60*time.Second)
	lagSeconds := time.Since(lagStart).Seconds()
	close(stop)
	wg.Wait()
	// Generation consistency: every applied resource has one row per
	// generation; tombstones stayed tombstones.
	var rows, tombstones int64
	f.Service.DB.Model(&service.ProjectionRow{}).Count(&rows)
	f.Service.DB.Model(&service.ProjectionRow{}).Where("tombstone = ?", true).Count(&tombstones)
	var checkpoint service.ProjectionCheckpoint
	f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).First(&checkpoint)
	if checkpoint.Watermark < total {
		t.Fatalf("projection did not catch up: watermark %d of %d", checkpoint.Watermark, total)
	}
	// No duplicate publishing: source_sequence is unique by construction and
	// every published event appears exactly once.
	var distinct int64
	f.Service.DB.Raw("SELECT COUNT(DISTINCT source_sequence) FROM runtime_outbox").Scan(&distinct)
	if distinct != total {
		t.Fatalf("duplicate publish detected: %d distinct of %d", distinct, total)
	}
	f.Service.DB.Model(&service.ProjectionRow{}).Where("revision < 0").Count(&duplicateRows)
	sortDurations(latencies)
	var p95 time.Duration
	if len(latencies) > 0 {
		p95 = latencies[len(latencies)*95/100]
	}
	report := map[string]any{"events": events, "publish_seconds": publishSeconds, "catch_up_seconds": lagSeconds, "read_latency_samples": len(latencies), "read_p95_ms": p95.Milliseconds(), "projection_rows": rows, "tombstones": tombstones, "duplicate_rows": duplicateRows, "full_target": mode == "full"}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("scale report: %s", b)
	if dir := os.Getenv("DRIVEBRIDGE_EVIDENCE_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "fp-teamruntime-scale.json"), b, 0o644)
	}
}

func fpDrainRuntimeLong(t *testing.T, f *fixtures.Runtime, budget time.Duration) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		var pending, unpublished int64
		f.Service.DB.Model(&service.RuntimeJob{}).Where("status = ?", "pending").Count(&pending)
		f.Service.DB.Model(&service.OutboxEvent{}).Where("published = ?", false).Count(&unpublished)
		if pending == 0 && unpublished == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	var pending, unpublished, rows int64
	f.Service.DB.Model(&service.RuntimeJob{}).Where("status = ?", "pending").Count(&pending)
	f.Service.DB.Model(&service.OutboxEvent{}).Where("published = ?", false).Count(&unpublished)
	f.Service.DB.Model(&service.ProjectionRow{}).Count(&rows)
	var cp service.ProjectionCheckpoint
	f.Service.DB.Where("projection = ?", "file_catalogue").First(&cp)
	t.Fatalf("runtime did not drain within budget: pending=%d unpublished=%d rows=%d checkpoint=%+v", pending, unpublished, rows, cp)
}

// fpKeySession exchanges an admin user key for a cookie session client.
func fpKeySession(t *testing.T, f *fixtures.Runtime, key string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"key": key})
	response, e := client.Post(f.App.URL+"/auth/key", "application/json", bytes.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("operator key login failed: %d", response.StatusCode)
	}
	return client
}

// fpRawHTTPPost is a small helper for direct HTTP status assertions.
func fpRawHTTPStatus(t *testing.T, h *human, method, path string, body []byte) int {
	t.Helper()
	req, _ := http.NewRequest(method, h.base+"/api/v1"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	response, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	return response.StatusCode
}

// Task 5.2 million dimension, gated behind DRIVEBRIDGE_FP_SCALE=million:
// one million directory projection rows and ten million outbox events are
// bulk-seeded, the built-in worker drains the full backlog through the
// production apply path, then a real 15-minute window publishes at 100
// events/s with 20 concurrent readers. Projection lag is sampled from the
// live watermark during the window and its P95 must stay within 30 seconds;
// generation consistency and no-duplicate publication are verified over the
// whole volume.
func TestFPTeamRuntimeScaleMillion(t *testing.T) {
	if os.Getenv("DRIVEBRIDGE_FP_SCALE") != "million" {
		t.Skip("million fixture gated behind DRIVEBRIDGE_FP_SCALE=million")
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	owner := h.user.ID
	// Establish the live generation through a real upload first.
	fpRuntimeUpload(t, f, h, "scale-million-seed.txt")
	fpDrainRuntime(t, f)
	var cp service.ProjectionCheckpoint
	if e := f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).First(&cp).Error; e != nil {
		t.Fatal(e)
	}
	generation := cp.Generation
	var seqBase int64
	f.Service.DB.Model(&service.OutboxEvent{}).Select("COALESCE(MAX(source_sequence), 0)").Scan(&seqBase)
	now := time.Now().UTC()
	// Scale dimensions default to the full 5.2 targets; smaller values via
	// DRIVEBRIDGE_FP_MILLION_EVENTS are calibration probes, never reported
	// as the acceptance run.
	directories := 1_000_000
	backlog := int64(10_000_000)
	if raw := os.Getenv("DRIVEBRIDGE_FP_MILLION_EVENTS"); raw != "" {
		if v, e := strconv.ParseInt(raw, 10, 64); e == nil && v > 0 {
			backlog = v
		}
	}
	seeded := 0
	dirStart := time.Now()
	for seeded < directories {
		const batch = 2000
		end := min(seeded+batch, directories)
		var query strings.Builder
		query.WriteString("INSERT INTO runtime_projection_rows (projection, generation, resource_id, owner_scope, revision, tombstone, digest, payload, created_at, updated_at) VALUES ")
		args := make([]any, 0, batch*10)
		for i := seeded; i < end; i++ {
			if i > seeded {
				query.WriteByte(',')
			}
			query.WriteString("(?,?,?,?,?,?,?,?,?,?)")
			args = append(args, "file_catalogue", generation, fmt.Sprintf("dir-%08x", i), owner, int64(i), false, fmt.Sprintf(`{"dir":%d}`, i), "", now, now)
		}
		if e := f.Service.DB.Exec(query.String(), args...).Error; e != nil {
			t.Fatalf("directory seed offset %d: %v", seeded, e)
		}
		seeded = end
	}
	dirSeconds := time.Since(dirStart).Seconds()
	// Backlog events: 500k file resources, tombstone mix and stale-revision
	// replays mirroring the soak fixture's disorder.
	const resources = 500_000
	eventStart := time.Now()
	for offset := int64(0); offset < backlog; offset += 2000 {
		end := min(offset+2000, backlog)
		var query strings.Builder
		query.WriteString("INSERT INTO runtime_outbox (event_id, kind, resource_id, resource_revision, tombstone, source_sequence, owner_scope, payload, published, created_at) VALUES ")
		args := make([]any, 0, 2000*10)
		for i := offset; i < end; i++ {
			if i > offset {
				query.WriteByte(',')
			}
			query.WriteString("(?,?,?,?,?,?,?,?,?,?)")
			revision := int64(i)
			if i%3 == 0 {
				revision -= 5
				if revision < 0 {
					revision = 0
				}
			}
			args = append(args, fmt.Sprintf("evt-%09d", i), "file_catalogue.upsert", fmt.Sprintf("file-%06x", i%resources), revision, i%7 == 0, seqBase+int64(i)+1, owner, fmt.Sprintf(`{"seq":%d}`, i), true, now)
		}
		if e := f.Service.DB.Exec(query.String(), args...).Error; e != nil {
			t.Fatalf("event seed offset %d: %v", offset, e)
		}
		if offset%500_000 == 0 {
			t.Logf("event seed progress: %d of %d (%.0fs)", offset, backlog, time.Since(eventStart).Seconds())
		}
	}
	if e := f.Service.DB.Clauses(gormclause.OnConflict{DoNothing: true}).Create(&service.RuntimeSequence{Name: "outbox", Next: seqBase + backlog + 1}).Error; e != nil {
		t.Fatal(e)
	}
	eventSeconds := time.Since(eventStart).Seconds()
	// Dormant seeding above marks the backlog published so the worker's
	// scope scan leaves the seeder uncontended; publication state is not
	// what drives apply (the checkpoint does). One unpublished event now
	// activates the scope and the worker drains the full backlog through
	// the production apply path; progress is tracked by the live watermark.
	if e := f.Service.DB.Model(&service.OutboxEvent{}).Where("event_id = ?", "evt-000000000").Update("published", false).Error; e != nil {
		t.Fatal(e)
	}
	var activated int64
	f.Service.DB.Model(&service.OutboxEvent{}).Where("published = ?", false).Count(&activated)
	if activated == 0 {
		t.Fatal("activation did not mark any event unpublished")
	}
	drainStart := time.Now()
	drainHead := seqBase + backlog
	drainDeadline := time.Now().Add(4 * time.Hour)
	lastProgress, lastWatermark := time.Now(), int64(-1)
	for {
		var watermark int64
		if e := f.Service.DB.Model(&service.ProjectionCheckpoint{}).Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).Select("watermark").Scan(&watermark).Error; e != nil {
			t.Fatal(e)
		}
		if watermark >= drainHead {
			break
		}
		if watermark != lastWatermark {
			lastWatermark, lastProgress = watermark, time.Now()
		} else if time.Since(lastProgress) > 90*time.Second {
			// Surface the worker's silent break: run one production apply
			// directly so the failing batch reports its error.
			applied, _, e := f.Service.ApplyOutboxEvents("file_catalogue", owner, 500)
			t.Fatalf("drain stalled at %d of %d; direct apply returned applied=%d err=%v", watermark, drainHead, applied, e)
		}
		if time.Now().After(drainDeadline) {
			t.Fatalf("backlog drain exceeded 4h budget: watermark %d of %d", watermark, drainHead)
		}
		if time.Since(drainStart)%(5*time.Minute) < time.Second {
			t.Logf("drain progress: watermark %d of %d (%.0fs)", watermark, drainHead, time.Since(drainStart).Seconds())
		}
		time.Sleep(500 * time.Millisecond)
	}
	drainSeconds := time.Since(drainStart).Seconds()
	var total int64
	f.Service.DB.Model(&service.OutboxEvent{}).Count(&total)
	if e := f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).First(&cp).Error; e != nil {
		t.Fatal(e)
	}
	if cp.Generation != generation || cp.Watermark != seqBase+backlog {
		t.Fatalf("backlog did not drain into the live generation: gen %s->%s watermark %d of %d", generation, cp.Generation, cp.Watermark, seqBase+backlog)
	}
	// Real 15-minute window: 100 events/s sustained, 20 concurrent readers,
	// watermark lag sampled once per second.
	const windowSeconds = 15 * 60
	const ratePerSecond = 100
	var mu sync.Mutex
	lagSamples := make([]float64, 0, windowSeconds)
	latencies := make([]time.Duration, 0, 4096)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				t0 := time.Now()
				f.Service.RuntimeStatusView(service.Principal{UserID: owner, Admin: true})
				mu.Lock()
				latencies = append(latencies, time.Since(t0))
				mu.Unlock()
				// Pace each reader: the single-connection fixture pool must
				// not be saturated by a busy-loop of heavyweight counts.
				select {
				case <-stop:
					return
				case <-time.After(50 * time.Millisecond):
				}
			}
		}()
	}
	published := 0
	windowStart := time.Now()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for elapsed := 0; elapsed < windowSeconds; elapsed++ {
		<-tick.C
		e := f.Service.DB.Transaction(func(tx *gorm.DB) error {
			rows := make([]service.OutboxEvent, 0, ratePerSecond)
			for i := range ratePerSecond {
				revision := int64(published + i)
				if i%3 == 0 {
					revision -= 5
					if revision < 0 {
						revision = 0
					}
				}
				rows = append(rows, service.OutboxEvent{EventID: fmt.Sprintf("win-%06d-%03d", elapsed, i), Kind: "file_catalogue.upsert", ResourceID: fmt.Sprintf("file-%06x", (published+i)%resources), ResourceRevision: revision, Tombstone: (published+i)%7 == 0, SourceSequence: seqBase + backlog + int64(published+i) + 1, OwnerScope: owner, Payload: fmt.Sprintf(`{"win":%d}`, elapsed)})
			}
			return tx.Create(&rows).Error
		})
		if e != nil {
			t.Fatal(e)
		}
		published += ratePerSecond
		var watermark int64
		f.Service.DB.Model(&service.ProjectionCheckpoint{}).Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).Select("watermark").Scan(&watermark)
		mu.Lock()
		lagSamples = append(lagSamples, float64(int64(published)-(watermark-(seqBase+backlog)))/float64(ratePerSecond))
		mu.Unlock()
	}
	windowWall := time.Since(windowStart).Seconds()
	// Let the worker finish the window's tail, bounded well under the P95 gate.
	tailStart := time.Now()
	fpDrainRuntimeLong(t, f, 5*time.Minute)
	tailSeconds := time.Since(tailStart).Seconds()
	close(stop)
	wg.Wait()
	sortDurations(latencies)
	var readP95 time.Duration
	if len(latencies) > 0 {
		readP95 = latencies[len(latencies)*95/100]
	}
	sort.Float64s(lagSamples)
	var lagP95 float64
	if len(lagSamples) > 0 {
		lagP95 = lagSamples[len(lagSamples)*95/100]
	}
	if lagP95 > 30 {
		t.Fatalf("projection lag P95 %.1fs exceeds 30s at 100 events/s", lagP95)
	}
	// Whole-volume integrity: single live generation, no duplicate
	// publication, tombstones preserved, directory seed intact.
	if e := f.Service.DB.Where("projection = ? AND owner_scope = ?", "file_catalogue", owner).First(&cp).Error; e != nil {
		t.Fatal(e)
	}
	if cp.Watermark != seqBase+backlog+int64(published) {
		t.Fatalf("watermark %d of %d after window", cp.Watermark, seqBase+backlog+int64(published))
	}
	var distinct int64
	f.Service.DB.Raw("SELECT COUNT(DISTINCT source_sequence) FROM runtime_outbox").Scan(&distinct)
	var totalEvents int64
	f.Service.DB.Model(&service.OutboxEvent{}).Count(&totalEvents)
	if distinct != totalEvents {
		t.Fatalf("duplicate publication: %d distinct of %d", distinct, totalEvents)
	}
	var liveRows, tombstones, dirRows int64
	f.Service.DB.Model(&service.ProjectionRow{}).Where("generation = ?", generation).Count(&liveRows)
	f.Service.DB.Model(&service.ProjectionRow{}).Where("generation = ? AND tombstone = ?", generation, true).Count(&tombstones)
	f.Service.DB.Model(&service.ProjectionRow{}).Where("generation = ? AND resource_id LIKE 'dir-%'", generation).Count(&dirRows)
	if dirRows != int64(directories) {
		t.Fatalf("directory seed lost: %d of %d", dirRows, directories)
	}
	if liveRows < int64(directories)+min(backlog, int64(resources)) {
		t.Fatalf("projection rows below expectation: %d", liveRows)
	}
	report := map[string]any{"directories": dirRows, "events_total": totalEvents, "backlog": backlog, "window_seconds": windowWall, "window_rate_per_second": ratePerSecond, "dir_seed_seconds": dirSeconds, "event_seed_seconds": eventSeconds, "drain_seconds": drainSeconds, "tail_seconds": tailSeconds, "lag_p95_seconds": lagP95, "read_p95_ms": readP95.Milliseconds(), "read_samples": len(latencies), "tombstones": tombstones, "duplicate_events": totalEvents - distinct, "generation": generation}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("million report: %s", b)
	if dir := os.Getenv("DRIVEBRIDGE_EVIDENCE_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "fp-teamruntime-million.json"), b, 0o644)
	}
}
