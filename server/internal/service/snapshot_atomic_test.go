package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestSnapshotAtomicPublication(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	first := orgUpload(t, c, h.space.ID, "snapshot/first.txt", "first snapshot fixture")
	second := orgUpload(t, c, h.space.ID, "snapshot/second.txt", "second snapshot fixture")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic", Kind: "manual", Scope: "private"}, "atomic-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{first.FileID, second.FileID}}); err != nil {
		t.Fatal(err)
	}
	assertEmpty := func() {
		t.Helper()
		for _, model := range []any{&service.CollectionSnapshot{}, &service.CollectionSnapshotEntry{}} {
			var count int64
			if err := f.Service.DB.Model(model).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed snapshot retained %T: count=%d error=%v", model, count, err)
			}
		}
	}
	for _, table := range []string{"collection_snapshots", "collection_snapshot_entries"} {
		hits := 0
		if err = f.Service.DB.Callback().Create().Before("gorm:create").Register("snapshot_write_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				if table == "collection_snapshots" || hits == 2 {
					tx.AddError(errors.New("synthetic snapshot write failure"))
				}
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.CreateFileSnapshot(p, collection.CollectionID)
		f.Service.DB.Callback().Create().Remove("snapshot_write_failure")
		if failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("snapshot write failure ignored", failure)
		}
		if table == "collection_snapshot_entries" && hits != 2 {
			t.Fatal("test did not reach second-entry failure")
		}
		assertEmpty()
	}
	hits := 0
	if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("snapshot_version_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "versions" {
			hits++
			tx.AddError(errors.New("synthetic version failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	f.Service.DB.Callback().Query().Remove("snapshot_version_failure")
	if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal("missing version proof silently accepted", failure)
	}
	assertEmpty()
	// Change the observed head inside the publication transaction, after the
	// enumeration read. This simulates a stale candidate without timing races.
	if err = f.Service.DB.Callback().Create().After("gorm:create").Register("snapshot_head_drift", func(tx *gorm.DB) {
		if tx.Statement.Table == "collection_snapshots" {
			err := tx.Session(&gorm.Session{NewDB: true}).Model(&service.File{}).Where("id = ?", first.FileID).Update("version_id", "changed-head").Error
			if err != nil {
				tx.AddError(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure = f.Service.CreateFileSnapshot(p, collection.CollectionID)
	f.Service.DB.Callback().Create().Remove("snapshot_head_drift")
	if failure == nil || failure.Error() != "query_changed" {
		t.Fatal("stale head silently frozen", failure)
	}
	assertEmpty()
	snapshot, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil || snapshot.EntriesTotal != 2 || len(snapshot.Items) != 2 {
		t.Fatal("snapshot did not recover after failed transaction", err)
	}
	want := map[string]string{first.FileID: first.VersionID, second.FileID: second.VersionID}
	for _, item := range snapshot.Items {
		if item.VersionID != want[item.FileID] || item.SHA256 == "" {
			t.Fatal("fixed version or hash omitted")
		}
	}
	var count int64
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("recovery duplicated snapshot", err)
	}
	if err = f.Service.DB.Model(&service.CollectionSnapshotEntry{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("recovery left partial entries", err)
	}
}
