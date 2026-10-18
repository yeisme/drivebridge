package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestWorkspaceHoldHTTPAndSDK(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	op := fpUploadBytes(t, c, h.space.ID, "held.txt", "upload", []byte("held"))
	q := sdk.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	base := "/files/" + op.FileID + "/retention-holds"
	post := func(path string, input any, key string) (int, contract.WorkspaceHoldReceipt) {
		t.Helper()
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.WorkspaceHoldReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	if status, _ := post(base, q, "off"); status != 422 {
		t.Fatal("default-off human endpoint", status)
	}
	f.Service.Config.HoldManagement = true
	// mustSDK is an Agent bearer, not the logged-in human. It must not
	// inherit the human's new management capability through this endpoint.
	if _, err := c.CreateWorkspaceHold(t.Context(), op.FileID, q, "agent"); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent create", err)
	}
	status, first := post(base, q, "create")
	if status != 200 || first.ResultRevision != 1 {
		t.Fatal(first, status)
	}
	if status, _ = post(base, q, "second"); status != 200 {
		t.Fatal(status)
	}
	var page contract.WorkspaceHoldList
	if status = h.call(t, "GET", base+"?version_id="+op.VersionID+"&limit=1", nil, &page); status != 200 || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal(page, status)
	}
	if _, err := c.ListWorkspaceHolds(t.Context(), op.FileID, op.VersionID, "", 1); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent list", err)
	}
	if _, err := c.ReleaseWorkspaceHold(t.Context(), first.HoldID, sdk.WorkspaceHoldRelease{ExpectedRevision: 1}, "agent-release"); err == nil || err.Error() != "forbidden" {
		t.Fatal("SDK Agent release", err)
	}
	status, released := post("/retention/holds/"+first.HoldID+"/release-cas", contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	if status != 200 || released.ResultRevision != 2 {
		t.Fatal(released, status)
	}
	if status := h.call(t, "GET", base+"?version_id="+op.VersionID+"&limit=1&cursor="+url.QueryEscape(page.NextCursor), nil, nil); status != 410 {
		t.Fatal("cursor status", status)
	}
	f.Service.Config.HoldManagement = false
	for action, expected := range map[string]contract.WorkspaceHoldReceipt{"create": first, "release": released} {
		var got contract.WorkspaceHoldReceipt
		if status = h.call(t, "POST", base+"/lookup", contract.WorkspaceHoldLookup{Action: action, IdempotencyKey: action}, &got); status != 200 || got != expected {
			t.Fatal("human recovery", got, status)
		}
		if _, err := c.LookupWorkspaceHold(t.Context(), op.FileID, sdk.WorkspaceHoldLookup{Action: action, IdempotencyKey: action}); err == nil || err.Error() != "forbidden" {
			t.Fatal("SDK Agent recovery", err)
		}
	}
	for _, query := range []string{"?limit=0", "?limit=201", "?limit=1&limit=2", "?cursor=x", "?unknown=1", "?version_id=", "?version_id=" + op.VersionID + "&version_id=" + op.VersionID} {
		status := h.call(t, "GET", base+query, nil, nil)
		if query == "?cursor=x" {
			if status != 410 {
				t.Fatal(query, status)
			}
		} else if status != 400 {
			t.Fatal(query, status)
		}
	}
	for _, body := range []string{`{}`, `{"version_id":"x","purpose":"legal_hold","source_kind":"operation","source_ref":"x","expires_at":"","owner":"spoof"}`, `{"version_id":"x","version_id":"y","purpose":"legal_hold","source_kind":"operation","source_ref":"x","expires_at":""}`} {
		if status, _ := fpRawCall(t, h, "POST", base, []byte(body)); status != 400 {
			t.Fatal("strict body", status)
		}
	}
	if status := h.call(t, "POST", base+"/lookup", contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "missing"}, nil); status != 404 {
		t.Fatal("missing receipt", status)
	}
}
