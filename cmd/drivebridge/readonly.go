package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge/internal/bridge"
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
