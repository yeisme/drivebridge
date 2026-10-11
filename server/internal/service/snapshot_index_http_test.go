package service_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestSnapshotIndexHTTPClosedReadOnly(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "HTTP history", Kind: "manual", Scope: "private"}, "http-history")
	if err != nil {
		t.Fatal(err)
	}
	path := "/file-collections/" + c.CollectionID + "/snapshots"
	get := func(suffix string) (int, contract.SnapshotIndex) {
		t.Helper()
		req, _ := http.NewRequest("GET", h.base+"/api/v1"+path+suffix, nil)
		res, e := h.http.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("history cacheable")
		}
		var env contract.DataEnvelope[contract.SnapshotIndex]
		if e = json.NewDecoder(res.Body).Decode(&env); e != nil {
			t.Fatal(e)
		}
		return res.StatusCode, env.Data
	}
	if status, view := get(""); status != 200 || view.Items == nil || len(view.Items) != 0 {
		t.Fatal("empty history not authorized", status)
	}
	var legacy service.SnapshotView
	if status := h.call(t, "POST", path, map[string]any{}, &legacy); status != 200 {
		t.Fatal("legacy POST broken", status)
	}
	keyed, err := f.Service.CreateCollectionSnapshot(t.Context(), p, c.CollectionID, 1, "history-http-key")
	if err != nil {
		t.Fatal(err)
	}
	writes, forbiddenReads := 0, 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("HTTP index wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("http_index_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("http_index_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("http_index_write", guard)
	f.Service.DB.Callback().Query().Before("gorm:query").Register("http_index_headers", func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "collection_snapshot_entries", "collection_members", "files", "versions", "workspace_requests":
			forbiddenReads++
			tx.AddError(errors.New("index expanded members"))
		}
	})
	status, view := get("")
	f.Service.DB.Callback().Create().Remove("http_index_write")
	f.Service.DB.Callback().Update().Remove("http_index_write")
	f.Service.DB.Callback().Delete().Remove("http_index_write")
	f.Service.DB.Callback().Query().Remove("http_index_headers")
	if status != 200 || writes != 0 || forbiddenReads != 0 || len(view.Items) != 2 || view.Items[0].SnapshotID != keyed.SnapshotID || view.Items[1].SnapshotID != legacy.SnapshotID {
		t.Fatal("header-only HTTP failed", status, writes, forbiddenReads)
	}
	for _, suffix := range []string{"?", "?limit=1", "?cursor=old", "?space_id=" + h.space.ID, "?limit=1&limit=2", "?%zz"} {
		if status, _ := get(suffix); status != 400 {
			t.Fatal("query not closed", suffix, status)
		}
	}
	if status := bob.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatal("foreign history visible", status)
	}
	if err = mustSDK(t, h).JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("Agent read human history")
	}
	for _, table := range []string{"spaces", "file_collections", "collection_snapshots"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("http_index_fault", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic index fault"))
			}
		})
		status, _ := get("")
		f.Service.DB.Callback().Query().Remove("http_index_fault")
		if hits == 0 || status != 500 {
			t.Fatal("query failure became empty", table, status)
		}
	}
	rows := make([]service.CollectionSnapshot, 999)
	for i := range rows {
		rows[i] = service.CollectionSnapshot{ID: fmt.Sprintf("%048x", i+1), CollectionID: c.CollectionID, Owner: p.UserID, Revision: 1, CreatedAt: time.Now().UTC()}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if status, view := get(""); status != 429 || len(view.Items) != 0 {
		t.Fatal("over-budget history not refused", status)
	}
}
