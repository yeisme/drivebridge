package service

import (
	"errors"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWorkspaceGrantRequestReplayAndAtomicRollback(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	invoke := func(key string, revision uint64) (WorkspaceRequest, error) {
		var receipt WorkspaceRequest
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			receipt, err = changeWorkspaceGrantRequestOn(tx, "alice", "tenant", "human", "grant", key, revision, "disable", now)
			return err
		})
		return receipt, err
	}
	first, err := invoke("request", 1)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := invoke("request", 1)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != first.ID || replay.ResultRevision != 2 {
		t.Fatal("replay changed identity")
	}
	if _, err = invoke("request", 2); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed request accepted")
	}
	abort := errors.New("fixture abort")
	err = db.Transaction(func(tx *gorm.DB) error {
		if _, err := changeWorkspaceGrantRequestOn(tx, "alice", "tenant", "human", "grant", "enable", 2, "enable", now); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var got WorkspaceGrant
	if err = db.First(&got, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if got.Revision != 2 || !got.Disabled {
		t.Fatal("rolled back request changed state")
	}
	var count int64
	if err = db.Model(&WorkspaceRequest{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("replay or rollback created extra receipts")
	}
}
