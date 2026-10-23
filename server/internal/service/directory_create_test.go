package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestDirectoryCreateHTTPReceiptLookupAndCompatibility(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	call := func(key, path string) (int, contract.DirectoryCreation) {
		t.Helper()
		body, _ := json.Marshal(contract.DirectoryCreate{SpaceID: h.space.ID, Path: path})
		req, err := http.NewRequest(http.MethodPost, h.base+"/api/v1/directories", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope contract.DataEnvelope[contract.DirectoryCreation]
		if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, envelope.Data
	}
	status, first := call("directory-create", "notes")
	if status != http.StatusCreated || first.Directory == nil || first.Directory.Path != "notes" {
		t.Fatalf("first create status=%d data=%+v", status, first)
	}
	status, replay := call("directory-create", "notes")
	if status != http.StatusCreated || replay.Directory == nil || replay.Directory.ID != first.Directory.ID {
		t.Fatalf("replay status=%d data=%+v", status, replay)
	}
	if status, _ = call("directory-create", "other"); status != http.StatusConflict {
		t.Fatalf("changed request key status %d", status)
	}
	if status, _ = call("directory-create-other", "notes"); status != http.StatusConflict {
		t.Fatalf("duplicate path status %d", status)
	}
	body, _ := json.Marshal(contract.DirectoryCreateLookup{SpaceID: h.space.ID, IdempotencyKey: "directory-create"})
	req, _ := http.NewRequest(http.MethodPost, h.base+"/api/v1/directories/lookup", bytes.NewReader(body))
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Content-Type", "application/json")
	res, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var envelope contract.DataEnvelope[contract.DirectoryCreation]
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || envelope.Data.Directory == nil || envelope.Data.Directory.ID != first.Directory.ID {
		t.Fatalf("lookup status=%d data=%+v", res.StatusCode, envelope.Data)
	}
	var legacy service.Directory
	if status := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "legacy"}, &legacy); status != http.StatusOK || legacy.Path != "legacy" {
		t.Fatalf("legacy create status=%d row=%+v", status, legacy)
	}
}

func TestNotebookCreationUsesPhysicalDirectoryAndDailyIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	if status := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "notes"}, nil); status != http.StatusOK {
		t.Fatal("create notes directory", status)
	}
	p := service.Principal{UserID: h.user.ID}
	first, err := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, Title: "Hello", DirectoryPath: "notes", Icon: "file"}, "note-physical")
	if err != nil || first.Page == nil || first.Page.Path != "notes/Hello.md" {
		t.Fatalf("physical note %+v %v", first, err)
	}
	replay, err := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, Title: "Hello", DirectoryPath: "notes", Icon: "file"}, "note-physical")
	if err != nil || replay.Page == nil || replay.Page.FileID != first.Page.FileID {
		t.Fatalf("physical replay %+v %v", replay, err)
	}
	var listing struct {
		Entries []service.BrowserEntry `json:"entries"`
	}
	if status := h.call(t, "GET", "/browser?space_id="+h.space.ID+"&path=notes", nil, &listing); status != http.StatusOK || len(listing.Entries) != 1 || listing.Entries[0].Path != "notes/Hello.md" {
		t.Fatalf("physical browser read status=%d entries=%+v", status, listing.Entries)
	}
	if _, err = f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, Title: "Hello", DirectoryPath: "notes", Icon: "file"}, "note-other"); err == nil || err.Error() != "name_conflict" {
		t.Fatalf("duplicate physical note error %v", err)
	}
	if _, err = f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, Title: "bad/name", DirectoryPath: "notes", Icon: "file"}, "note-bad"); err == nil || err.Error() != "invalid_request" {
		t.Fatalf("invalid physical title error %v", err)
	}
	if status := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "journal"}, nil); status != http.StatusOK {
		t.Fatal("create journal directory", status)
	}
	dailyA, err := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, DailyDate: "2026-10-18", DirectoryPath: "notes"}, "daily-a")
	if err != nil || dailyA.Page == nil || dailyA.Page.Path != "notes/2026-10-18.md" {
		t.Fatalf("daily notes %+v %v", dailyA, err)
	}
	dailyB, err := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, contract.NotebookCreate{SpaceID: h.space.ID, DailyDate: "2026-10-18", DirectoryPath: "journal"}, "daily-b")
	if err != nil || dailyB.Page == nil || dailyB.Page.Path != "journal/2026-10-18.md" {
		t.Fatalf("daily journal %+v %v", dailyB, err)
	}
}
