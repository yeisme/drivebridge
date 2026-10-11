package service

import (
	"errors"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWorkspaceGrantConcurrentRevisionSingleWinner(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, action := range []string{"disable", "revoke"} {
		go func(action string) {
			<-start
			results <- db.Transaction(func(tx *gorm.DB) error {
				_, err := changeWorkspaceGrantStateOn(tx, "alice", "tenant", "grant", 1, action, now)
				return err
			})
		}(action)
	}
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if err.Error() == "revision_conflict" {
			conflicts++
		} else {
			t.Fatalf("unexpected result: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
	var got WorkspaceGrant
	if err := db.First(&got, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if got.Revision != 2 || (got.Disabled == got.Revoked) {
		t.Fatal("lost update or mixed decisions")
	}
}

func TestWorkspaceGrantCASAndRollback(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Hour)
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &expiry, IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	abort := errors.New("fixture rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := changeWorkspaceGrantStateOn(tx, "alice", "tenant", "grant", 1, "disable", now); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var unchanged WorkspaceGrant
	if err := db.First(&unchanged, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != 1 || unchanged.Disabled {
		t.Fatal("rollback leaked mutation")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		updated, err := changeWorkspaceGrantStateOn(tx, "alice", "tenant", "grant", 1, "disable", now)
		if err != nil {
			return err
		}
		if updated.Revision != 2 || !updated.Disabled || !updated.ExpiresAt.Equal(expiry) {
			t.Fatal("state or expiry changed incorrectly")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := changeWorkspaceGrantStateOn(db, "alice", "tenant", "grant", 1, "enable", now); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale revision won")
	}
	if _, err := changeWorkspaceGrantStateOn(db, "bob", "tenant", "grant", 2, "enable", now); err == nil || err.Error() != "not_found" {
		t.Fatal("cross-owner mutation")
	}
	if _, err := changeWorkspaceGrantStateOn(db, "alice", "tenant", "grant", 2, "enable", expiry); err == nil {
		t.Fatal("expired grant revived")
	}
	if _, err := changeWorkspaceGrantStateOn(db, "alice", "tenant", "grant", 2, "revoke", now); err != nil {
		t.Fatal(err)
	}
	if _, err := changeWorkspaceGrantStateOn(db, "alice", "tenant", "grant", 3, "enable", now); err == nil {
		t.Fatal("revoked grant revived")
	}
}
