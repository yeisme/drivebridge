package service_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceHomePublishedFilesAndIsolation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "real.txt", Size: 4, SHA256: service.Hash("body")}, "home-upload")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString("body")); err != nil {
		t.Fatal(err)
	}
	done, err := f.Service.Complete(t.Context(), p, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Reading", Kind: "manual", Scope: "private"}, "home-collection")
	if err != nil {
		t.Fatal(err)
	}
	path := "/workspace/home?space_id=" + h.space.ID
	var home contract.WorkspaceHome
	if status := h.call(t, "GET", path, nil, &home); status != 200 {
		t.Fatalf("home status %d", status)
	}
	if home.Sections.RecentFiles.State != "ready" || len(home.RecentFiles) != 1 || home.RecentFiles[0].FileID != done.FileID || home.RecentFiles[0].VersionID != done.VersionID || !strings.HasSuffix(home.RecentFiles[0].Ref, "/version/"+done.VersionID) {
		t.Fatalf("fixed result missing: %+v", home.Sections)
	}
	if home.Sections.Collections.State != "ready" || len(home.Collections) != 1 || home.Collections[0].CollectionID != collection.CollectionID {
		t.Fatal("canonical collection missing")
	}
	if home.Sections.PendingCounts.State != "partial" || home.Sections.PendingCounts.Reason != "submission_states_only" || home.PendingCounts.Submitted == nil || *home.PendingCounts.Submitted != 0 {
		t.Fatal("invented global pending count")
	}
	if home.Sections.Usage.State != "ready" || home.Usage.UsedBytes == nil || *home.Usage.UsedBytes != 4 || home.Usage.ReservedBytes == nil || *home.Usage.ReservedBytes != 0 {
		t.Fatal("logical capacity missing")
	}
	if status := bob.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatalf("foreign home %d", status)
	}
	_, client := agent(t, h, "", "read,write")
	if err := client.JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("agent admitted to human home")
	}
	for _, query := range []string{"", "space_id=", "space_id=" + h.space.ID + "&recent_limit=0", "space_id=" + h.space.ID + "&recent_limit=21", "space_id=" + h.space.ID + "&recent_limit=1&recent_limit=2", "space_id=" + h.space.ID + "&extra=1"} {
		if status := h.call(t, "GET", "/workspace/home?"+query, nil, nil); status != 400 {
			t.Fatalf("invalid query status %d", status)
		}
	}
	if err = f.Service.DB.Delete(&service.Version{}, "id = ?", done.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", path, nil, &home); status != 200 || home.Sections.RecentFiles.State != "partial" || len(home.RecentFiles) != 0 {
		t.Fatal("missing fixed version replaced with head or empty ready")
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", path, nil, nil); status != 403 {
		t.Fatalf("disabled user %d", status)
	}
}

func TestWorkspaceHomeSectionFailures(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	path := "/workspace/home?space_id=" + h.space.ID
	for _, table := range []string{"files", "file_collections", "workspace_submissions"} {
		t.Run(table, func(t *testing.T) {
			name := "home_section_failure"
			hits := 0
			if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					hits++
					tx.AddError(errors.New("synthetic home section failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			var home contract.WorkspaceHome
			status := h.call(t, "GET", path, nil, &home)
			if err := f.Service.DB.Callback().Query().Remove(name); err != nil {
				t.Fatal(err)
			}
			if hits == 0 || status != 200 {
				t.Fatal("section failure lost whole response")
			}
			if home.Sections.Usage.State != "ready" || home.Usage.UsedBytes == nil {
				t.Fatal("healthy usage swallowed")
			}
			switch table {
			case "files":
				if home.Sections.RecentFiles.State != "unavailable" || home.Sections.Collections.State != "ready" {
					t.Fatal("recent failure not isolated")
				}
			case "file_collections":
				if home.Sections.Collections.State != "unavailable" || home.Sections.RecentFiles.State != "ready" {
					t.Fatal("collection failure not isolated")
				}
			case "workspace_submissions":
				if home.Sections.PendingCounts.State != "unavailable" || home.PendingCounts.Submitted != nil || home.PendingCounts.Accepted != nil {
					t.Fatal("failed pending became zero")
				}
			}
		})
	}
	if err := f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("used", -1).Error; err != nil {
		t.Fatal(err)
	}
	var home contract.WorkspaceHome
	if status := h.call(t, "GET", path, nil, &home); status != 200 || home.Sections.Usage.State != "unavailable" || home.Usage.UsedBytes != nil {
		t.Fatal("bad usage not unknown")
	}
}

func TestWorkspaceHomeBoundedSources(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	now := time.Now().UTC()
	rows := make([]service.File, 1001)
	for i := range rows {
		rows[i] = service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: fmt.Sprintf("file-%d", i), Updated: now}
	}
	if err := f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	collections := make([]service.FileCollection, 21)
	for i := range collections {
		collections[i] = service.FileCollection{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Name: fmt.Sprintf("c-%d", i), Kind: "manual", Scope: "private", Revision: 1}
	}
	if err := f.Service.DB.Create(&collections).Error; err != nil {
		t.Fatal(err)
	}
	subs := make([]service.WorkspaceSubmission, 1001)
	for i := range subs {
		subs[i] = service.WorkspaceSubmission{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, GrantID: "fixture", CreationKey: fmt.Sprintf("s-%d", i), UploadOperationID: service.ID(), State: "submitted"}
	}
	if err := f.Service.DB.CreateInBatches(subs, 100).Error; err != nil {
		t.Fatal(err)
	}
	var home contract.WorkspaceHome
	if status := h.call(t, "GET", "/workspace/home?space_id="+h.space.ID, nil, &home); status != 200 || home.Sections.RecentFiles.State != "unavailable" || home.Sections.RecentFiles.Reason != "scan_budget_exceeded" || len(home.RecentFiles) != 0 || home.Sections.Usage.State != "ready" {
		t.Fatal("scan budget became fabricated recent list")
	}
	if home.Sections.Collections.State != "partial" || home.Sections.Collections.Reason != "collection_limit" || len(home.Collections) != 20 {
		t.Fatal("collection truncation not disclosed")
	}
	if home.Sections.PendingCounts.State != "unavailable" || home.Sections.PendingCounts.Reason != "scan_budget_exceeded" || home.PendingCounts.Submitted != nil {
		t.Fatal("submission budget became zero")
	}
}
