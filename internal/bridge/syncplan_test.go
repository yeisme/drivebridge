package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// planFixture pairs a local directory with an alias-backed remote directory and
// isolates the bisync journals per test, mirroring syncFixture.
func planFixture(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("plan matrix exercises the rclone bisync engine over local fs pairs")
	}
	local := t.TempDir()
	remoteDir := t.TempDir()
	const remote = "plan-fixture"
	config.Data().SetValue(remote, "type", "alias")
	config.Data().SetValue(remote, "remote", remoteDir)
	t.Cleanup(func() { config.Data().DeleteSection(remote) })
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := New(state, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.AddSpace(Space{ID: "paired", Remote: remote, Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	return s, local, remoteDir, filepath.Join(state, "bisync")
}

// flakyListFS fails the listing of one directory so a scan can be made
// partial deterministically. ListR is masked so the walker really goes
// through List and hits the failure instead of a recursive shortcut.
type flakyListFS struct {
	fs.Fs
	failPath string
}

func (f flakyListFS) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if dir == f.failPath {
		return nil, errors.New("listing unavailable")
	}
	return f.Fs.List(ctx, dir)
}

func (f flakyListFS) Features() *fs.Features {
	return f.Fs.Features().Disable("ListR")
}

// planApplyWait applies a plan and gives the engine lock a moment, matching
// the sync fixtures.
func planApplyWait(t *testing.T, s *Service, planID string, revision int64) (SyncPlanResult, error) {
	t.Helper()
	result, err := s.ApplySyncPlan(context.Background(), planID, ApplySyncPlanOptions{ExpectedRevision: revision})
	time.Sleep(50 * time.Millisecond)
	return result, err
}

// establishBaseline runs one resync plan apply so later plans classify against
// a real baseline. It returns the plan result of the apply.
func establishBaseline(t *testing.T, s *Service, workdir string) SyncPlanResult {
	t.Helper()
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: workdir})
	if err != nil {
		t.Fatalf("baseline plan: %v", err)
	}
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err != nil {
		t.Fatalf("baseline apply: %v", err)
	}
	return result
}

func mustLocalRoot(t *testing.T, s *Service) string {
	t.Helper()
	p, err := s.Space("paired")
	if err != nil {
		t.Fatal(err)
	}
	return p.LocalRoot
}

func mustRemoteRoot(t *testing.T) string {
	t.Helper()
	remote := config.GetValue("plan-fixture", "remote")
	if remote == "" {
		t.Fatal("plan-fixture remote missing")
	}
	return remote
}

func TestFPSyncPlanFirstPlanRequiresExplicitResync(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	syncWrite(t, local, "local-only.txt", "l", 0)
	syncWrite(t, remoteDir, "remote-only.txt", "r", 0)
	_, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err == nil || err.Error() != "resync_required" {
		t.Fatalf("first plan without resync: %v", err)
	}
	var count int64
	s.DB.Model(&SyncPlan{}).Count(&count)
	if count != 0 {
		t.Fatal("refused plan still froze rows")
	}
	if len(syncCount(t, local)) != 1 || len(syncCount(t, remoteDir)) != 1 {
		t.Fatal("refused plan changed either side")
	}
}

func TestFPSyncPlanFreezesWithStableIdentityAndPersists(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	syncWrite(t, local, "add-me.txt", "l", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Plan.ID == "" || plan.Plan.Phase != "planned" || plan.Plan.Revision != 1 {
		t.Fatalf("frozen plan header: %+v", plan.Plan)
	}
	if plan.Plan.SpaceID != "paired" || plan.Plan.Direction != "bidirectional" {
		t.Fatalf("plan identity: %+v", plan.Plan)
	}
	if plan.Plan.WriterHost == "" || plan.Plan.WriterHost == "single_designated_host" {
		t.Fatalf("writer identity must be concrete, got %q", plan.Plan.WriterHost)
	}
	if plan.Plan.MaxDelete != syncDefaultMaxDelete {
		t.Fatalf("default cap not frozen: %d", plan.Plan.MaxDelete)
	}
	var row SyncPlan
	if err = s.DB.First(&row, "id = ?", plan.Plan.ID).Error; err != nil {
		t.Fatal("plan row not persisted")
	}
	var baseline SyncBaseline
	if err = s.DB.First(&baseline, "id = ?", plan.Plan.BaselineID).Error; err != nil {
		t.Fatal("baseline generation not persisted")
	}
	if baseline.SpaceID != "paired" || baseline.At.IsZero() {
		t.Fatalf("baseline row: %+v", baseline)
	}
	var items int64
	s.DB.Model(&SyncItem{}).Where("plan_id = ?", plan.Plan.ID).Count(&items)
	if items == 0 || items != int64(plan.TotalItems) {
		t.Fatalf("items persisted=%d total=%d", items, plan.TotalItems)
	}
	// The freeze must not touch either side before apply.
	if _, err = os.Stat(filepath.Join(remoteDir, "add-me.txt")); err == nil {
		t.Fatal("planning wrote to the remote side")
	}
	// Old identities stay untouched: the space and its local root resolve as before.
	p, err := s.Space("paired")
	if err != nil || p.LocalRoot != local {
		t.Fatalf("space rebased by planning: %+v err=%v", p, err)
	}
}

func TestFPSyncPlanApplyExecutesFrozenSetAndKeepsOperation(t *testing.T) {
	s, local, _, wd := planFixture(t)
	syncWrite(t, local, "frozen.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.Phase != "completed" || result.Plan.OperationID == "" {
		t.Fatalf("apply result: %+v", result.Plan)
	}
	if b := mustRead(t, filepath.Join(local, "frozen.txt")); string(b) != "v" {
		t.Fatalf("local original changed: %q", b)
	}
	op, err := s.Get(result.Plan.OperationID)
	if err != nil || op.Status != "completed" || op.Kind != "sync" {
		t.Fatalf("original operation: %+v err=%v", op, err)
	}
	// The operation is keyed to the plan: one plan, one durable operation
	// identity across retries.
	var rows int64
	s.DB.Model(&Operation{}).Where("space_id = ?", "paired").Count(&rows)
	if rows != 1 {
		t.Fatalf("apply created extra operations: %d", rows)
	}
	// Applying a terminal plan again is an explicit refusal, never a replay.
	_, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err == nil || err.Error() != "plan_not_applicable" {
		t.Fatalf("replay of completed plan: %v", err)
	}
}

func TestFPSyncPlanRevisionGuardRejectsStaleConfirmation(t *testing.T) {
	s, _, _, wd := planFixture(t)
	syncWrite(t, mustLocalRoot(t, s), "guarded.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	_, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision+5)
	if err == nil || err.Error() != "version_conflict" {
		t.Fatalf("stale revision accepted: %v", err)
	}
	var row SyncPlan
	s.DB.First(&row, "id = ?", plan.Plan.ID)
	if row.Phase != "planned" || row.OperationID != "" {
		t.Fatalf("rejected revision still mutated the plan: %+v", row)
	}
	// The correct revision still applies afterwards.
	if _, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestFPSyncPlanSourceChangeAfterFreezeRefused(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	establishBaseline(t, s, wd)
	syncWrite(t, local, "pending.txt", "new-local", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range plan.Items {
		if item.Path == "pending.txt" && item.Class == "add_local" {
			found = true
		}
	}
	if !found {
		t.Fatalf("frozen items missing the addition: %+v", plan.Items)
	}
	// The remote side changes after confirmation.
	syncWrite(t, remoteDir, "remote-late.txt", "late", 0)
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err == nil || err.Error() != "plan_stale" {
		t.Fatalf("stale plan applied: %v", err)
	}
	if result.Plan.Phase != "stale" || result.Plan.StaleReason == "" {
		t.Fatalf("stale plan state: %+v", result.Plan)
	}
	// The original remote change survives untouched.
	if b := mustRead(t, filepath.Join(remoteDir, "remote-late.txt")); string(b) != "late" {
		t.Fatalf("remote original overwritten: %q", b)
	}
	// No operation was created for the refused apply.
	var rows int64
	s.DB.Model(&Operation{}).Where("kind = ?", "sync").Count(&rows)
	if rows != 1 { // the baseline apply only
		t.Fatalf("refused apply created operations: %d", rows)
	}
	// Recovery is an explicit rescan, and the new plan sees the late file.
	next, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	sawLate, sawPending := false, false
	for _, item := range next.Items {
		if item.Path == "remote-late.txt" && item.Class == "add_remote" {
			sawLate = true
		}
		if item.Path == "pending.txt" && item.Class == "add_local" {
			sawPending = true
		}
	}
	if !sawLate || !sawPending {
		t.Fatalf("regenerated plan lost changes: %+v", next.Items)
	}
}

func TestFPSyncPlanConflictKeepsBothCopiesWithSource(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	// A stable mirrored file keeps the engine's all-changed safety abort
	// out of the picture: only shared.txt differs.
	syncMirror(t, local, remoteDir, "stable.txt", "s", 2*time.Hour)
	syncMirror(t, local, remoteDir, "shared.txt", "v1", 2*time.Hour)
	establishBaseline(t, s, wd)
	syncWrite(t, local, "shared.txt", "local-edit", 1*time.Hour)
	syncWrite(t, remoteDir, "shared.txt", "remote-edit", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	conflicts := 0
	for _, item := range plan.Items {
		if item.Class == "conflict" && item.Path == "shared.txt" {
			conflicts++
		}
	}
	if conflicts != 1 || plan.Plan.Conflicts != 1 {
		t.Fatalf("conflict not previewed: %+v", plan)
	}
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.Phase != "completed" {
		t.Fatalf("conflict apply: %+v", result.Plan)
	}
	// Both edits survive, named by the original operation (the engine may
	// append its dedup counter to the suffix), sources visible.
	names := syncCount(t, local)
	opID := result.Plan.OperationID
	localKeep, remoteKeep := "", ""
	for name, content := range names {
		if strings.HasPrefix(name, "shared.txt.sync-"+opID+"-local") && content == "local-edit" {
			localKeep = name
		}
		if strings.HasPrefix(name, "shared.txt.sync-"+opID+"-remote") && content == "remote-edit" {
			remoteKeep = name
		}
	}
	if localKeep == "" || remoteKeep == "" {
		t.Fatalf("conflict copies not kept with sources: %v", names)
	}
}

func TestFPSyncPlanWriterHandoffRequiresExplicitConfirmation(t *testing.T) {
	s, _, _, wd := planFixture(t)
	syncWrite(t, mustLocalRoot(t, s), "handoff.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	// Another device's writer identity claims the plan; a backdated update
	// time must not turn into silent takeover.
	if err = s.DB.Model(&SyncPlan{}).Where("id = ?", plan.Plan.ID).Updates(map[string]any{"writer_host": "other-device", "updated_at": time.Now().UTC().Add(-2 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err == nil || err.Error() != "writer_handoff_required" {
		t.Fatalf("foreign writer applied the plan: %v", err)
	}
	hint := result.Recovery["writer_handoff_required"]
	if hint == "" || !strings.Contains(hint, "timeout") {
		t.Fatalf("hint must say a timeout proves nothing: %q", hint)
	}
	var row SyncPlan
	s.DB.First(&row, "id = ?", plan.Plan.ID)
	if row.Phase != "planned" || row.OperationID != "" {
		t.Fatalf("refused handoff mutated the plan: %+v", row)
	}
	// The explicit handoff is a new plan on this device rebuilding the
	// baseline; the old confirmation is never extended.
	fresh, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatalf("handoff plan: %v", err)
	}
	if fresh.Plan.ID == plan.Plan.ID {
		t.Fatal("handoff reused the stale plan identity")
	}
	if fresh.Plan.WriterHost == "other-device" {
		t.Fatal("handoff plan kept the foreign writer")
	}
}

func TestFPSyncPlanPartialScanRefusesDeletionWithCoverage(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	syncMirror(t, local, remoteDir, "keep.txt", "k", 2*time.Hour)
	syncMirror(t, local, remoteDir, "sub/one.txt", "1", 2*time.Hour)
	syncMirror(t, local, remoteDir, "sub/two.txt", "2", 2*time.Hour)
	establishBaseline(t, s, wd)
	// Remote loses one file; the local scan then fails on the subdirectory,
	// so a naive plan would read surviving local entries as deletions.
	if err := os.Remove(filepath.Join(remoteDir, "sub", "one.txt")); err != nil {
		t.Fatal(err)
	}
	s.syncPairOverride = func(ctx context.Context, p Space) (fs.Fs, fs.Fs, error) {
		localSide, err := fs.NewFs(ctx, filepath.ToSlash(p.LocalRoot))
		if err != nil {
			return nil, nil, err
		}
		provider, ok := s.providerFor(p).(remoteFsProvider)
		if !ok {
			return nil, nil, errors.New("sync_unsupported")
		}
		remoteSide, err := provider.Fs(ctx, p)
		if err != nil {
			return nil, nil, err
		}
		return localSide, flakyListFS{Fs: remoteSide, failPath: "sub"}, nil
	}
	result, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err == nil || err.Error() != "sync_scan_incomplete" {
		t.Fatalf("partial scan planned: %v", err)
	}
	if result.Coverage == nil || result.Coverage.ScanErrors == 0 || result.Coverage.LocalEntries+result.Coverage.RemoteEntries == 0 {
		t.Fatalf("coverage not reported: %+v", result.Coverage)
	}
	var count int64
	s.DB.Model(&SyncPlan{}).Count(&count)
	if count != 1 { // baseline plan only
		t.Fatalf("partial scan froze a plan: %d", count)
	}
	// The surviving local file was not deleted by propagation.
	if _, err = os.Stat(filepath.Join(local, "sub", "two.txt")); err != nil {
		t.Fatal("partial scan propagated a deletion")
	}
}

func TestFPSyncPlanInterruptedRecoversThroughOriginalOperation(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	// A mirrored file keeps the engine's prior listing non-empty so the
	// resumed (non-resync) apply runs the real path.
	syncMirror(t, local, remoteDir, "base.txt", "b", 2*time.Hour)
	establishBaseline(t, s, wd)
	syncWrite(t, local, "interrupted.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a process dying mid-apply: the plan is applying and the
	// operation row exists with its plan-scoped key.
	dyingOp := Operation{ID: newID(), SpaceID: "paired", Kind: "sync", Key: operationKey("paired", "syncplan", plan.Plan.ID), Status: "transferring", Updated: time.Now().UTC()}
	if err = s.DB.Create(&dyingOp).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Model(&SyncPlan{}).Where("id = ?", plan.Plan.ID).Updates(map[string]any{"phase": "applying", "operation_id": dyingOp.ID}).Error; err != nil {
		t.Fatal(err)
	}
	state := s.state
	s.Close()
	s2, err := New(state, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	var reopened SyncPlan
	if err = s2.DB.First(&reopened, "id = ?", plan.Plan.ID).Error; err != nil {
		t.Fatal("plan lost across restart")
	}
	if reopened.Phase != "interrupted" {
		t.Fatalf("applying plan not marked interrupted on restart: %s", reopened.Phase)
	}
	// Nothing changed since the freeze: the recovery reuses the original
	// operation identity and completes.
	result, err := planApplyWait(t, s2, plan.Plan.ID, plan.Plan.Revision)
	if err != nil {
		t.Fatalf("resume after interruption: %v", err)
	}
	if result.Plan.OperationID != dyingOp.ID || result.Plan.Phase != "completed" {
		t.Fatalf("resume changed identity: %+v", result.Plan)
	}
	var ops []Operation
	s2.DB.Where("space_id = ?", "paired").Order("id").Find(&ops)
	if len(ops) != 2 { // baseline apply + this one, rows reused not duplicated
		t.Fatalf("resume duplicated operations: %d", len(ops))
	}
	if b := mustRead(t, filepath.Join(local, "interrupted.txt")); string(b) != "v" {
		t.Fatalf("local original changed: %q", b)
	}
}

func TestFPSyncPlanPartialApplyLandsStaleWithOriginalReadable(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	syncMirror(t, local, remoteDir, "base.txt", "b", 2*time.Hour)
	establishBaseline(t, s, wd)
	syncWrite(t, mustLocalRoot(t, s), "partial.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	dyingOp := Operation{ID: newID(), SpaceID: "paired", Kind: "sync", Key: operationKey("paired", "syncplan", plan.Plan.ID), Status: "transferring", Updated: time.Now().UTC()}
	if err = s.DB.Create(&dyingOp).Error; err != nil {
		t.Fatal(err)
	}
	// Post-restart state: the plan died applying and came back interrupted.
	if err = s.DB.Model(&SyncPlan{}).Where("id = ?", plan.Plan.ID).Updates(map[string]any{"phase": "interrupted", "operation_id": dyingOp.ID}).Error; err != nil {
		t.Fatal(err)
	}
	s.DB.Model(&Operation{}).Where("id = ?", dyingOp.ID).Updates(map[string]any{"status": "interrupted", "error": "resume_required"})
	// The dead engine half-landed its effect on the remote side.
	syncWrite(t, mustRemoteRoot(t), "partial.txt", "v", 0)
	result, err := planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err == nil || err.Error() != "plan_stale" {
		t.Fatalf("partial apply extended the old confirmation: %v", err)
	}
	if result.Plan.Phase != "stale" {
		t.Fatalf("partial apply state: %+v", result.Plan)
	}
	// The original operation stays readable for result recovery.
	op, err := s.Get(dyingOp.ID)
	if err != nil || op.Status != "interrupted" {
		t.Fatalf("original operation unreadable: %+v err=%v", op, err)
	}
	if result.Recovery["plan_stale"] == "" || !strings.Contains(result.Recovery["plan_stale"], dyingOp.ID) {
		t.Fatalf("stale recovery must point at the original operation: %+v", result.Recovery)
	}
}

func TestFPSyncPlanDeletionCapNeedsExplicitChoice(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	for i, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		syncMirror(t, local, remoteDir, name, string(rune('a'+i)), 2*time.Hour)
	}
	establishBaseline(t, s, wd)
	// Two of four remote files vanish: 50% deletions.
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.Remove(filepath.Join(remoteDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err == nil || err.Error() != "sync_deletion_cap_exceeded" {
		t.Fatalf("over-cap plan frozen: %v", err)
	}
	// The user explicitly raises the cap for a new plan.
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{MaxDelete: 60, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Plan.MaxDelete != 60 {
		t.Fatalf("explicit cap not frozen: %d", plan.Plan.MaxDelete)
	}
	deletions := 0
	for _, item := range plan.Items {
		if item.Class == "delete_local" || item.Class == "delete_remote" {
			deletions++
		}
	}
	if deletions != 2 {
		t.Fatalf("expected two deletions, items: %+v", plan.Items)
	}
	if _, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(local, "a.txt")); err == nil {
		t.Fatal("explicit deletions did not execute")
	}
}

func TestFPSyncPlanSpaceRevocationRefusesApply(t *testing.T) {
	s, _, _, wd := planFixture(t)
	syncWrite(t, mustLocalRoot(t, s), "revoked.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	// Permission revocation: the space disappears from the allowed set.
	if err = s.DB.Where("id = ?", "paired").Delete(&Space{}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision)
	if err == nil || err.Error() != "space_not_allowed" {
		t.Fatalf("revoked space applied: %v", err)
	}
	var row SyncPlan
	s.DB.First(&row, "id = ?", plan.Plan.ID)
	if row.Phase != "planned" {
		t.Fatalf("revocation mutated the plan: %+v", row)
	}
}

func TestFPSyncPlanStatusPaginatesKeyset(t *testing.T) {
	s, local, _, wd := planFixture(t)
	for i := range 7 {
		syncWrite(t, local, "page-"+string(rune('a'+i))+".txt", "v", 0)
	}
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalItems != 7 {
		t.Fatalf("total items: %d", plan.TotalItems)
	}
	// Default page is 50; an explicit bound above the maximum is clamped.
	status, err := s.SyncPlanGet(plan.Plan.ID, 0, 0)
	if err != nil || len(status.Items) != 7 {
		t.Fatalf("default page: %d err=%v", len(status.Items), err)
	}
	status, err = s.SyncPlanGet(plan.Plan.ID, 0, 500)
	if err != nil || len(status.Items) != 7 {
		t.Fatalf("clamped page: %d err=%v", len(status.Items), err)
	}
	// Keyset walk with limit 3 covers every item exactly once, ordered by ID.
	seen := map[string]bool{}
	after := uint64(0)
	pages := 0
	for {
		page, err := s.SyncPlanGet(plan.Plan.ID, after, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 0 {
			break
		}
		pages++
		if page.Items[0].ID <= after {
			t.Fatal("keyset cursor regressed")
		}
		for _, item := range page.Items {
			if seen[item.Path] {
				t.Fatalf("duplicate item %s", item.Path)
			}
			seen[item.Path] = true
			after = item.ID
		}
	}
	if len(seen) != 7 || pages != 3 {
		t.Fatalf("keyset walk: seen=%d pages=%d", len(seen), pages)
	}
	// Unknown plans are an explicit refusal.
	if _, err = s.SyncPlanGet("missing-plan", 0, 0); err == nil || err.Error() != "plan_not_found" {
		t.Fatalf("unknown plan: %v", err)
	}
}

func TestFPSyncPlanSurfacesShareOneApplicationContract(t *testing.T) {
	s, local, _, wd := planFixture(t)
	syncWrite(t, local, "surface.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	// MCP surface: read-only plan read exists; apply is never an agent tool.
	tools := s.Tools()
	names := map[string]bool{}
	readOnly := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name] = true
		if tl.Annotations.ReadOnlyHint != nil && *tl.Annotations.ReadOnlyHint {
			readOnly[tl.Name] = true
		}
	}
	if !names["sync_plan_get"] || !names["retention_preview"] {
		t.Fatalf("read-only tools missing: %v", names)
	}
	if !readOnly["sync_plan_get"] || !readOnly["retention_preview"] {
		t.Fatal("plan/retention tools not marked read-only")
	}
	for _, forbidden := range []string{"sync_plan_apply", "sync_apply", "sync_plan"} {
		if names[forbidden] {
			t.Fatalf("agent surface gained %q", forbidden)
		}
	}
	if !names["list"] || !names["operation_get"] || !names["mount_status"] {
		t.Fatal("old tool set shrunk")
	}
	// MCP call and the application service return the same plan identity.
	viaMCP, err := s.Call(context.Background(), "sync_plan_get", map[string]any{"plan_id": plan.Plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(viaMCP)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), plan.Plan.ID) || !strings.Contains(string(encoded), `"phase":"planned"`) {
		t.Fatalf("mcp plan projection: %s", encoded)
	}
	// The local owner socket exposes the same read-only actions.
	viaSocket, err := s.RunLocalFileAction(context.Background(), LocalFileAction{Action: "sync_plan_get", PlanID: plan.Plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	if via, ok := viaSocket.(SyncPlanResult); !ok || via.Plan.ID != plan.Plan.ID {
		t.Fatalf("socket plan projection: %+v", viaSocket)
	}
	if _, err = s.RunLocalFileAction(context.Background(), LocalFileAction{Action: "retention_preview", Space: "paired"}); err != nil {
		t.Fatalf("socket retention preview: %v", err)
	}
	// Old socket actions keep working.
	if _, err = s.RunLocalFileAction(context.Background(), LocalFileAction{Action: "operations", Space: "paired"}); err != nil {
		t.Fatalf("old socket action: %v", err)
	}
	// Write actions on the socket stay limited to the existing set.
	if _, err = s.RunLocalFileAction(context.Background(), LocalFileAction{Action: "sync_apply", PlanID: plan.Plan.ID}); err == nil || err.Error() != "unsupported" {
		t.Fatalf("socket gained an apply action: %v", err)
	}
}

func TestFPSyncPlanRetentionPreviewClassifiesAndProtects(t *testing.T) {
	s, local, _, wd := planFixture(t)
	establishBaseline(t, s, wd)
	// A trashed reference enters the local trash class (real upload flow).
	src := filepath.Join(local, "trashme.bin")
	if err := os.WriteFile(src, []byte("trash-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	upload, err := s.Upload(context.Background(), "paired", "", src, "trashme.bin", "retention-trash")
	if err != nil {
		t.Fatal(err)
	}
	upload = waitOp(t, s, upload.ID)
	if upload.Status != "completed" {
		t.Fatalf("retention upload: %s %s", upload.Status, upload.Error)
	}
	if _, err := s.Trash(context.Background(), upload.FileRef); err != nil {
		t.Fatal(err)
	}
	// Browser staging payload.
	if err := os.MkdirAll(filepath.Join(local, ".drivebridge-staging"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, ".drivebridge-staging", "pending-upload"), []byte("staged"), 0600); err != nil {
		t.Fatal(err)
	}
	// rclone VFS cache with unflushed upload bytes for this remote.
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	vfsDir := filepath.Join(cacheHome, "rclone", "vfs", "plan-fixture", "sub")
	if err := os.MkdirAll(filepath.Join(vfsDir, "upload"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vfsDir, "upload", "chunk.dat"), []byte("unflushed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vfsDir, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vfsDir, "data", "block.dat"), []byte("rebuildable"), 0600); err != nil {
		t.Fatal(err)
	}
	preview, err := s.RetentionPreview(context.Background(), "paired")
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]RetentionClass{}
	for _, class := range preview.Classes {
		classes[class.Class] = class
	}
	if c := classes["staging"]; c.Files != 1 || c.Bytes != int64(len("staged")) {
		t.Fatalf("staging class: %+v", c)
	}
	if c := classes["local_trash"]; c.Files != 1 || c.Bytes != upload.Bytes {
		t.Fatalf("trash class: %+v upload=%+v", c, upload)
	}
	pending, ok := classes["vfs_pending_upload"]
	if !ok || pending.Bytes != int64(len("unflushed")) || pending.Protected != true || pending.Cleanable != false {
		t.Fatalf("pending VFS bytes must be protected: %+v", pending)
	}
	if c := classes["vfs_cache_rebuildable"]; c.Bytes != int64(len("rebuildable")) || c.Cleanable != true {
		t.Fatalf("rebuildable cache class: %+v", c)
	}
	if c := classes["remote"]; c.Cleanable != false {
		t.Fatalf("remote class must not be locally cleanable: %+v", c)
	}
	if preview.MountAliveNote == "" || !strings.Contains(preview.MountAliveNote, "persist") {
		t.Fatalf("mount-alive note missing: %q", preview.MountAliveNote)
	}
	b, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "credential=ephemeral") || strings.Contains(string(b), cacheHome) {
		t.Fatal("retention preview leaked sensitive values")
	}
}

func TestFPSyncPlanOldContractUnchangedAlongsidePlans(t *testing.T) {
	s, local, remoteDir, wd := planFixture(t)
	syncMirror(t, local, remoteDir, "old.txt", "v1", 2*time.Hour)
	// The old one-shot surface keeps its exact flags and behavior.
	report, err := syncWait(t, s, "paired", SyncOptions{Resync: true, Workdir: wd})
	if err != nil || report.Status != "completed" || report.Resync != true || report.Baseline != false {
		t.Fatalf("legacy sync: %+v err=%v", report, err)
	}
	// Plans coexist with the legacy snapshot without disturbing it.
	syncWrite(t, local, "coexist.txt", "v", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = planApplyWait(t, s, plan.Plan.ID, plan.Plan.Revision); err != nil {
		t.Fatal(err)
	}
	// The legacy surface still runs after plan activity.
	report, err = syncWait(t, s, "paired", SyncOptions{Workdir: wd})
	if err != nil || report.Status != "completed" {
		t.Fatalf("legacy sync after plans: %+v err=%v", report, err)
	}
	if report.MaxDelete != syncDefaultMaxDelete {
		t.Fatalf("legacy default cap changed: %d", report.MaxDelete)
	}
}

func TestFPSyncPlanScaleFreezesTenThousandItems(t *testing.T) {
	if testing.Short() {
		t.Skip("scale fixture writes 10000 small files")
	}
	s, local, _, wd := planFixture(t)
	const total = 10000
	stamp := time.Now().Add(-2 * time.Hour)
	for i := range total {
		dir := filepath.Join(local, "batch", itoa(i%50), itoa(i/50))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "file-"+itoa(i)+".txt")
		if err := os.WriteFile(path, []byte("v"), 0600); err != nil {
			t.Fatal(err)
		}
		fileStamp := stamp.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(path, fileStamp, fileStamp); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: wd})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalItems != total {
		t.Fatalf("scale items: %d", plan.TotalItems)
	}
	if plan.Plan.LocalCount != total || plan.Plan.RemoteCount != 0 {
		t.Fatalf("scale counts: local=%d remote=%d", plan.Plan.LocalCount, plan.Plan.RemoteCount)
	}
	if plan.Plan.LocalDigest == "" || plan.Plan.RemoteDigest == "" {
		t.Fatal("scan digests missing")
	}
	// Keyset pagination walks all 10000 items in bounded pages.
	seen := 0
	after := uint64(0)
	for {
		page, err := s.SyncPlanGet(plan.Plan.ID, after, 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 0 {
			break
		}
		if len(page.Items) > 200 {
			t.Fatalf("page over the maximum: %d", len(page.Items))
		}
		seen += len(page.Items)
		after = page.Items[len(page.Items)-1].ID
	}
	if seen != total {
		t.Fatalf("scale pagination: %d", seen)
	}
	var items int64
	s.DB.Model(&SyncItem{}).Where("plan_id = ?", plan.Plan.ID).Count(&items)
	if items != total {
		t.Fatalf("persisted items: %d", items)
	}
	t.Logf("10000-item plan frozen and paged in %s", time.Since(started))
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	p := len(b)
	for v > 0 {
		p--
		b[p] = byte('0' + v%10)
		v /= 10
	}
	return string(b[p:])
}
