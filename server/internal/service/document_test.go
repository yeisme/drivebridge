package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func documentCall(t *testing.T, h *human, id, method, suffix, key string, body any) (int, json.RawMessage) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, h.base+"/api/v1/files/"+id+suffix, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Content-Type", "application/json")
	res, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error any             `json:"error"`
	}
	if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, envelope.Data
}

func TestDocumentSaveHTTPVersionsReplayAndIsolation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, client := agent(t, alice, "notes", "read,write")
	op, err := client.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "notes/a.md"}, "document-original", strings.NewReader("# Original"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var capability contract.DocumentCapability
	if status := alice.call(t, "GET", "/files/"+op.FileID+"/document-capability", nil, &capability); status != 200 || !capability.Editable {
		t.Fatal("capability unavailable", status)
	}
	input := contract.SaveDocumentRequest{BaseVersionID: op.VersionID, Text: "# Updated\n\n中文原文\n"}
	status, raw := documentCall(t, alice, op.FileID, "PUT", "/document", "edit-one", input)
	if status != 200 {
		t.Fatalf("save status %d", status)
	}
	var saved contract.DocumentSaveView
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Status != "completed" || saved.VersionID == op.VersionID || saved.FileRef == "" {
		t.Fatal("save did not produce a fixed new version")
	}
	status, replay := documentCall(t, alice, op.FileID, "PUT", "/document", "edit-one", input)
	if status != 200 || string(raw) != string(replay) {
		t.Fatal("duplicate edit did not replay original receipt")
	}
	status, lookup := documentCall(t, alice, op.FileID, "POST", "/document/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "edit-one"})
	var found contract.DocumentLookupView
	if json.Unmarshal(lookup, &found) != nil || status != 200 || !found.Found || found.Save.OperationID != saved.OperationID {
		t.Fatal("lost-response lookup failed")
	}
	for version, expected := range map[string]string{op.VersionID: "# Original", saved.VersionID: input.Text} {
		res, err := alice.http.Get(alice.base + "/api/v1/files/" + op.FileID + "/content?version=" + version)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || string(body) != expected {
			t.Fatal("fixed content changed")
		}
	}
	if status, _ := documentCall(t, bob, op.FileID, "PUT", "/document", "edit-one", input); status != 404 {
		t.Fatalf("cross-user save %d", status)
	}
	if status, _ := documentCall(t, bob, op.FileID, "POST", "/document/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "edit-one"}); status != 404 {
		t.Fatalf("cross-user lookup %d", status)
	}
	input.Text = "different"
	if status, _ := documentCall(t, alice, op.FileID, "PUT", "/document", "edit-one", input); status != 409 {
		t.Fatalf("idempotency conflict %d", status)
	}
	if status, _ := documentCall(t, alice, op.FileID, "PUT", "/document", "edit-two", input); status != 409 {
		t.Fatalf("base version conflict %d", status)
	}
	for _, invalid := range []any{map[string]any{"base_version_id": saved.VersionID}, map[string]any{"base_version_id": saved.VersionID, "text": nil}, map[string]any{"base_version_id": saved.VersionID, "text": "ok", "actor": "other"}, contract.SaveDocumentRequest{BaseVersionID: saved.VersionID, Text: strings.Repeat("中", contract.DocumentMaxBytes/3+1)}} {
		if status, _ := documentCall(t, alice, op.FileID, "PUT", "/document", "invalid", invalid); status != 400 {
			t.Fatalf("invalid save %d", status)
		}
	}
	input.BaseVersionID, input.Text = saved.VersionID, ""
	if status, _ := documentCall(t, alice, op.FileID, "PUT", "/document", "empty", input); status != 200 {
		t.Fatalf("empty document %d", status)
	}
	// Direct service tests switch this flag only after all requests settled.
	f.Service.Config.DocumentEditing = false
	if capability, err := f.Service.DocumentCapability(service.Principal{UserID: alice.user.ID}, op.FileID); err != nil || capability.Editable {
		t.Fatal("disabled editor remained writable")
	}
	if _, err := f.Service.SaveDocument(context.Background(), service.Principal{UserID: alice.user.ID}, op.FileID, input, "disabled"); err == nil {
		t.Fatal("disabled save accepted")
	}
}

func TestDocumentConcurrentCASAndHumanGate(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "notes", "read,write")
	op, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "notes/concurrent.md"}, "original", strings.NewReader("start"), nil)
	if err != nil {
		t.Fatal(err)
	}
	p := service.Principal{UserID: h.user.ID}
	if _, err = f.Service.SaveDocument(context.Background(), service.Principal{UserID: h.user.ID, Agent: &a}, op.FileID, contract.SaveDocumentRequest{BaseVersionID: op.VersionID, Text: "denied"}, "agent"); err == nil {
		t.Fatal("Agent used human editor")
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, text := range []string{"one", "two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.Service.SaveDocument(context.Background(), p, op.FileID, contract.SaveDocumentRequest{BaseVersionID: op.VersionID, Text: text}, text)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent saves succeeded %d times", success)
	}
	var space service.Space
	if err := f.Service.DB.First(&space, "id = ?", h.space.ID).Error; err != nil || space.Reserved != 0 {
		t.Fatal("conflicting document save retained quota reservation")
	}
	var versions int64
	if err := f.Service.DB.Model(&service.Version{}).Where("file_id = ?", op.FileID).Count(&versions).Error; err != nil || versions != 2 {
		t.Fatal("CAS created extra versions")
	}
}
