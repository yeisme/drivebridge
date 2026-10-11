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

func TestWorkspaceSuggestionDecisionHTTP(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	file := service.File{ID: service.ID(), Owner: alice.user.ID, SpaceID: alice.space.ID, Path: "refs/file", VersionID: "fixed-version"}
	proposal := service.WorkspaceLabelSuggestion{ID: service.ID(), Owner: alice.user.ID, AgentID: "fixture-agent", GrantID: "fixture-grant", FileID: file.ID, VersionID: file.VersionID, CreationKey: "original", RequestDigest: "fixture", Labels: []string{"keep", "new"}, State: "pending", Revision: 1}
	for _, row := range []any{&file, &proposal, &service.FileLabelAssignment{ID: service.ID(), Owner: alice.user.ID, FileID: file.ID, Namespace: "manual", Label: "keep", AuthorKind: "human"}} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	call := func(h *human, key, decision string) (int, contract.LabelSuggestionView) {
		t.Helper()
		labels := []string{"new"}
		if decision == "reject" {
			labels = []string{}
		}
		body, _ := json.Marshal(map[string]any{"expected_revision": 1, "base_version_id": file.VersionID, "decision": decision, "labels": labels})
		req, err := http.NewRequest("POST", h.base+"/api/v1/label-suggestions/"+proposal.ID+"/decision", bytes.NewReader(body))
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
		var env contract.DataEnvelope[contract.LabelSuggestionView]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	_, agentClient := agent(t, alice, "refs", "read,write")
	if err := agentClient.JSON(t.Context(), "POST", "/api/v1/label-suggestions/"+proposal.ID+"/decision", "agent-key", map[string]any{"expected_revision": 1, "base_version_id": file.VersionID, "decision": "accept", "labels": []string{"new"}}, nil); err == nil {
		t.Fatal("Agent self-approved through HTTP")
	}
	csrf := alice.csrf
	alice.csrf = ""
	missingCSRF, _ := call(alice, "csrf-key", "accept")
	alice.csrf = csrf
	if missingCSRF != 403 {
		t.Fatalf("missing CSRF status %d", missingCSRF)
	}
	if status, _ := call(bob, "key", "accept"); status != 404 {
		t.Fatalf("foreign status %d", status)
	}
	callback := "suggestion_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.suggestion.accept" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, _ := call(alice, "failed", "accept")
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if status != 503 {
		t.Fatalf("audit failure status %d", status)
	}
	var unchanged service.WorkspaceLabelSuggestion
	if err := f.Service.DB.First(&unchanged, "id = ?", proposal.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.State != "pending" || unchanged.Revision != 1 {
		t.Fatal("failed audit committed decision")
	}
	status, view := call(alice, "key", "accept")
	if status != 200 || view.State != "accepted" || view.Revision != 2 {
		t.Fatal("decision failed")
	}
	status, replay := call(alice, "key", "accept")
	if status != 200 || replay.SuggestionID != proposal.ID || replay.Revision != 2 {
		t.Fatal("decision replay failed")
	}
	if status, _ := call(alice, "key", "reject"); status != 409 {
		t.Fatalf("changed decision reused key: %d", status)
	}
	if status, _ := call(alice, "another-key", "accept"); status != 409 {
		t.Fatalf("stale decision revision status %d", status)
	}
	originalID := proposal.ID
	proposal.ID = service.ID()
	proposal.CreationKey = "second-proposal"
	proposal.Labels = []string{"must-not-add"}
	if err := f.Service.DB.Create(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	if status, rejected := call(alice, "reject-key", "reject"); status != 200 || rejected.State != "rejected" || rejected.Revision != 2 {
		t.Fatal("explicit rejection failed")
	}
	if status, rejected := call(alice, "reject-key", "reject"); status != 200 || rejected.State != "rejected" {
		t.Fatal("rejection replay failed")
	}
	proposal.ID = originalID
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", alice.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := call(alice, "key", "accept"); status != 403 {
		t.Fatalf("disabled human replay status %d", status)
	}
	var labels []service.FileLabelAssignment
	if err := f.Service.DB.Where("owner = ? AND file_id = ? AND namespace = ?", alice.user.ID, file.ID, "manual").Order("label").Find(&labels).Error; err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 || labels[0].Label != "keep" || labels[1].Label != "new" {
		t.Fatal("manual labels not merged")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ?", proposal.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("failed request or replay duplicated receipt")
	}
}
