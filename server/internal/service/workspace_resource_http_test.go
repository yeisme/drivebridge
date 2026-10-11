package service_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceResourceHTTP(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	op := orgUpload(t, c, h.space.ID, "resource/http.txt", "resource HTTP fixture")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	input := contract.RegisterResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "template", DisplayName: "Template"}
	create := func(raw []byte, key string) (int, contract.ResourceView) {
		t.Helper()
		req, _ := http.NewRequest("POST", h.base+"/api/v1/resource-descriptors", bytes.NewReader(raw))
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.ResourceView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("resource response cacheable")
		}
		return res.StatusCode, env.Data
	}
	raw, _ := json.Marshal(input)
	status, first := create(raw, "http-resource")
	if status != 201 || first.FileRef != input.FileRef || first.CheckState != "unknown" {
		t.Fatal("create HTTP failed", status)
	}
	status, replay := create(raw, "http-resource")
	if status != 201 || first != replay {
		t.Fatal("replay drift", status)
	}
	secondInput := input
	secondInput.DisplayName = "Second"
	secondRaw, _ := json.Marshal(secondInput)
	if status, _ := create(secondRaw, "http-resource-2"); status != 201 {
		t.Fatal(status)
	}
	for _, bad := range [][]byte{[]byte(`{"space_id":null}`), append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"kind":"skill"}`)...), append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"owner":"spoof"}`)...)} {
		if status, _ := create(bad, "bad"); status != 400 {
			t.Fatal("closed request accepted", status)
		}
	}
	if status, _ := create(raw, ""); status != 400 {
		t.Fatal("missing key accepted")
	}
	var page contract.Page[contract.ResourceView]
	if status := h.call(t, "GET", "/resource-descriptors?limit=1", nil, &page); status != 200 || len(page.Items) != 1 || page.NextCursor == "" || page.Total != nil {
		t.Fatal("list mismatch", status)
	}
	next := "/resource-descriptors?limit=1&cursor=" + url.QueryEscape(page.NextCursor)
	if status := h.call(t, "GET", next, nil, nil); status != 200 {
		t.Fatal("stable page", status)
	}
	if status := bob.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("cross owner cursor", status)
	}
	if status := h.call(t, "GET", next+"&kind=template", nil, nil); status != 410 {
		t.Fatal("query binding lost", status)
	}
	if status := bob.call(t, "GET", "/resource-descriptors/"+first.DescriptorID, nil, nil); status != 404 {
		t.Fatal("foreign detail leaked", status)
	}
	for _, query := range []string{"limit=1&limit=2", "kind=executable", "space_id=%zz", "unknown=1"} {
		if status := h.call(t, "GET", "/resource-descriptors?"+query, nil, nil); status != 400 {
			t.Fatal("bad query", status)
		}
	}
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", next, nil, nil); status != 410 {
		t.Fatal("visibility change cursor", status)
	}
	if status := h.call(t, "GET", "/resource-descriptors/"+first.DescriptorID, nil, nil); status != 404 {
		t.Fatal("trashed result read", status)
	}
	if status := h.call(t, "GET", "/resource-descriptors", nil, &page); status != 200 || len(page.Items) != 0 {
		t.Fatal("hidden items leaked", status)
	}
}

func TestWorkspaceResourceListCumulativeBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, c, h.space.ID, "pack/entry.txt", "pack budget fixture")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Budget pack", Kind: "manual", Scope: "private"}, "pack-budget")
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
	// 501 descriptors, each requiring two fixed target validations. No single
	// pack exceeds its budget; the combined page request must still stop at 1000.
	var entry service.CollectionSnapshotEntry
	if err = f.Service.DB.Where("snapshot_id = ?", snapshot.SnapshotID).First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	other := orgUpload(t, c, h.space.ID, "pack/second.txt", "second budget fixture")
	entry.ID = service.ID()
	entry.FileID = other.FileID
	entry.VersionID = other.VersionID
	entry.Path = "pack/second.txt"
	if err = f.Service.DB.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Where("id = ?", snapshot.SnapshotID).Updates(map[string]any{"entries_total": 2, "entries_visible": 2}).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]service.WorkspaceResourceDescriptor, 501)
	for i := range rows {
		rows[i] = service.WorkspaceResourceDescriptor{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, SnapshotID: &snapshot.SnapshotID, Kind: "reference_pack", DisplayName: fmt.Sprint(i), Revision: 1}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/resource-descriptors?limit=1", nil, nil); status != 429 {
		t.Fatal("cumulative target budget missing", status)
	}
	if err = f.Service.DB.Migrator().DropTable(&service.WorkspaceResourceDescriptor{}); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/resource-descriptors", nil, nil); status != 500 {
		t.Fatal("DB failure became empty", status)
	}
}
