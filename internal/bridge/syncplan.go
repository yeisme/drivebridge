package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd/bisync"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"gorm.io/gorm"
)

// Sync plan phases describe the frozen-intent lifecycle; they never replace
// the legacy Operation.status enum.
const (
	planPhasePlanned     = "planned"
	planPhaseStale       = "stale"
	planPhaseApplying    = "applying"
	planPhaseCompleted   = "completed"
	planPhaseInterrupted = "interrupted"
	planPhaseFailed      = "failed"
)

// Item classes mirror the four-class sync plan plus the conflict class.
const (
	itemAddLocal     = "add_local"
	itemAddRemote    = "add_remote"
	itemUpdateL2R    = "update_local_to_remote"
	itemUpdateR2L    = "update_remote_to_local"
	itemDeleteLocal  = "delete_local"
	itemDeleteRemote = "delete_remote"
	itemConflict     = "conflict"
)

// SyncPlan freezes one reviewed sync intent: both root identities, the
// baseline generation it was computed against, the writer that owns the
// apply, the scan digests, the deletion cap and the per-path item set. The
// plan is immutable once frozen; changing the world invalidates it.
type SyncPlan struct {
	ID          string `json:"plan_id" gorm:"primaryKey"`
	SpaceID     string `json:"space_id" gorm:"index:idx_syncplan_space"`
	Phase       string `json:"phase"`
	Direction   string `json:"direction"`
	Revision    int64  `json:"revision"`
	BaselineID  string `json:"baseline_generation"`
	Resync      bool   `json:"resync"`
	LocalRootID string `json:"-"` // absolute root identity stays in the local control plane
	RemoteIdent string `json:"remote_root_identity"`
	WriterHost  string `json:"writer_host"`
	MaxDelete   int    `json:"max_delete_percent"`
	// Scan summary frozen at planning time.
	ScanComplete bool   `json:"scan_complete"`
	ScanErrors   int    `json:"scan_errors"`
	LocalCount   int    `json:"local_entries"`
	RemoteCount  int    `json:"remote_entries"`
	LocalDigest  string `json:"-"`
	RemoteDigest string `json:"-"`
	// BaselineSnapshotDigest fingerprints the engine snapshot file at freeze
	// time so an interleaved sync run is detected at apply.
	BaselineSnapshotDigest string    `json:"-"`
	Workdir                string    `json:"-"`
	DeleteRatioLocal       int       `json:"delete_ratio_local"`
	DeleteRatioRemote      int       `json:"delete_ratio_remote"`
	Conflicts              int       `json:"conflicts"`
	OperationID            string    `json:"operation_id,omitempty"`
	StaleReason            string    `json:"stale_reason,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// SyncBaseline is one frozen baseline generation projection. The engine
// journal snapshot remains the source of truth; this row records which
// generation a plan was computed against so it can be rebuilt or audited
// without re-reading engine internals.
type SyncBaseline struct {
	ID             string    `json:"baseline_id" gorm:"primaryKey"`
	SpaceID        string    `json:"space_id" gorm:"index:idx_syncbaseline_space"`
	PlanID         string    `json:"plan_id" gorm:"index"`
	Source         string    `json:"source"` // snapshot-file | resync-scan
	SnapshotDigest string    `json:"snapshot_digest,omitempty"`
	LocalDigest    string    `json:"-"`
	RemoteDigest   string    `json:"-"`
	LocalCount     int       `json:"local_entries"`
	RemoteCount    int       `json:"remote_entries"`
	At             time.Time `json:"at"`
}

// SyncItem is one frozen per-path action inside a plan. IDs are keyset
// cursors for plan status paging.
type SyncItem struct {
	ID     uint64 `json:"id" gorm:"primaryKey"`
	PlanID string `json:"plan_id" gorm:"index:idx_syncitem_plan"`
	Path   string `json:"path"`
	Class  string `json:"class"`
}

// PlanSyncOptions controls plan freezing. Resync mirrors the legacy surface:
// the first plan needs it and must preview same-name differences honestly.
type PlanSyncOptions struct {
	Resync    bool
	MaxDelete int
	Workdir   string
}

// ApplySyncPlanOptions carries the user's confirmed revision. An apply never
// extends a confirmation made against a different revision.
type ApplySyncPlanOptions struct {
	ExpectedRevision int64
}

// ScanCoverage reports how complete the observations behind a decision were.
type ScanCoverage struct {
	LocalEntries  int `json:"local_entries"`
	RemoteEntries int `json:"remote_entries"`
	ScanErrors    int `json:"scan_errors"`
}

// SyncPlanResult is the shared projection for plan, apply and status: every
// surface (CLI, MCP, local socket) renders this same application result.
type SyncPlanResult struct {
	Plan          SyncPlan          `json:"plan"`
	Items         []SyncItem        `json:"items"`
	NextAfter     uint64            `json:"next_after,omitempty"`
	TotalItems    int64             `json:"total_items"`
	Operation     *Operation        `json:"operation,omitempty"`
	Coverage      *ScanCoverage     `json:"coverage,omitempty"`
	RefusalReason string            `json:"refusal_reason,omitempty"`
	Recovery      map[string]string `json:"recovery,omitempty"`
}

// planPageSizeDefault and planPageSizeMax fix plan item paging: default 50,
// maximum 200, keyset by item ID.
const (
	planPageSizeDefault = 50
	planPageSizeMax     = 200
)

// writerIdentity names the single designated local writer. It deliberately
// proves nothing about other machines: the process lock underneath protects
// one host only, so a second device must hand over explicitly.
func (s *Service) writerIdentity() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown-host"
	}
	sum := sha256.Sum256([]byte(filepath.Join(s.state, "state.sqlite")))
	return host + "/" + hex.EncodeToString(sum[:8])
}

// remoteIdent freezes the remote root identity of a space.
func remoteIdent(p Space) string {
	return p.Kind() + "|" + p.Remote + "|" + p.DriveID + "|" + p.RootID
}

// syncFsPair builds the local/remote fs pair for the sync engine. Tests may
// wrap one side through syncPairOverride to inject listing failures.
func (s *Service) syncFsPair(ctx context.Context, p Space) (fs.Fs, fs.Fs, error) {
	if s.syncPairOverride != nil {
		return s.syncPairOverride(ctx, p)
	}
	if info, e := os.Stat(p.LocalRoot); e != nil || !info.IsDir() {
		return nil, nil, errors.New("local_root_unavailable")
	}
	local, err := fs.NewFs(ctx, filepath.ToSlash(p.LocalRoot))
	if err != nil {
		return nil, nil, errors.New("local_root_unavailable")
	}
	provider, ok := s.providerFor(p).(remoteFsProvider)
	if !ok {
		return nil, nil, errors.New("sync_unsupported")
	}
	remote, err := provider.Fs(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	return local, remote, nil
}

// planWalkSide walks one side and counts listing errors: a partial scan can
// never masquerade as a complete observation.
func planWalkSide(ctx context.Context, f fs.Fs) (map[string]syncEntry, int) {
	out := map[string]syncEntry{}
	errs := 0
	// A non-nil walk return is itself a partial observation: count it so
	// ScanErrors reflects every incompleteness, not only per-entry errors.
	if walk.Walk(ctx, f, "", false, -1, func(path string, entries fs.DirEntries, err error) error {
		if err != nil {
			errs++
			return nil
		}
		for _, entry := range entries {
			if o, ok := entry.(fs.Object); ok {
				out[strings.TrimPrefix(o.Remote(), "/")] = syncEntry{Size: o.Size(), ModTime: o.ModTime(ctx)}
			}
		}
		return nil
	}) != nil {
		errs++
	}
	return out, errs
}

// scanDigest fingerprints one side observation: path, size and modtime in
// path order. Two digests match only when both sides are byte-for-byte the
// same world the plan froze.
func scanDigest(side map[string]syncEntry) string {
	paths := make([]string, 0, len(side))
	for path := range side {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		e := side[path]
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, e.Size, e.ModTime.UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fileDigest fingerprints a file's bytes; missing files digest to "".
func fileDigest(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func deletionRatio(baseline, current map[string]syncEntry) int {
	if len(baseline) == 0 {
		return 0
	}
	deleted := 0
	for path := range baseline {
		if _, ok := current[path]; !ok {
			deleted++
		}
	}
	return 100 * deleted / len(baseline)
}

func itemClass(list []string, class string) []SyncItem {
	out := make([]SyncItem, 0, len(list))
	for _, path := range list {
		out = append(out, SyncItem{Path: path, Class: class})
	}
	return out
}

// vanishedSinceBaseline returns the deletions a run would propagate: paths
// present in one side's baseline, missing from that side now and still
// present on the other side. Without a baseline nothing is a deletion.
func vanishedSinceBaseline(snapshot syncSnapshot, haveSnapshot bool, side1, side2 map[string]syncEntry) (remoteDeleted, localDeleted []string) {
	if !haveSnapshot {
		return nil, nil
	}
	for path := range snapshot.Remote {
		if _, gone := side2[path]; !gone {
			if _, kept := side1[path]; kept {
				remoteDeleted = append(remoteDeleted, path)
			}
		}
	}
	for path := range snapshot.Local {
		if _, gone := side1[path]; !gone {
			if _, kept := side2[path]; kept {
				localDeleted = append(localDeleted, path)
			}
		}
	}
	sort.Strings(remoteDeleted)
	sort.Strings(localDeleted)
	return remoteDeleted, localDeleted
}

// PlanSync scans both sides and freezes the four-class plan plus conflicts
// for explicit review. Planning never writes to either side.
func (s *Service) PlanSync(ctx context.Context, space string, opt PlanSyncOptions) (SyncPlanResult, error) {
	if !s.Enabled {
		return SyncPlanResult{}, errors.New("disabled")
	}
	p, err := s.Space(space)
	if err != nil {
		return SyncPlanResult{}, err
	}
	local, remote, err := s.syncFsPair(ctx, p)
	if err != nil {
		return SyncPlanResult{}, err
	}
	workdir := opt.Workdir
	if workdir == "" {
		workdir = bisync.DefaultWorkdir
	}
	basePath := bilib.BasePath(ctx, workdir, local, remote)
	snapshotPath := basePath + ".drivebridge.json"
	snapshot, haveSnapshot := loadSyncSnapshot(snapshotPath)
	if !opt.Resync {
		if _, e := os.Stat(basePath + ".path1.lst"); e != nil {
			return SyncPlanResult{}, errors.New("resync_required")
		}
		if e := trustSyncJournal(basePath + ".path1.lst"); e != nil {
			return SyncPlanResult{}, e
		}
		if e := trustSyncJournal(basePath + ".path2.lst"); e != nil {
			return SyncPlanResult{}, e
		}
	}
	maxDelete := opt.MaxDelete
	if maxDelete <= 0 {
		maxDelete = syncDefaultMaxDelete
	}
	side1, errs1 := planWalkSide(ctx, local)
	side2, errs2 := planWalkSide(ctx, remote)
	report := SyncReport{}
	classifySync(snapshot, haveSnapshot, side1, side2, &report)
	sortSyncReport(&report)
	// Deletions are measured directly against the baseline, not from the
	// preview labels: a mirrored file that vanished from one side is a
	// deletion to propagate, whatever the four-class preview calls it.
	remoteDeleted, localDeleted := vanishedSinceBaseline(snapshot, haveSnapshot && !opt.Resync, side1, side2)
	coverage := &ScanCoverage{LocalEntries: len(side1), RemoteEntries: len(side2), ScanErrors: errs1 + errs2}
	// A partial scan is not deletion evidence: when any listing failed and
	// the plan would propagate deletions, refuse with the coverage visible.
	if coverage.ScanErrors > 0 && len(remoteDeleted)+len(localDeleted) > 0 {
		return SyncPlanResult{Coverage: coverage}, errors.New("sync_scan_incomplete")
	}
	if haveSnapshot && !opt.Resync {
		if reason, e := checkDeletionCap(snapshot.Local, side1, "local", maxDelete); e != nil {
			return SyncPlanResult{Coverage: coverage, RefusalReason: reason}, e
		}
		if reason, e := checkDeletionCap(snapshot.Remote, side2, "remote", maxDelete); e != nil {
			return SyncPlanResult{Coverage: coverage, RefusalReason: reason}, e
		}
	}
	now := time.Now().UTC()
	plan := SyncPlan{
		ID:                     newID(),
		SpaceID:                space,
		Phase:                  planPhasePlanned,
		Direction:              "bidirectional",
		Revision:               1,
		Resync:                 opt.Resync,
		LocalRootID:            p.LocalRoot,
		RemoteIdent:            remoteIdent(p),
		WriterHost:             s.writerIdentity(),
		MaxDelete:              maxDelete,
		ScanComplete:           coverage.ScanErrors == 0,
		ScanErrors:             coverage.ScanErrors,
		LocalCount:             len(side1),
		RemoteCount:            len(side2),
		LocalDigest:            scanDigest(side1),
		RemoteDigest:           scanDigest(side2),
		BaselineSnapshotDigest: fileDigest(snapshotPath),
		Workdir:                workdir,
		DeleteRatioLocal:       deletionRatio(snapshot.Local, side1),
		DeleteRatioRemote:      deletionRatio(snapshot.Remote, side2),
		Conflicts:              len(report.Conflicts),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	// Local additions exclude paths that are really remote deletions: the
	// engine propagates the deletion instead of re-uploading the local copy.
	deletedRemote := map[string]bool{}
	for _, path := range remoteDeleted {
		deletedRemote[path] = true
	}
	genuineAdditions1 := make([]string, 0, len(report.Additions1))
	for _, path := range report.Additions1 {
		if !deletedRemote[path] {
			genuineAdditions1 = append(genuineAdditions1, path)
		}
	}
	items := append(itemClass(genuineAdditions1, itemAddLocal), itemClass(report.Additions2, itemAddRemote)...)
	items = append(items, itemClass(report.Updates1, itemUpdateL2R)...)
	items = append(items, itemClass(report.Updates2, itemUpdateR2L)...)
	items = append(items, itemClass(remoteDeleted, itemDeleteRemote)...)
	items = append(items, itemClass(localDeleted, itemDeleteLocal)...)
	items = append(items, itemClass(report.Conflicts, itemConflict)...)
	for i := range items {
		items[i].PlanID = plan.ID
	}
	source := "resync-scan"
	if haveSnapshot {
		source = "snapshot-file"
	}
	baseline := SyncBaseline{
		ID:             newID(),
		SpaceID:        space,
		PlanID:         plan.ID,
		Source:         source,
		SnapshotDigest: plan.BaselineSnapshotDigest,
		LocalDigest:    plan.LocalDigest,
		RemoteDigest:   plan.RemoteDigest,
		LocalCount:     plan.LocalCount,
		RemoteCount:    plan.RemoteCount,
		At:             now,
	}
	plan.BaselineID = baseline.ID
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&plan).Error; e != nil {
			return errors.New("state_unavailable")
		}
		if e := tx.Create(&baseline).Error; e != nil {
			return errors.New("state_unavailable")
		}
		if len(items) > 0 {
			if e := tx.Create(&items).Error; e != nil {
				return errors.New("state_unavailable")
			}
		}
		return nil
	})
	if err != nil {
		return SyncPlanResult{}, err
	}
	return s.SyncPlanGet(plan.ID, 0, planPageSizeDefault)
}

// markStale freezes the refusal reason on a plan whose world changed. The old
// confirmation is never extended; recovery is always a new plan. A failed
// persist surfaces as state_unavailable instead of silently vanishing: the
// caller must know the stale verdict was not durably recorded.
func (s *Service) markStale(plan *SyncPlan, reason string) error {
	plan.Phase = planPhaseStale
	plan.StaleReason = reason
	plan.UpdatedAt = time.Now().UTC()
	if err := s.DB.Save(plan).Error; err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// planRecovery maps a plan phase to its recovery hint. Hints never contain
// credentials or raw engine text; the original operation ID is the recovery
// handle for partial results.
func planRecovery(plan SyncPlan, code string) map[string]string {
	out := map[string]string{}
	switch code {
	case "plan_stale":
		hint := "Rescan both sides and generate a new plan; the old confirmation cannot be extended."
		if plan.OperationID != "" {
			hint = fmt.Sprintf("Read the original operation %s for partial results, then rescan and generate a new plan; the old confirmation cannot be extended.", plan.OperationID)
		}
		out[code] = hint
	case "writer_handoff_required":
		out[code] = "Another device owns this plan's writer slot and a timeout does not prove it stopped: confirm the old writer stopped, then create a new plan on this device to rebuild the baseline."
	case "plan_not_applicable":
		out[code] = "This plan already reached a terminal phase; read its original operation for results and generate a new plan for further work."
	case "plan_not_found":
		out[code] = "No plan with this ID exists on this machine; list plans from the terminal that created them."
	case "sync_scan_incomplete":
		out[code] = "A side scan was incomplete; deletions are not propagated on partial evidence. Restore listing (network or permissions) and plan again."
	case "version_conflict":
		out[code] = "The plan revision does not match your confirmation; re-read the plan and apply with its current revision."
	default:
		switch plan.Phase {
		case planPhasePlanned:
			out["next"] = fmt.Sprintf("Review the plan, then apply it with drivebridge sync apply %s --revision %d from the user terminal.", plan.ID, plan.Revision)
		case planPhaseInterrupted:
			op := plan.OperationID
			if op == "" {
				op = "(none recorded)"
			}
			out["next"] = fmt.Sprintf("Read the original operation %s for per-item results; unchanged sides resume with the same identity, changed sides require a new plan.", op)
		case planPhaseStale:
			out["next"] = "This plan is stale; generate a new one from a fresh scan."
		}
	}
	return out
}

// ApplySyncPlan revalidates the frozen world (writer, roots, baseline, both
// sides, deletion scale) and only then executes the frozen set through the
// same guarded engine as the legacy surface. Recovery keeps the original
// plan, operation identity and idempotency key; it never swaps them.
func (s *Service) ApplySyncPlan(ctx context.Context, planID string, opt ApplySyncPlanOptions) (SyncPlanResult, error) {
	if !s.Enabled {
		return SyncPlanResult{}, errors.New("disabled")
	}
	var plan SyncPlan
	if err := s.DB.First(&plan, "id = ?", planID).Error; err != nil {
		return SyncPlanResult{Recovery: planRecovery(plan, "plan_not_found")}, errors.New("plan_not_found")
	}
	if opt.ExpectedRevision <= 0 || opt.ExpectedRevision != plan.Revision {
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "version_conflict")}, errors.New("version_conflict")
	}
	switch plan.Phase {
	case planPhasePlanned, planPhaseInterrupted:
	case planPhaseStale:
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "plan_stale")}, errors.New("plan_stale")
	default: // applying, completed, failed
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "plan_not_applicable")}, errors.New("plan_not_applicable")
	}
	p, err := s.Space(plan.SpaceID)
	if err != nil {
		return SyncPlanResult{Plan: plan}, err
	}
	if plan.WriterHost != s.writerIdentity() {
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "writer_handoff_required")}, errors.New("writer_handoff_required")
	}
	local, remote, err := s.syncFsPair(ctx, p)
	if err != nil {
		return SyncPlanResult{Plan: plan}, err
	}
	basePath := bilib.BasePath(ctx, plan.Workdir, local, remote)
	snapshotPath := basePath + ".drivebridge.json"
	stale := func(reason string) (SyncPlanResult, error) {
		if err := s.markStale(&plan, reason); err != nil {
			return SyncPlanResult{Plan: plan}, err
		}
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "plan_stale")}, errors.New("plan_stale")
	}
	// Root identity: the plan is void when either root moved or rebound.
	if plan.LocalRootID != p.LocalRoot || plan.RemoteIdent != remoteIdent(p) {
		return stale("root_identity_changed")
	}
	// Baseline generation: an interleaved sync run moved the engine
	// snapshot between freeze and apply.
	if fileDigest(snapshotPath) != plan.BaselineSnapshotDigest {
		return stale("baseline_moved")
	}
	// Source observations: a complete fresh scan must reproduce the frozen
	// digests exactly. Incomplete rescans refuse before anything moves.
	side1, errs1 := planWalkSide(ctx, local)
	side2, errs2 := planWalkSide(ctx, remote)
	if errs1+errs2 > 0 {
		return SyncPlanResult{Plan: plan, Coverage: &ScanCoverage{LocalEntries: len(side1), RemoteEntries: len(side2), ScanErrors: errs1 + errs2}, Recovery: planRecovery(plan, "sync_scan_incomplete")}, errors.New("sync_scan_incomplete")
	}
	if scanDigest(side1) != plan.LocalDigest || scanDigest(side2) != plan.RemoteDigest {
		return stale("source_changed")
	}
	// Deletion scale: recompute the frozen classification against the same
	// snapshot and re-apply the frozen cap.
	if snapshot, have := loadSyncSnapshot(snapshotPath); have && !plan.Resync {
		fresh := SyncReport{}
		classifySync(snapshot, have, side1, side2, &fresh)
		if reason, e := checkDeletionCap(snapshot.Local, side1, "local", plan.MaxDelete); e != nil {
			return SyncPlanResult{Plan: plan, RefusalReason: reason}, e
		}
		if reason, e := checkDeletionCap(snapshot.Remote, side2, "remote", plan.MaxDelete); e != nil {
			return SyncPlanResult{Plan: plan, RefusalReason: reason}, e
		}
	}
	// The operation identity is keyed to the plan: retries after an
	// interrupted apply reuse the same row, key and conflict suffixes.
	opKey := operationKey(plan.SpaceID, "syncplan", plan.ID)
	var op Operation
	if err := s.DB.First(&op, "key = ?", opKey).Error; err != nil {
		op = Operation{ID: newID(), SpaceID: plan.SpaceID, Kind: "sync", Key: opKey, Status: "transferring", Updated: time.Now().UTC()}
		if err := s.DB.Create(&op).Error; err != nil {
			return SyncPlanResult{Plan: plan}, errors.New("state_unavailable")
		}
	} else {
		op.Status, op.Error, op.Updated = "transferring", "", time.Now().UTC()
		if err := s.DB.Save(&op).Error; err != nil {
			return SyncPlanResult{Plan: plan}, errors.New("state_unavailable")
		}
	}
	plan.Phase = planPhaseApplying
	plan.OperationID = op.ID
	plan.UpdatedAt = time.Now().UTC()
	if err := s.DB.Save(&plan).Error; err != nil {
		return SyncPlanResult{Plan: plan}, errors.New("state_unavailable")
	}
	engineOpt := bisync.Options{
		Resync:          plan.Resync,
		MaxDelete:       plan.MaxDelete,
		Workdir:         plan.Workdir,
		ConflictResolve: bisync.PreferNone,
		// Keep-both: both copies survive, named by the original operation.
		ConflictSuffixFlag: "sync-" + op.ID + "-local,sync-" + op.ID + "-remote",
	}
	if plan.Resync {
		engineOpt.ResyncMode = bisync.PreferPath1
	}
	engineErr := bisync.Bisync(ctx, local, remote, &engineOpt)
	if engineErr != nil {
		// Cancellation keeps completed side effects and leaves the plan
		// recoverable; it is not a terminal failure.
		if ctx.Err() != nil {
			op.Status, op.Error, op.Updated = "cancelled", "operation_cancelled", time.Now().UTC()
			_ = s.DB.Save(&op).Error
			plan.Phase, plan.StaleReason, plan.UpdatedAt = planPhaseInterrupted, "apply_cancelled", time.Now().UTC()
			_ = s.DB.Save(&plan).Error
			return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "")}, errors.New("operation_cancelled")
		}
		code := classifySyncError(engineErr)
		op.Status, op.Error, op.Updated = "failed", code, time.Now().UTC()
		_ = s.DB.Save(&op).Error
		s.emit(plan.SpaceID, "operation_failed", "", op.ID)
		plan.Phase, plan.UpdatedAt = planPhaseFailed, time.Now().UTC()
		_ = s.DB.Save(&plan).Error
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "")}, errors.New(code)
	}
	fresh1, fresh2 := walkSide(ctx, local), walkSide(ctx, remote)
	if err := saveSyncSnapshot(snapshotPath, fresh1, fresh2); err != nil {
		op.Status, op.Error, op.Updated = "failed", "state_unavailable", time.Now().UTC()
		_ = s.DB.Save(&op).Error
		plan.Phase, plan.UpdatedAt = planPhaseFailed, time.Now().UTC()
		_ = s.DB.Save(&plan).Error
		return SyncPlanResult{Plan: plan, Recovery: planRecovery(plan, "")}, errors.New("state_unavailable")
	}
	// Sync-created remote files enter the stable reference register.
	s.rememberRemoteSide(ctx, plan.SpaceID, fresh2)
	op.Status, op.Error, op.Updated = "completed", "", time.Now().UTC()
	_ = s.DB.Save(&op).Error
	s.emit(plan.SpaceID, "operation_completed", "", op.ID)
	plan.Phase, plan.UpdatedAt = planPhaseCompleted, time.Now().UTC()
	_ = s.DB.Save(&plan).Error
	return s.SyncPlanGet(plan.ID, 0, planPageSizeDefault)
}

// SyncPlanGet is the read-only plan projection shared by the CLI, the MCP
// sync_plan_get tool and the local owner socket. Items page by keyset with
// default 50 and maximum 200.
func (s *Service) SyncPlanGet(planID string, after uint64, limit int) (SyncPlanResult, error) {
	if limit <= 0 {
		limit = planPageSizeDefault
	}
	if limit > planPageSizeMax {
		limit = planPageSizeMax
	}
	var plan SyncPlan
	if err := s.DB.First(&plan, "id = ?", planID).Error; err != nil {
		return SyncPlanResult{Recovery: planRecovery(plan, "plan_not_found")}, errors.New("plan_not_found")
	}
	items := []SyncItem{}
	query := s.DB.Where("plan_id = ?", planID)
	if after > 0 {
		query = query.Where("id > ?", after)
	}
	if err := query.Order("id").Limit(limit + 1).Find(&items).Error; err != nil {
		return SyncPlanResult{Plan: plan}, errors.New("state_unavailable")
	}
	var total int64
	if err := s.DB.Model(&SyncItem{}).Where("plan_id = ?", planID).Count(&total).Error; err != nil {
		return SyncPlanResult{Plan: plan}, errors.New("state_unavailable")
	}
	next := uint64(0)
	if len(items) > limit {
		items = items[:limit]
		next = items[len(items)-1].ID
	}
	result := SyncPlanResult{Plan: plan, Items: items, NextAfter: next, TotalItems: total, Recovery: planRecovery(plan, "")}
	if plan.OperationID != "" {
		if op, err := s.Get(plan.OperationID); err == nil {
			result.Operation = &op
		}
	}
	return result, nil
}
