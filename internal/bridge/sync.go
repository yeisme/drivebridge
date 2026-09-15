package bridge

import (
	"context"
	"encoding/json"
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
)

// syncDefaultMaxDelete tightens rclone bisync's 50% deletion cap: a manual
// sync still refuses to propagate mass deletions unless explicitly overridden.
const syncDefaultMaxDelete = 25

// SyncOptions controls one guarded bidirectional sync run.
type SyncOptions struct {
	Resync    bool
	DryRun    bool
	MaxDelete int    // percent of deletions allowed per side; 0 = tightened default
	Workdir   string // journal location; empty = the rclone cache default
}

// SyncReport is the four-class plan plus run status. The plan mirrors the
// engine's size/modtime comparison against the last successful baseline.
type SyncReport struct {
	Space       string   `json:"space"`
	OperationID string   `json:"operation_id"`
	Status      string   `json:"status"`
	DryRun      bool     `json:"dry_run"`
	Resync      bool     `json:"resync"`
	Baseline    bool     `json:"baseline_established"`
	MaxDelete   int      `json:"max_delete_percent"`
	Reason      string   `json:"refusal_reason,omitempty"`
	Additions1  []string `json:"additions_local_only,omitempty"`
	Additions2  []string `json:"additions_remote_only,omitempty"`
	Updates1    []string `json:"updates_local_to_remote,omitempty"`
	Updates2    []string `json:"updates_remote_to_local,omitempty"`
	Deletions1  []string `json:"deletions_local,omitempty"`
	Deletions2  []string `json:"deletions_remote,omitempty"`
	Conflicts   []string `json:"conflicts,omitempty"`
}

type syncEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// syncSnapshot is the DriveBridge-owned baseline next to the engine journal;
// it backs the plan, the deletion cap check and honest recovery reporting.
type syncSnapshot struct {
	Local  map[string]syncEntry `json:"local"`
	Remote map[string]syncEntry `json:"remote"`
	At     time.Time            `json:"at"`
}

// remoteFsProvider is implemented by the providers that can expose the
// space's remote root as an rclone fs pair for the sync engine.
type remoteFsProvider interface {
	Fs(ctx context.Context, p Space) (fs.Fs, error)
}

// Fs exposes the configured rclone onedrive fs scoped to the space root.
func (r *OneDrive) Fs(ctx context.Context, p Space) (fs.Fs, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.client(ctx, p)
}

// Fs exposes the generic rclone remote scoped to the space root.
func (g *Generic) Fs(ctx context.Context, p Space) (fs.Fs, error) {
	return g.remote(p, ctx)
}

// Sync runs one guarded bidirectional sync between the space's local root and
// its remote root. The first run requires an explicit resync to establish the
// baseline; deletions are capped; conflicts keep both copies named by the
// operation ID; a dry run reports the plan without touching either side. The
// command surface is deliberately CLI-only: neither MCP nor agents trigger it.
func (s *Service) Sync(ctx context.Context, space string, opt SyncOptions) (SyncReport, error) {
	if !s.Enabled {
		return SyncReport{}, errors.New("disabled")
	}
	p, err := s.Space(space)
	if err != nil {
		return SyncReport{}, err
	}
	if info, e := os.Stat(p.LocalRoot); e != nil || !info.IsDir() {
		return SyncReport{}, errors.New("local_root_unavailable")
	}
	local, err := fs.NewFs(ctx, filepath.ToSlash(p.LocalRoot))
	if err != nil {
		return SyncReport{}, errors.New("local_root_unavailable")
	}
	provider, ok := s.providerFor(p).(remoteFsProvider)
	if !ok {
		return SyncReport{}, errors.New("sync_unsupported")
	}
	remote, err := provider.Fs(ctx, p)
	if err != nil {
		return SyncReport{}, err
	}
	workdir := opt.Workdir
	if workdir == "" {
		// The rclone cache default keeps parity with manual rclone bisync runs.
		workdir = bisync.DefaultWorkdir
	}
	basePath := bilib.BasePath(ctx, workdir, local, remote)
	journal := basePath + ".path1.lst"
	if _, e := os.Stat(journal); e != nil && !opt.Resync {
		// No baseline: the engine must not guess which side is authoritative.
		return SyncReport{Space: space, Status: "refused", Resync: opt.Resync, DryRun: opt.DryRun}, errors.New("resync_required")
	}
	if !opt.Resync {
		if e := trustSyncJournal(basePath + ".path1.lst"); e != nil {
			return SyncReport{Space: space, Status: "refused", Resync: opt.Resync, DryRun: opt.DryRun}, e
		}
		if e := trustSyncJournal(basePath + ".path2.lst"); e != nil {
			return SyncReport{Space: space, Status: "refused", Resync: opt.Resync, DryRun: opt.DryRun}, e
		}
	}
	maxDelete := opt.MaxDelete
	if maxDelete <= 0 {
		maxDelete = syncDefaultMaxDelete
	}
	snapshot, haveSnapshot := loadSyncSnapshot(basePath + ".drivebridge.json")
	side1 := walkSide(ctx, local)
	side2 := walkSide(ctx, remote)
	report := SyncReport{Space: space, DryRun: opt.DryRun, Resync: opt.Resync, Baseline: haveSnapshot, MaxDelete: maxDelete, Status: "planned"}
	classifySync(snapshot, haveSnapshot, side1, side2, &report)
	sortSyncReport(&report)
	// The tightened deletion cap runs before the engine: refusing early keeps
	// both sides untouched and reports the exact ratio and limit.
	if haveSnapshot && !opt.Resync {
		if reason, e := checkDeletionCap(snapshot.Local, side1, "local", maxDelete); e != nil {
			report.Status, report.Reason = "refused", reason
			return report, e
		}
		if reason, e := checkDeletionCap(snapshot.Remote, side2, "remote", maxDelete); e != nil {
			report.Status, report.Reason = "refused", reason
			return report, e
		}
	}
	opID := newID()
	op := Operation{ID: opID, SpaceID: space, Kind: "sync", Key: operationKey(space, "sync", opID), Status: "transferring", Updated: time.Now().UTC()}
	if err = s.DB.Create(&op).Error; err != nil {
		return report, errors.New("state_unavailable")
	}
	report.OperationID = opID
	engineOpt := bisync.Options{
		Resync:          opt.Resync,
		DryRun:          opt.DryRun,
		MaxDelete:       maxDelete,
		Workdir:         workdir,
		ConflictResolve: bisync.PreferNone,
		// Keep-both: both copies survive, named by this operation's ID.
		ConflictSuffixFlag: "sync-" + opID + "-local,sync-" + opID + "-remote",
	}
	if opt.Resync {
		// The baseline seeds from path1 (the space's local root): differing
		// files resolve deterministically instead of erroring.
		engineOpt.ResyncMode = bisync.PreferPath1
	}
	engineErr := bisync.Bisync(ctx, local, remote, &engineOpt)
	op.Updated = time.Now().UTC()
	if engineErr != nil {
		code := classifySyncError(engineErr)
		op.Status, op.Error = "failed", code
		_ = s.DB.Save(&op).Error
		s.emit(op.SpaceID, "operation_failed", "", op.ID)
		report.Status = "failed"
		// Only the fixed code crosses the boundary; the engine's own wording
		// stays out of error strings (rclone text can carry URLs).
		return report, errors.New(code)
	}
	if !opt.DryRun {
		fresh1, fresh2 := walkSide(ctx, local), walkSide(ctx, remote)
		if err = saveSyncSnapshot(basePath+".drivebridge.json", fresh1, fresh2); err != nil {
			op.Status, op.Error = "failed", "state_unavailable"
			_ = s.DB.Save(&op).Error
			return report, errors.New("state_unavailable")
		}
		// Sync-created remote files enter the stable reference register the
		// same way manual uploads do: one provider listing per touched
		// directory performs the remember pass.
		dirs := map[string]bool{"": true}
		for path := range fresh2 {
			dirs[filepath.ToSlash(filepath.Dir(path))] = true
		}
		for dir := range dirs {
			if dir == "." || dir == "/" {
				dir = ""
			}
			_, _ = s.List(ctx, space, dir)
		}
	}
	op.Status, op.Error = "completed", ""
	_ = s.DB.Save(&op).Error
	s.emit(op.SpaceID, "operation_completed", "", op.ID)
	report.Status = "completed"
	return report, nil
}

// trustSyncJournal validates the structural shape of an engine listing. A
// journal that fails the basic shape check must not seed a run: the honest
// answer is an explicit resync demand, never a guessed baseline.
func trustSyncJournal(listing string) error {
	b, err := os.ReadFile(listing)
	if err != nil {
		return errors.New("resync_required")
	}
	head := string(b)
	if i := strings.IndexByte(head, '\n'); i >= 0 {
		head = head[:i]
	}
	if !strings.HasPrefix(head, "# bisync listing ") {
		return errors.New("resync_required")
	}
	return nil
}
// last baseline. Without a baseline (first run), only cross-side differences
// are classifiable: equal files are already in sync, differing files are
// conflicts, one-sided files are additions.
func classifySync(snapshot syncSnapshot, haveSnapshot bool, side1, side2 map[string]syncEntry, report *SyncReport) {
	report.Additions1, report.Additions2 = []string{}, []string{}
	report.Updates1, report.Updates2 = []string{}, []string{}
	report.Deletions1, report.Deletions2 = []string{}, []string{}
	report.Conflicts = []string{}
	entryDiffers := func(a, b syncEntry) bool {
		return a.Size != b.Size || a.ModTime.Sub(b.ModTime) > time.Second || b.ModTime.Sub(a.ModTime) > time.Second
	}
	for path, e1 := range side1 {
		e2, in2 := side2[path]
		if !haveSnapshot {
			switch {
			case !in2:
				report.Additions1 = append(report.Additions1, path)
			case entryDiffers(e1, e2):
				report.Conflicts = append(report.Conflicts, path)
			}
			continue
		}
		b1, had1 := snapshot.Local[path]
		b2, had2 := snapshot.Remote[path]
		changed1 := had1 && entryDiffers(e1, b1)
		changed2 := had2 && entryDiffers(e2, b2)
		switch {
		case !in2:
			switch {
			case !had1 && had2:
				report.Deletions2 = append(report.Deletions2, path)
			default:
				report.Additions1 = append(report.Additions1, path)
			}
		case changed1 && changed2:
			report.Conflicts = append(report.Conflicts, path)
		case changed1:
			if !had2 {
				report.Additions1 = append(report.Additions1, path)
			} else {
				report.Updates1 = append(report.Updates1, path)
			}
		case changed2:
			report.Updates2 = append(report.Updates2, path)
		}
	}
	for path := range side2 {
		if _, in1 := side1[path]; in1 {
			continue
		}
		_, had1 := snapshot.Local[path]
		if haveSnapshot && had1 {
			report.Deletions1 = append(report.Deletions1, path)
		} else {
			report.Additions2 = append(report.Additions2, path)
		}
	}
}

// checkDeletionCap refuses when the share of baseline entries that vanished
// from one side exceeds the tightened cap.
func checkDeletionCap(baseline, current map[string]syncEntry, side string, maxDelete int) (string, error) {
	if len(baseline) == 0 {
		return "", nil
	}
	deleted := 0
	for path := range baseline {
		if _, ok := current[path]; !ok {
			deleted++
		}
	}
	if deleted == 0 {
		return "", nil
	}
	ratio := 100 * deleted / len(baseline)
	if ratio > maxDelete {
		return fmt.Sprintf("%s side would delete %d of %d (%d%%) above the %d%% cap; raise --max-delete to override", side, deleted, len(baseline), ratio, maxDelete), errors.New("sync_deletion_cap_exceeded")
	}
	return "", nil
}

// classifySyncError maps engine failures to the fixed code set.
func classifySyncError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "cannot read prior listing"), strings.Contains(message, "corrupt"):
		return "resync_required"
	case strings.Contains(message, "too many deletes"):
		return "sync_deletion_cap_exceeded"
	}
	return "sync_failed"
}

func walkSide(ctx context.Context, f fs.Fs) map[string]syncEntry {
	out := map[string]syncEntry{}
	_ = walk.Walk(ctx, f, "", false, -1, func(path string, entries fs.DirEntries, err error) error {
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if o, ok := entry.(fs.Object); ok {
				out[strings.TrimPrefix(o.Remote(), "/")] = syncEntry{Size: o.Size(), ModTime: o.ModTime(ctx)}
			}
		}
		return nil
	})
	return out
}

func loadSyncSnapshot(path string) (syncSnapshot, bool) {
	var snapshot syncSnapshot
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &snapshot) != nil || len(snapshot.Local) == 0 {
		return syncSnapshot{}, false
	}
	return snapshot, true
}

func saveSyncSnapshot(path string, side1, side2 map[string]syncEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(syncSnapshot{Local: side1, Remote: side2, At: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func sortSyncReport(report *SyncReport) {
	for _, list := range []*[]string{&report.Additions1, &report.Additions2, &report.Updates1, &report.Updates2, &report.Deletions1, &report.Deletions2, &report.Conflicts} {
		sort.Strings(*list)
	}
}
