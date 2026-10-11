package service_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

func TestSnapshotReadCurrentAuthorityAndFixedVersion(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Snapshot read", Kind: "manual", Scope: "private"}, "snapshot-read")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: "other"}} {
		if _, err = f.Service.ReadSnapshot(other, empty.SnapshotID); err == nil {
			t.Fatal("empty snapshot bypassed current scope")
		}
	}
	op := orgUpload(t, c, h.space.ID, "allowed/original.txt", "original snapshot content")
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{op.FileID}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := c.Upload(t.Context(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "allowed/original.txt", FileID: op.FileID, ExpectedVersion: op.VersionID}, "snapshot-update", strings.NewReader("updated snapshot content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 2, Exclude: []string{op.FileID}}); err != nil {
		t.Fatal(err)
	}
	read, err := f.Service.ReadSnapshot(p, snapshot.SnapshotID)
	if err != nil || len(read.Items) != 1 || read.Items[0].VersionID != op.VersionID || read.Items[0].VersionID == updated.VersionID || read.CollectionRevision != 2 {
		t.Fatal("historical snapshot followed current collection/head", err)
	}
	// Current file readability is insufficient when its captured path is outside
	// the current Agent prefix; do not expose that historical path to the Agent.
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("path", "new-scope/moved.txt").Error; err != nil {
		t.Fatal(err)
	}
	_, restricted := agent(t, h, "new-scope", "read")
	filtered, err := restricted.ReadSnapshot(t.Context(), snapshot.SnapshotID)
	if err != nil || len(filtered.Items) != 0 || filtered.EntriesVisible != 0 {
		t.Fatal("historical path leaked to narrowed Agent", err)
	}
	if err = f.Service.DB.Where("id = ?", op.VersionID).Delete(&service.Version{}).Error; err != nil {
		t.Fatal(err)
	}
	missing, err := f.Service.ReadSnapshot(p, snapshot.SnapshotID)
	if err != nil || len(missing.Items) != 0 || missing.EntriesTotal != 1 {
		t.Fatal("missing historical version replaced or counted visible", err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadSnapshot(p, empty.SnapshotID); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled user read empty snapshot", err)
	}
}

func TestSnapshotReadFailuresNeverBecomeEmpty(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	op := orgUpload(t, c, h.space.ID, "snapshot/read.txt", "read failure fixture")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Failures", Kind: "manual", Scope: "private"}, "snapshot-errors")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{op.FileID}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collection_snapshots", "users", "file_collections", "spaces", "collection_snapshot_entries", "files", "versions"} {
		hits := 0
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("snapshot_read_error", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic snapshot read failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.ReadSnapshot(p, snapshot.SnapshotID)
		f.Service.DB.Callback().Query().Remove("snapshot_read_error")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("snapshot DB failure hidden", table, failure)
		}
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("snapshot read attempted write")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("snapshot_no_create", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("snapshot_no_update", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("snapshot_no_delete", guard)
	_, err = f.Service.ReadSnapshot(p, snapshot.SnapshotID)
	f.Service.DB.Callback().Create().Remove("snapshot_no_create")
	f.Service.DB.Callback().Update().Remove("snapshot_no_update")
	f.Service.DB.Callback().Delete().Remove("snapshot_no_delete")
	if err != nil || writes != 0 {
		t.Fatal("snapshot observation wrote state", err, writes)
	}
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Where("id = ?", snapshot.SnapshotID).Update("entries_total", 2).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadSnapshot(p, snapshot.SnapshotID); err == nil || err.Error() != "dependency_unavailable" {
		t.Fatal("partial stored snapshot shown complete", err)
	}
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Where("id = ?", snapshot.SnapshotID).Update("entries_total", 30001).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadSnapshot(p, snapshot.SnapshotID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("snapshot scan budget missing", err)
	}
}
