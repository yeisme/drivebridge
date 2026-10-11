package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestWorkspaceSuggestionAuthenticatedFlow(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	file := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: "refs/file", VersionID: service.ID()}
	version := service.Version{ID: file.VersionID, Owner: h.user.ID, FileID: file.ID, ConnectionID: h.space.ConnectionID}
	for _, row := range []any{&file, &version} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	humanPOST := func(path, key string, body any, out any) int {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewReader(data))
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
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode < 300 && out != nil {
			if err = json.Unmarshal(env.Data, out); err != nil {
				t.Fatal(err)
			}
		}
		return res.StatusCode
	}
	var issued contract.GrantIssuedView
	if status := humanPOST("/agent-grants", "create", map[string]any{"name": "Suggest only", "protocol": "http", "space_id": h.space.ID, "connection_id": h.space.ConnectionID, "prefix": "refs", "actions": []string{"labels.suggest"}}, &issued); status != 201 {
		t.Fatalf("create status %d", status)
	}
	client, err := sdk.New(h.base, issued.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	outside := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: "private/file", VersionID: service.ID()}
	outsideVersion := service.Version{ID: outside.VersionID, Owner: h.user.ID, FileID: outside.ID, ConnectionID: h.space.ConnectionID}
	for _, row := range []any{&outside, &outsideVersion} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/files/"+outside.ID+"/label-suggestions", "outside", contract.LabelSuggestionRequest{VersionID: outside.VersionID, Labels: []string{"bad"}}, nil); err == nil {
		t.Fatal("out-of-prefix suggestion accepted")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/uploads", "wide-upload", map[string]any{"space_id": h.space.ID, "path": "refs/new", "size_bytes": 0}, nil); err == nil {
		t.Fatal("suggest-only grant reached legacy upload")
	}
	bob, _ := login(t, f, "bob")
	foreign := service.File{ID: service.ID(), Owner: bob.user.ID, SpaceID: bob.space.ID, Path: "refs/file", VersionID: service.ID()}
	foreignVersion := service.Version{ID: foreign.VersionID, Owner: bob.user.ID, FileID: foreign.ID, ConnectionID: bob.space.ConnectionID}
	for _, row := range []any{&foreign, &foreignVersion} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/files/"+foreign.ID+"/label-suggestions", "foreign-file", contract.LabelSuggestionRequest{VersionID: foreign.VersionID, Labels: []string{"bad"}}, nil); err == nil {
		t.Fatal("cross-user suggestion accepted")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "foreign-version", contract.LabelSuggestionRequest{VersionID: foreign.VersionID, Labels: []string{"bad"}}, nil); err == nil {
		t.Fatal("foreign version attached to owned file")
	}
	input := contract.LabelSuggestionRequest{VersionID: version.ID, Labels: []string{"reference"}}
	callback := "workspace_submit_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.suggestion.submit" {
			tx.AddError(errors.New("fixture audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed := client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "failed-submit", input, nil)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatal("submission audit failure did not fail closed")
	}
	var rolledBack int64
	if err := f.Service.DB.Model(&service.WorkspaceLabelSuggestion{}).Where("creation_key = ?", "failed-submit").Count(&rolledBack).Error; err != nil {
		t.Fatal(err)
	}
	if rolledBack != 0 {
		t.Fatal("failed submission left proposal")
	}
	var proposal contract.LabelSuggestionView
	if err = client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "suggest", input, &proposal); err != nil {
		t.Fatalf("dedicated submission failed: %v", err)
	}
	if proposal.AgentID != issued.Grant.AgentID || proposal.State != "pending" {
		t.Fatal("proposal provenance mismatch")
	}
	var replay contract.LabelSuggestionView
	if err = client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "suggest", input, &replay); err != nil || replay.SuggestionID != proposal.SuggestionID {
		t.Fatal("submit replay failed")
	}
	if err = client.JSON(t.Context(), "PUT", "/api/v1/files/"+file.ID+"/labels", "wide", map[string]any{"namespace": "manual", "labels": []string{"bad"}}, nil); err == nil {
		t.Fatal("suggest grant inherited legacy write")
	}
	if err = client.JSON(t.Context(), "GET", "/api/v1/files/"+file.ID+"/content?version="+version.ID, "", nil, nil); err == nil {
		t.Fatal("suggest grant inherited content read")
	}
	var second contract.GrantIssuedView
	if status := humanPOST("/agent-grants", "create-second", map[string]any{"name": "Second suggest", "protocol": "http", "space_id": h.space.ID, "connection_id": h.space.ConnectionID, "prefix": "refs", "actions": []string{"labels.suggest"}}, &second); status != 201 {
		t.Fatalf("second grant status %d", status)
	}
	secondClient, err := sdk.New(h.base, second.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	var secondProposal contract.LabelSuggestionView
	secondInput := contract.LabelSuggestionRequest{VersionID: version.ID, Labels: []string{"second-label"}}
	if err := secondClient.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "suggest", secondInput, &secondProposal); err != nil {
		t.Fatalf("second submission failed: %v", err)
	}
	if secondProposal.SuggestionID == proposal.SuggestionID || secondProposal.AgentID != second.Grant.AgentID {
		t.Fatal("agents shared proposal identity")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "suggest", secondInput, nil); err == nil {
		t.Fatal("changed payload overwrote first proposal")
	}
	listPath := "/api/v1/files/" + file.ID + "/label-suggestions?version_id=" + version.ID
	var own contract.Page[contract.LabelSuggestionView]
	if err := client.JSON(t.Context(), "GET", listPath, "", nil, &own); err != nil || len(own.Items) != 1 || own.Items[0].SuggestionID != proposal.SuggestionID {
		t.Fatal("Agent suggestion list crossed source identity")
	}
	if err := secondClient.JSON(t.Context(), "GET", listPath, "", nil, &own); err != nil || len(own.Items) != 1 || own.Items[0].SuggestionID != secondProposal.SuggestionID {
		t.Fatal("second Agent list crossed source identity")
	}
	var visible contract.Page[contract.LabelSuggestionView]
	if status := h.call(t, "GET", "/files/"+file.ID+"/label-suggestions?version_id="+version.ID, nil, &visible); status != 200 || len(visible.Items) != 2 {
		t.Fatal("human cannot read both suggestion sources")
	}
	if status := bob.call(t, "GET", "/files/"+file.ID+"/label-suggestions?version_id="+version.ID, nil, nil); status != 404 {
		t.Fatalf("foreign suggestion list status %d", status)
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files/"+file.ID+"/label-suggestions", "", nil, nil); err == nil {
		t.Fatal("missing fixed version accepted")
	}
	humanList := "/files/" + file.ID + "/label-suggestions?version_id=" + version.ID + "&limit=1"
	var firstPage, secondPage contract.Page[contract.LabelSuggestionView]
	if status := h.call(t, "GET", humanList, nil, &firstPage); status != 200 || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatal("first suggestion page failed")
	}
	nextPage := humanList + "&cursor=" + url.QueryEscape(firstPage.NextCursor)
	if status := h.call(t, "GET", nextPage, nil, &secondPage); status != 200 || len(secondPage.Items) != 1 || secondPage.NextCursor != "" || firstPage.Items[0].SuggestionID == secondPage.Items[0].SuggestionID {
		t.Fatal("suggestion pagination duplicated or omitted source")
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1"+nextPage, "", nil, nil); err == nil || err.Error() != "cursor_expired" {
		t.Fatal("Agent reused human suggestion cursor")
	}
	var firstStored service.WorkspaceLabelSuggestion
	if err := f.Service.DB.First(&firstStored, "id = ?", proposal.SuggestionID).Error; err != nil {
		t.Fatal(err)
	}
	if len(firstStored.Labels) != 1 || firstStored.Labels[0] != "reference" {
		t.Fatal("second Agent changed first labels")
	}
	decision := contract.SuggestionDecisionRequest{ExpectedRevision: 1, BaseVersionID: version.ID, Decision: "accept", Labels: []string{"reference"}}
	if err = client.JSON(t.Context(), "POST", "/api/v1/label-suggestions/"+proposal.SuggestionID+"/decision", "self", decision, nil); err == nil {
		t.Fatal("Agent self-approved")
	}
	var accepted contract.LabelSuggestionView
	if status := humanPOST("/label-suggestions/"+proposal.SuggestionID+"/decision", "accept", decision, &accepted); status != 200 || accepted.State != "accepted" {
		t.Fatal("human decision failed")
	}
	if status := h.call(t, "GET", nextPage, nil, nil); status != 410 {
		t.Fatalf("decision retained stale suggestion cursor: %d", status)
	}
	if status := humanPOST("/agent-grants/"+issued.Grant.GrantID+"/revoke", "revoke", contract.RevisionRequest{ExpectedRevision: 1}, nil); status != 200 {
		t.Fatalf("revoke status %d", status)
	}
	if err = client.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "after-revoke", input, nil); err == nil {
		t.Fatal("revoked grant submitted")
	}
	if err := client.JSON(t.Context(), "GET", listPath, "", nil, nil); err == nil {
		t.Fatal("revoked Agent listed suggestions")
	}
	var secondReplay contract.LabelSuggestionView
	if err := secondClient.JSON(t.Context(), "POST", "/api/v1/files/"+file.ID+"/label-suggestions", "suggest", secondInput, &secondReplay); err != nil || secondReplay.SuggestionID != secondProposal.SuggestionID {
		t.Fatal("revoking first Agent affected second Agent")
	}
	var labels []service.FileLabelAssignment
	if err = f.Service.DB.Where("owner = ? AND file_id = ? AND namespace = ?", h.user.ID, file.ID, "manual").Find(&labels).Error; err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 || labels[0].Label != "reference" || labels[0].AcceptedFrom != proposal.SuggestionID {
		t.Fatal("accepted result not persisted")
	}
}
