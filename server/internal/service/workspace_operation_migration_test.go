package service

import (
	"testing"
	"time"
)

// Only pre-workspace columns are present in this disposable migration fixture.
type legacyWorkspaceOperationSchema struct {
	ID           string `gorm:"primaryKey"`
	Owner        string `gorm:"uniqueIndex:operation_key"`
	Key          string `gorm:"uniqueIndex:operation_key"`
	AgentID      string
	ActorID      string
	Kind         string
	SpaceID      string
	ConnectionID string
	Path         string
	VersionID    string
	Status       string
	Size         int64
	Transferred  int64
	Fingerprint  string
	Updated      time.Time
}

func (legacyWorkspaceOperationSchema) TableName() string { return "operations" }

func TestWorkspaceOperationUpgradePreservesRecoveryIdentity(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&legacyWorkspaceOperationSchema{}); err != nil {
		t.Fatal(err)
	}
	original := legacyWorkspaceOperationSchema{ID: "original-operation", Owner: "owner", Key: "original-key", AgentID: "legacy-agent", ActorID: "owner", Kind: "upload", SpaceID: "space", ConnectionID: "connection", Path: "refs/file.txt", VersionID: "reserved-version", Status: "transferring", Size: 100, Transferred: 50, Fingerprint: "original-digest", Updated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.AutoMigrate(&Operation{}); err != nil {
			t.Fatal(err)
		}
		if err := migrateWorkspaceModels(db); err != nil {
			t.Fatal(err)
		}
		var current Operation
		if err := db.First(&current, "id = ?", original.ID).Error; err != nil {
			t.Fatal(err)
		}
		if current.Purpose != "" || current.WorkspaceGrantID != "" {
			t.Fatal("legacy operation reclassified as workspace")
		}
		if current.Key != original.Key || current.Owner != original.Owner || current.AgentID != original.AgentID || current.ActorID != original.ActorID || current.VersionID != original.VersionID || current.Fingerprint != original.Fingerprint || current.Path != original.Path || current.SpaceID != original.SpaceID || current.ConnectionID != original.ConnectionID || current.Status != original.Status || current.Size != original.Size || current.Transferred != original.Transferred || !current.Updated.Equal(original.Updated) {
			t.Fatal("migration changed original transfer identity or progress")
		}
	}
	duplicate := Operation{ID: "duplicate", Owner: original.Owner, Key: original.Key}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("upgrade lost original idempotency uniqueness")
	}
	// A different owner retains its independent key domain.
	duplicate.Owner = "other"
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
}
