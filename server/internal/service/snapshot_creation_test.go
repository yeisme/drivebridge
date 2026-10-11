package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestSnapshotCreationOriginalReceiptAndNoWrites(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	a := orgUpload(t, client, h.space.ID, "snapshot/a.txt", "a")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Snapshot", Kind: "manual", Scope: "private"}, "snapshot-source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{a.FileID}}); err != nil {
		t.Fatal(err)
	}
	path := "/file-collections/" + c.CollectionID + "/snapshots/creations"
	create := func(raw []byte, key string) (int, contract.SnapshotCreationReceipt) {
		t.Helper()
		req, _ := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.SnapshotCreationReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	raw := []byte(`{"expected_revision":2}`)
	status, first := create(raw, "snapshot-original")
	if status != 200 || first.SnapshotID == "" || first.CollectionID != c.CollectionID || first.CollectionRevision != 2 {
		t.Fatal("snapshot creation failed", status)
	}
	frozen, err := f.Service.ReadSnapshot(p, first.SnapshotID)
	if err != nil || len(frozen.Items) != 1 || frozen.Items[0].VersionID != a.VersionID {
		t.Fatal("not fixed source version", err)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 2}); err != nil {
		t.Fatal(err)
	}
	later, err := f.Service.CreateCollectionSnapshot(t.Context(), p, c.CollectionID, 3, "later-snapshot")
	if err != nil || later.SnapshotID == first.SnapshotID {
		t.Fatal("new intent missing", err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("lookup wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("snapshot_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("snapshot_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("snapshot_no_write", guard)
	var recovered contract.SnapshotCreationReceipt
	status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "snapshot-original"}, &recovered)
	f.Service.DB.Callback().Create().Remove("snapshot_no_write")
	f.Service.DB.Callback().Update().Remove("snapshot_no_write")
	f.Service.DB.Callback().Delete().Remove("snapshot_no_write")
	if status != 200 || writes != 0 || recovered != first {
		t.Fatal("not original read-only receipt", status, writes)
	}
	// Replays must not enumerate members or create another snapshot.
	f.Service.DB.Callback().Query().Before("gorm:query").Register("snapshot_no_enumerate", func(tx *gorm.DB) {
		if tx.Statement.Table == "collection_members" || tx.Statement.Table == "files" {
			tx.AddError(errors.New("replay enumerated source"))
		}
	})
	status, replay := create(raw, "snapshot-original")
	f.Service.DB.Callback().Query().Remove("snapshot_no_enumerate")
	if status != 200 || replay != first {
		t.Fatal("replay re-enumerated", status)
	}
	var count int64
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Where("collection_id = ?", c.CollectionID).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("replay duplicated snapshot", count, err)
	}
	if status, _ = create([]byte(`{"expected_revision":3}`), "snapshot-original"); status != 409 {
		t.Fatal("different input replayed", status)
	}
	if status, _ = create(raw, "stale-snapshot"); status != 409 {
		t.Fatal("stale intent accepted", status)
	}
	if status = bob.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "snapshot-original"}, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	if err = client.JSON(t.Context(), "POST", "/api/v1"+path, "agent-snapshot", contract.SnapshotCreationRequest{ExpectedRevision: 3}, nil); err == nil {
		t.Fatal("agent used human creation")
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.LookupSnapshotCreation(t.Context(), bad, c.CollectionID, "snapshot-original"); err == nil {
			t.Fatal("scope bypass")
		}
	}
	for _, bad := range []string{`{}`, `{"expected_revision":null}`, `{"expected_revision":0}`, `{"expected_revision":3,"expected_revision":3}`, `{"expected_revision":3,"owner":"spoof"}`} {
		if status, _ = create([]byte(bad), "bad"); status != 400 {
			t.Fatal("closed request failed", status)
		}
	}
	for _, table := range []string{"workspace_requests", "collection_snapshots", "file_collections", "users", "spaces"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("snapshot_lookup_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic dependency failure"))
			}
		})
		status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "snapshot-original"}, nil)
		f.Service.DB.Callback().Query().Remove("snapshot_lookup_fail")
		if hits == 0 || status != 500 {
			t.Fatal("dependency failure hidden", table, status)
		}
	}
}

func TestSnapshotCreationReceiptFailureRollsBackAll(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	a := orgUpload(t, mustSDK(t, h), h.space.ID, "atomic-snapshot/a.txt", "a")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic", Kind: "manual", Scope: "private"}, "atomic-snapshot-source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{a.FileID}}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collection_snapshots", "collection_snapshot_entries", "workspace_requests"} {
		hits := 0
		f.Service.DB.Callback().Create().Before("gorm:create").Register("snapshot_receipt_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic publication failure"))
			}
		})
		_, failure := f.Service.CreateCollectionSnapshot(t.Context(), p, c.CollectionID, 2, "atomic-snapshot")
		f.Service.DB.Callback().Create().Remove("snapshot_receipt_fail")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("publication failure ignored", table, failure)
		}
		var count int64
		for _, model := range []any{&service.CollectionSnapshot{}, &service.CollectionSnapshotEntry{}} {
			if err = f.Service.DB.Model(model).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("partial snapshot left", table, count, err)
			}
		}
		if _, err = f.Service.LookupSnapshotCreation(t.Context(), p, c.CollectionID, "atomic-snapshot"); err == nil || err.Error() != "not_found" {
			t.Fatal("failed publication receipt exists", err)
		}
	}
	first, err := f.Service.CreateCollectionSnapshot(t.Context(), p, c.CollectionID, 2, "atomic-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Where("action = ? AND resource_id = ?", "collection.snapshot", first.SnapshotID).Delete(&service.WorkspaceRequest{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.LookupSnapshotCreation(t.Context(), p, c.CollectionID, "atomic-snapshot"); err == nil || err.Error() != "not_found" {
		t.Fatal("current snapshot fabricated original receipt", err)
	}
}
