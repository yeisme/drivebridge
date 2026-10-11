package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWorkspaceGrantReceiptSurvivesDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.sqlite")
	open := func() *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		pool, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = pool.Close() })
		if err = migrateWorkspaceModels(db); err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	var original WorkspaceRequest
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		original, err = changeWorkspaceGrantRequestOn(tx, "alice", "tenant", "alice", "grant", "request", 1, "disable", now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := open()
	if err = reopened.Transaction(func(tx *gorm.DB) error {
		replay, err := changeWorkspaceGrantRequestOn(tx, "alice", "tenant", "alice", "grant", "request", 1, "disable", now.Add(time.Hour))
		if err != nil {
			return err
		}
		if replay.ID != original.ID || replay.ResultRevision != original.ResultRevision || !replay.CreatedAt.Equal(original.CreatedAt) {
			t.Fatal("reopen changed original receipt")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stored WorkspaceGrant
	if err = reopened.First(&stored, "id = ?", grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 2 || !stored.Disabled || stored.ExpiresAt != nil {
		t.Fatal("reopen lost state or changed expiry")
	}
	var count int64
	if err = reopened.Model(&WorkspaceRequest{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("replay created another receipt")
	}
}
