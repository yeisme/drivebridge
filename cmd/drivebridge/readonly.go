package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge/internal/bridge"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Snapshot reads back the operations table without the process lock,
// mirroring doctor's read-only database pattern. It backs the two
// snapshot-friendly commands (recent, operation) while a registered MCP
// client holds the exclusive state lock, so inspection never dies on the
// lock that guards mutation.
type snapshot struct {
	db *sql.DB
}

func openSnapshot(state string) (*snapshot, error) {
	path := filepath.Join(state, "state.sqlite")
	if _, err := os.Lstat(path); err != nil {
		return nil, fail("state_unavailable", "The state database does not exist yet; run a DriveBridge service command once.")
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fail("state_unavailable", "The state database could not be opened read-only.")
	}
	return &snapshot{db: db}, nil
}

func (s *snapshot) close() { _ = s.db.Close() }

const operationColumns = "id, space_id, kind, status, caller, task_ref, file_ref, bytes, transferred, error, updated"

func scanOperations(rows *sql.Rows) ([]bridge.Operation, error) {
	defer rows.Close()
	out := []bridge.Operation{}
	for rows.Next() {
		var op bridge.Operation
		var updated time.Time
		if err := rows.Scan(&op.ID, &op.SpaceID, &op.Kind, &op.Status, &op.Caller, &op.TaskRef, &op.FileRef, &op.Bytes, &op.Transferred, &op.Error, &updated); err != nil {
			return nil, fail("state_unavailable", "The operations table could not be read.")
		}
		op.Updated = updated
		out = append(out, op)
	}
	return out, rows.Err()
}

func (s *snapshot) operations(space string, pending bool, limit int) ([]bridge.Operation, error) {
	query, args := "SELECT "+operationColumns+" FROM operations", []any{}
	where := ""
	if space != "" {
		where, args = "space_id = ?", append(args, space)
	}
	if pending {
		if where != "" {
			where += " AND "
		}
		where += "status NOT IN ('completed', 'cancelled')"
	}
	if where != "" {
		query += " WHERE " + where
	}
	rows, err := s.db.Query(query+" ORDER BY updated DESC LIMIT ?", append(args, limit)...)
	if err != nil {
		return nil, fail("state_unavailable", "The operations table could not be read.")
	}
	return scanOperations(rows)
}

func (s *snapshot) operation(id string) (bridge.Operation, error) {
	rows, err := s.db.Query("SELECT "+operationColumns+" FROM operations WHERE id = ?", id)
	if err != nil {
		return bridge.Operation{}, fail("state_unavailable", "The operations table could not be read.")
	}
	ops, err := scanOperations(rows)
	if err != nil {
		return bridge.Operation{}, err
	}
	if len(ops) != 1 {
		return bridge.Operation{}, fail("reference_not_found", "The operation does not exist in this state.")
	}
	return ops[0], nil
}

// openReadonlyModel opens the state database read-only through GORM so
// index-backed commands can serve the same model semantics (filtering,
// ordering, JSON projection) without the exclusive process lock and without
// any migration or write path.
func openReadonlyModel(state string) (*gorm.DB, error) {
	path := filepath.Join(state, "state.sqlite")
	if _, err := os.Lstat(path); err != nil {
		return nil, fail("state_unavailable", "The state database does not exist yet; run a DriveBridge service command once.")
	}
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fail("state_unavailable", "The state database could not be opened read-only.")
	}
	return db, nil
}

// snapshotSearch mirrors Service.Search over the read-only snapshot: the
// index may lag the remote, so snapshot hits stay advisory — stat verifies.
func snapshotSearch(db *gorm.DB, space, query, dir, mime string, after time.Time) ([]bridge.File, error) {
	tx := db.Where("space_id = ? AND trashed_at IS NULL", space)
	if dir != "" {
		tx = tx.Where("dir = ?", dir)
	}
	if mime != "" {
		tx = tx.Where("mime = ?", mime)
	}
	if !after.IsZero() {
		tx = tx.Where("observed_at >= ?", after)
	}
	if needle := strings.ToLower(query); needle != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+needle+"%")
	}
	files := []bridge.File{}
	if err := tx.Order("name").Find(&files).Error; err != nil {
		return nil, fail("state_unavailable", "The files index could not be read.")
	}
	return files, nil
}

// isStateLocked reports whether the error is the typed lock conflict.
func isStateLocked(err error) bool {
	var f *cliFailure
	return errors.As(err, &f) && f.code == "state_locked"
}
