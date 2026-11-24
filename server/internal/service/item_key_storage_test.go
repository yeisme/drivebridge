package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestItemKeyStoragePreservesLegacyAndWireIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "items.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&UploadBatchItem{}, &Operation{}, &OrganizationPlanItem{}); err != nil {
		t.Fatal(err)
	}
	key := "db-item-v1:notes/example.md\x00digest"
	item := UploadBatchItem{ID: "new", BatchID: "batch", ItemKey: key}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.ItemKey != key {
		t.Fatal("create changed API identity")
	}
	var read UploadBatchItem
	if err := matchingItemKey(db, key).First(&read, "batch_id = ?", "batch").Error; err != nil || read.ItemKey != key {
		t.Fatalf("encoded lookup: %v", err)
	}
	// Seed a pre-encoding row through the fixture repository, then prove it
	// still resolves without changing its durable request identity.
	if err := db.Model(&UploadBatchItem{}).Where("id = ?", "new").UpdateColumn("item_key", key).Error; err != nil {
		t.Fatal(err)
	}
	read = UploadBatchItem{}
	if err := matchingItemKey(db, key).First(&read, "batch_id = ?", "batch").Error; err != nil || read.ItemKey != key {
		t.Fatalf("legacy lookup: %v", err)
	}
	op := Operation{ID: "op", Owner: "owner", Key: "request", ItemKey: key}
	if err := db.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	var restored Operation
	if err := db.First(&restored, "id = ?", op.ID).Error; err != nil || restored.ItemKey != key {
		t.Fatalf("operation identity: %v", err)
	}
}

func TestOperationKeyStorageDoesNotGuessLegacyEncoding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "operations.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err = db.AutoMigrate(&Operation{}); err != nil {
		t.Fatal(err)
	}
	for i, key := range []string{"legacy\x00tuple", "db-item-v1:literal", "normal"} {
		op := Operation{ID: fmt.Sprint(i), Owner: "owner", Key: key, KeyEncodingVersion: operationKeyEncoding(key)}
		if err = db.Create(&op).Error; err != nil {
			t.Fatal(err)
		}
		var restored Operation
		if err = matchingOperationKey(db, "owner", key).First(&restored).Error; err != nil || restored.Key != key {
			t.Fatalf("key roundtrip failed: %v", err)
		}
	}
	legacy := Operation{ID: "legacy", Owner: "old-owner", Key: "old\x00key"}
	if err = db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	var restored Operation
	if err = matchingOperationKey(db, legacy.Owner, legacy.Key).First(&restored).Error; err != nil || restored.Key != legacy.Key {
		t.Fatalf("legacy key changed: %v", err)
	}
}
