// Package managedstate implements the candidate local state-layout fence.
// It is not wired to service startup or binding admission. A fenced database
// is a maintenance state, not permission to start a reader or writer session.
package managedstate

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/gofrs/flock"
)

const markerPrefix = "DriveBridge managed state v1\n"
const snapshotDir = ".managed-state-v1"

var identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// Plan is an immutable source revision, not a guard or authorization token.
type Plan struct {
	Kind          string `json:"kind"`
	MigrationID   string `json:"migration_id"`
	StateIdentity string `json:"state_identity"`
	SourceDigest  string `json:"source_digest"`
	SourceBytes   int64  `json:"source_bytes"`
}

type State struct {
	Layout string
	Plan   Plan
	// DatabasePath stays inside the owning application; never send it to a renderer.
	DatabasePath string
}

func code(value string) error { return errors.New(value) }

func databaseName(kind string) (string, error) {
	switch kind {
	case "cli":
		return "state.sqlite", nil
	case "server":
		return "metadata.sqlite", nil
	default:
		return "", code("state_kind_invalid")
	}
}

func openState(path string) (*os.Root, error) {
	// Linux has process-crash evidence. Other platforms must earn their own
	// atomic replacement, directory-sync and legacy-binary acceptance first.
	if runtime.GOOS != "linux" {
		return nil, code("state_layout_unsupported")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) || canonical != path {
		return nil, code("state_path_invalid")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, code("state_unavailable")
	}
	info, err := root.Lstat(".")
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		root.Close()
		return nil, code("state_not_private")
	}
	return root, nil
}

func privateFile(root *os.Root, name string) (*os.File, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, code("state_file_invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, code("state_unavailable")
	}
	current, err := file.Stat()
	if err != nil || !os.SameFile(info, current) {
		file.Close()
		return nil, code("state_changed")
	}
	return file, nil
}

func digest(root *os.Root, name string) (string, int64, error) {
	file, err := privateFile(root, name)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, code("state_unavailable")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), size, nil
}

func noSidecars(root *os.Root, name string) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := root.Lstat(name + suffix); !errors.Is(err, os.ErrNotExist) {
			return code("state_checkpoint_required")
		}
	}
	return nil
}

func sameRoot(root *os.Root) bool {
	opened, err := root.Lstat(".")
	if err != nil {
		return false
	}
	current, err := os.Lstat(root.Name())
	return err == nil && current.IsDir() && os.SameFile(opened, current)
}

func inspect(root *os.Root, kind string) (State, error) {
	name, err := databaseName(kind)
	if err != nil {
		return State{}, err
	}
	file, err := privateFile(root, name)
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err = io.ReadFull(file, header); err != nil {
		return State{}, code("state_layout_invalid")
	}
	if string(header) == "SQLite format 3\x00" {
		return State{Layout: "legacy", DatabasePath: filepath.Join(root.Name(), name)}, nil
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return State{}, code("state_unavailable")
	}
	payload, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(payload) > 4096 || !bytes.HasPrefix(payload, []byte(markerPrefix)) {
		return State{}, code("state_layout_invalid")
	}
	var plan Plan
	decoder := json.NewDecoder(bytes.NewReader(payload[len(markerPrefix):]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Kind != kind || !validPlan(plan) {
		return State{}, code("state_layout_invalid")
	}
	// Only the canonical encoding authored by this service is accepted,
	// including rejection of duplicate JSON keys and alternate spellings.
	encoded, _ := json.Marshal(plan)
	if !bytes.Equal(payload, append([]byte(markerPrefix), encoded...)) {
		return State{}, code("state_layout_invalid")
	}
	directory, err := root.Lstat(snapshotDir)
	if err != nil || !directory.IsDir() || directory.Mode().Perm()&0077 != 0 {
		return State{}, code("state_recovery_required")
	}
	copy, err := privateFile(root, snapshotDir+"/"+name)
	if err != nil {
		return State{}, code("state_recovery_required")
	}
	defer copy.Close()
	if _, err = io.ReadFull(copy, header); err != nil || string(header) != "SQLite format 3\x00" {
		return State{}, code("state_recovery_required")
	}
	return State{Layout: "fenced", Plan: plan, DatabasePath: filepath.Join(root.Name(), snapshotDir, name)}, nil
}

func validPlan(plan Plan) bool {
	_, err := databaseName(plan.Kind)
	return err == nil && identityPattern.MatchString(plan.MigrationID) && digestPattern.MatchString(plan.StateIdentity) && digestPattern.MatchString(plan.SourceDigest) && plan.SourceBytes >= 16
}

// Inspect never initializes, repairs or mutates a state directory.
func Inspect(path, kind string) (State, error) {
	root, err := openState(path)
	if err != nil {
		return State{}, err
	}
	defer root.Close()
	state, err := inspect(root, kind)
	if err == nil && !sameRoot(root) {
		return State{}, code("state_changed")
	}
	return state, err
}

// Prepare is read-only. Install rechecks this revision under the existing lock.
func Prepare(path, kind, migrationID string) (Plan, error) {
	root, err := openState(path)
	if err != nil {
		return Plan{}, err
	}
	defer root.Close()
	state, err := inspect(root, kind)
	if err != nil {
		return Plan{}, err
	}
	if state.Layout != "legacy" || !identityPattern.MatchString(migrationID) {
		return Plan{}, code("state_plan_invalid")
	}
	name, _ := databaseName(kind)
	if err = noSidecars(root, name); err != nil {
		return Plan{}, err
	}
	sum, size, err := digest(root, name)
	if err != nil {
		return Plan{}, err
	}
	identity, err := rootIdentity(root)
	return Plan{Kind: kind, MigrationID: migrationID, StateIdentity: identity, SourceDigest: sum, SourceBytes: size}, err
}

// Install requires a separately authorized, quiescent state migration. It
// acquires the original process.lock; it never stops another process or drops
// WAL files. A fenced result still requires the separate guard installation.
func Install(path string, plan Plan) (State, error) { return install(path, plan, nil) }

func install(path string, plan Plan, checkpoint func(string)) (State, error) {
	if !validPlan(plan) {
		return State{}, code("state_plan_invalid")
	}
	root, err := openState(path)
	if err != nil {
		return State{}, err
	}
	defer root.Close()
	lockFile, err := privateFile(root, "process.lock")
	if err != nil {
		return State{}, code("state_lock_unverifiable")
	}
	defer lockFile.Close()
	lock := flock.New(filepath.Join(path, "process.lock"), flock.SetFlag(os.O_RDWR))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return State{}, code("state_locked")
	}
	defer lock.Close()
	lockInfo, err := lockFile.Stat()
	if err != nil {
		return State{}, code("state_lock_unverifiable")
	}
	current, err := root.Lstat("process.lock")
	if err != nil || !os.SameFile(lockInfo, current) {
		return State{}, code("state_lock_unverifiable")
	}
	if !sameRoot(root) {
		return State{}, code("state_changed")
	}
	if identity, err := rootIdentity(root); err != nil || identity != plan.StateIdentity {
		return State{}, code("state_changed")
	}
	state, err := inspect(root, plan.Kind)
	if err != nil {
		return State{}, err
	}
	if state.Layout == "fenced" {
		if state.Plan != plan {
			return State{}, code("state_migration_conflict")
		}
		if err = syncDir(root, "."); err != nil {
			return State{}, code("state_commit_unknown")
		}
		return state, nil
	}
	name, _ := databaseName(plan.Kind)
	if err = noSidecars(root, name); err != nil {
		return State{}, err
	}
	if sum, size, err := digest(root, name); err != nil || sum != plan.SourceDigest || size != plan.SourceBytes {
		return State{}, code("state_changed")
	}
	if err = syncDir(root, "."); err != nil {
		return State{}, code("state_layout_unsupported")
	}
	if err = root.Mkdir(snapshotDir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return State{}, code("state_unavailable")
	}
	info, err := root.Lstat(snapshotDir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return State{}, code("state_file_invalid")
	}
	if err = snapshot(root, name, plan, checkpoint); err != nil {
		return State{}, err
	}
	if err = syncDir(root, snapshotDir); err != nil {
		return State{}, code("state_unavailable")
	}
	if err = syncDir(root, "."); err != nil {
		return State{}, code("state_unavailable")
	}
	step(checkpoint, "snapshot_durable")
	encoded, _ := json.Marshal(plan)
	temp, file, err := temporary(root, ".")
	if err != nil {
		return State{}, err
	}
	defer root.Remove(temp)
	_, writeErr := file.Write(append([]byte(markerPrefix), encoded...))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return State{}, code("state_unavailable")
	}
	step(checkpoint, "marker_durable")
	if !sameRoot(root) {
		return State{}, code("state_changed")
	}
	if err = noSidecars(root, name); err != nil {
		return State{}, err
	}
	if sum, size, err := digest(root, name); err != nil || sum != plan.SourceDigest || size != plan.SourceBytes {
		return State{}, code("state_changed")
	}
	if err = root.Rename(temp, name); err != nil {
		return State{}, code("state_unavailable")
	}
	step(checkpoint, "marker_published")
	if err = syncDir(root, "."); err != nil {
		return State{}, code("state_commit_unknown")
	}
	step(checkpoint, "commit_durable")
	if !sameRoot(root) {
		return State{}, code("state_commit_unknown")
	}
	return inspect(root, plan.Kind)
}

func step(checkpoint func(string), name string) {
	if checkpoint != nil {
		checkpoint(name)
	}
}

func temporary(root *os.Root, directory string) (string, *os.File, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", nil, code("state_unavailable")
	}
	name := filepath.Join(directory, ".fence-"+hex.EncodeToString(random[:])+".tmp")
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, code("state_unavailable")
	}
	return name, file, nil
}

func snapshot(root *os.Root, name string, plan Plan, checkpoint func(string)) error {
	target := snapshotDir + "/" + name
	if _, err := root.Lstat(target); err == nil {
		sum, size, err := digest(root, target)
		if err != nil || sum != plan.SourceDigest || size != plan.SourceBytes {
			return code("state_recovery_required")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return code("state_unavailable")
	}
	source, err := privateFile(root, name)
	if err != nil {
		return err
	}
	defer source.Close()
	temp, file, err := temporary(root, snapshotDir)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	hash := sha256.New()
	size, copyErr := io.Copy(file, io.TeeReader(source, hash))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		return code("state_unavailable")
	}
	if "sha256:"+hex.EncodeToString(hash.Sum(nil)) != plan.SourceDigest || size != plan.SourceBytes {
		return code("state_changed")
	}
	step(checkpoint, "snapshot_written")
	// Link publishes without replacing an existing recovery artifact. The
	// old SQLite path remains valid until the final marker rename.
	if err = root.Link(temp, target); err != nil {
		return code("state_recovery_required")
	}
	return nil
}

func syncDir(root *os.Root, name string) error {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
