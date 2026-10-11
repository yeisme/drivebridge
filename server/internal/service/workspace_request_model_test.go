package service

import (
	"errors"
	"gorm.io/gorm"
	"testing"
)

func TestWorkspaceRequestIdentityAndRollback(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	original := WorkspaceRequest{ID: "receipt-one", Owner: "alice", TenantID: "tenant", ActorID: "human", Action: "grant.disable", RequestKey: "request", RequestDigest: "digest", ResourceID: "grant", ResultRevision: 2}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := original
	duplicate.ID = "receipt-two"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate actor/action/key accepted")
	}
	other := original
	other.ID = "receipt-other"
	other.ActorID = "other"
	if err := db.Create(&other).Error; err != nil {
		t.Fatal("actor identity not isolated")
	}
	abort := errors.New("rollback fixture")
	err := db.Transaction(func(tx *gorm.DB) error {
		receipt := original
		receipt.ID = "receipt-aborted"
		receipt.RequestKey = "aborted"
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&WorkspaceRequest{}).Where("request_key = ?", "aborted").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("uncommitted receipt survived rollback")
	}
}
