package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceGrantCreationDurationMatrix(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, seconds := range []int64{3600, 86400, 604800, 2592000, 7776000, 31536000} {
		key := fmt.Sprintf("duration-%d", seconds)
		input := contract.CreateGrantRequest{Name: "Reader", Protocol: "mcp", SpaceID: "space", ConnectionID: "connection", Actions: []string{"files.read"}, ExpiryMode: contract.Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: contract.Field[int64]{Present: true, Value: seconds}}
		if err := db.Transaction(func(tx *gorm.DB) error {
			grant, _, issued, err := createWorkspaceGrantOn(tx, "alice", "tenant", "alice", key, Hash(key), input, now)
			if err != nil {
				return err
			}
			if !issued || grant.ExpiresAt == nil || !grant.ExpiresAt.Equal(now.Add(time.Duration(seconds)*time.Second)) {
				t.Fatalf("duration %d mismatch", seconds)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkspaceGrantCreationAtomicIdentityAndReplay(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	input := contract.CreateGrantRequest{Name: "Reader", Protocol: "http", SpaceID: "space", ConnectionID: "connection", Prefix: "refs", Actions: []string{"files.read"}}
	var first WorkspaceGrant
	var identity Agent
	err := db.Transaction(func(tx *gorm.DB) error {
		var issued bool
		var err error
		first, identity, issued, err = createWorkspaceGrantOn(tx, "alice", "tenant", "alice", "key", Hash("fixture-secret"), input, now)
		if err == nil && !issued {
			t.Fatal("first request did not issue")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExpiryMode != "never" || first.ExpiresAt != nil || identity.Scopes != "" || identity.RestrictionProfile != "personal_workspace_v1" || identity.Expires.After(now) {
		t.Fatal("new identity inherited broad legacy authority")
	}
	explicit := input
	explicit.ExpiryMode = contract.Field[string]{Present: true, Value: "never"}
	if err = db.Transaction(func(tx *gorm.DB) error {
		replay, a, issued, err := createWorkspaceGrantOn(tx, "alice", "tenant", "alice", "key", "", explicit, now.Add(time.Hour))
		if err != nil {
			return err
		}
		if issued || replay.ID != first.ID || a.ID != identity.ID || !replay.IssuedAt.Equal(now) {
			t.Fatal("replay changed identity or expiry")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	changed := input
	changed.Name = "Different"
	if err = db.Transaction(func(tx *gorm.DB) error {
		_, _, _, err := createWorkspaceGrantOn(tx, "alice", "tenant", "alice", "key", Hash("unused"), changed, now)
		return err
	}); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed request reused key")
	}
	abort := errors.New("fixture rollback")
	if err = db.Transaction(func(tx *gorm.DB) error {
		if _, _, _, err := createWorkspaceGrantOn(tx, "alice", "tenant", "alice", "aborted", Hash("fixture-other"), input, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	for _, model := range []any{&Agent{}, &WorkspaceGrant{}, &WorkspaceRequest{}, &WorkspaceCredentialHistory{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("unexpected records after replay/rollback for %T", model)
		}
	}
}
