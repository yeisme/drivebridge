package service_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestSnapshotIndexCurrentAuthorityAndHeaderOnly(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "History", Kind: "manual", Scope: "private"}, "history-source")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID)
	if err != nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal("empty history failed", err)
	}
	legacy, err := f.Service.CreateFileSnapshot(p, c.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	keyed, err := f.Service.CreateCollectionSnapshot(t.Context(), p, c.CollectionID, 2, "history-keyed")
	if err != nil {
		t.Fatal(err)
	}
	writes, forbiddenReads := 0, 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("index wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("index_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("index_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("index_no_write", guard)
	f.Service.DB.Callback().Query().Before("gorm:query").Register("index_headers_only", func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "collection_snapshot_entries", "collection_members", "files", "versions", "workspace_requests":
			forbiddenReads++
			tx.AddError(errors.New("index read members or receipts"))
		}
	})
	view, err := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID)
	f.Service.DB.Callback().Create().Remove("index_no_write")
	f.Service.DB.Callback().Update().Remove("index_no_write")
	f.Service.DB.Callback().Delete().Remove("index_no_write")
	f.Service.DB.Callback().Query().Remove("index_headers_only")
	if err != nil || writes != 0 || forbiddenReads != 0 || len(view.Items) != 2 || view.Items[0].SnapshotID != keyed.SnapshotID || view.Items[1].SnapshotID != legacy.SnapshotID || view.Items[1].CollectionRevision != 1 || view.ObservedAt.IsZero() {
		t.Fatal("header history invalid", err, writes, forbiddenReads)
	}
	raw, _ := json.Marshal(view)
	for _, field := range []string{"entries_total", "entries_visible", "request_digest", "file_id", "version_id", "path"} {
		if strings.Contains(string(raw), `"`+field+`"`) {
			t.Fatal("index exposed protected field", field)
		}
	}
	if _, err = f.Service.LookupSnapshotCreation(t.Context(), p, c.CollectionID, "legacy-no-key"); err == nil {
		t.Fatal("index manufactured legacy receipt")
	}
	a, _ := agent(t, h, "", "read,write")
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, Agent: &a}, {UserID: p.UserID, ExternalRead: true}} {
		denied, e := f.Service.ListCollectionSnapshots(t.Context(), bad, c.CollectionID)
		if e == nil || len(denied.Items) != 0 {
			t.Fatal("unauthorized history", e)
		}
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled actor read history", err)
	}
}

func TestSnapshotIndexBudgetFailuresAndGeneration(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Bounded history", Kind: "manual", Scope: "private"}, "bounded-history")
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := make([]service.CollectionSnapshot, 1001)
	for i := range rows {
		rows[i] = service.CollectionSnapshot{ID: fmt.Sprintf("%048x", i+1), CollectionID: c.CollectionID, Owner: p.UserID, Revision: 1, CreatedAt: stamp}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if view, e := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID); e == nil || e.Error() != "query_budget_exceeded" || len(view.Items) != 0 {
		t.Fatal("history silently truncated", e)
	}
	if err = f.Service.DB.Delete(&rows[1000]).Error; err != nil {
		t.Fatal(err)
	}
	view, err := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID)
	if err != nil || len(view.Items) != 1000 || view.Items[0].SnapshotID != rows[999].ID {
		t.Fatal("bounded stable tie sort", err)
	}
	// Other owners/collections must not consume this history's candidate budget.
	rows[1000].Owner = "foreign-owner"
	if err = f.Service.DB.Create(&rows[1000]).Error; err != nil {
		t.Fatal(err)
	}
	if view, err = f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID); err != nil || len(view.Items) != 1000 {
		t.Fatal("foreign candidate consumed budget", err)
	}
	for _, table := range []string{"users", "spaces", "file_collections", "collection_snapshots"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("index_dependency", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic read failure"))
			}
		})
		result, e := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID)
		f.Service.DB.Callback().Query().Remove("index_dependency")
		if hits == 0 || e == nil || e.Error() != "database_unavailable" || len(result.Items) != 0 {
			t.Fatal("dependency hidden as empty", table, e)
		}
	}
	if err = f.Service.DB.Model(&rows[0]).Update("revision", 0).Error; err != nil {
		t.Fatal(err)
	}
	if result, e := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID); e == nil || e.Error() != "dependency_unavailable" || len(result.Items) != 0 {
		t.Fatal("malformed header accepted", e)
	}
	if err = f.Service.DB.Model(&rows[0]).Update("revision", 1).Error; err != nil {
		t.Fatal(err)
	}
	changed := false
	f.Service.DB.Callback().Query().After("gorm:query").Register("index_generation", func(tx *gorm.DB) {
		if tx.Statement.Table == "collection_snapshots" && !changed {
			changed = true
			tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Model(&service.FileCollection{}).Where("id = ?", c.CollectionID).Update("revision", 2).Error)
		}
	})
	result, e := f.Service.ListCollectionSnapshots(t.Context(), p, c.CollectionID)
	f.Service.DB.Callback().Query().Remove("index_generation")
	if !changed || e == nil || e.Error() != "query_changed" || len(result.Items) != 0 {
		t.Fatal("mixed collection generation accepted", e)
	}
}
