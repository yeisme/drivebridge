package service_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestWorkspaceTwoAgentsCannotCrossSubmissionBoundary(t *testing.T) {
	for _, payload := range []string{"", "independent agent bytes"} {
		name := "empty"
		if payload != "" {
			name = "with_part"
		}
		t.Run(name, func(t *testing.T) { workspaceTwoAgentSubmission(t, []byte(payload)) })
	}
}

func workspaceTwoAgentSubmission(t *testing.T, payload []byte) {
	t.Helper()
	f := rig(t)
	owner, _ := login(t, f, "owner")
	other, _ := login(t, f, "other")
	issue := func(key string) contract.GrantIssuedView {
		t.Helper()
		data, err := json.Marshal(map[string]any{"name": key, "protocol": "mcp", "space_id": owner.space.ID, "connection_id": owner.space.ConnectionID, "prefix": "refs", "actions": []string{"inbox.submit"}})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", owner.base+"/api/v1/agent-grants", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", owner.base)
		req.Header.Set("X-CSRF-Token", owner.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := owner.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.GrantIssuedView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 201 {
			t.Fatalf("issuance status %d", res.StatusCode)
		}
		return env.Data
	}
	a, b := issue("agent-a"), issue("agent-b")
	ca, err := sdk.New(owner.base, a.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := sdk.New(owner.base, b.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(payload)
	input := contract.CreateSubmissionRequest{Name: "result.txt", MIMEType: "text/plain", SizeBytes: int64(len(payload)), SHA256: hex.EncodeToString(hash[:])}
	var sa, sb contract.SubmissionUploadView
	for _, tc := range []struct {
		c   *sdk.Client
		out *contract.SubmissionUploadView
	}{{ca, &sa}, {cb, &sb}} {
		if err := tc.c.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "same-key", input, tc.out); err != nil {
			t.Fatal(err)
		}
	}
	if sa.SubmissionID == sb.SubmissionID || sa.OperationID == sb.OperationID {
		t.Fatal("shared key collapsed Agent identities")
	}
	ma, mb := workspaceMCP(t, owner.base, a.Credential.Secret), workspaceMCP(t, owner.base, b.Credential.Secret)
	for _, tc := range []struct {
		c            *sdk.Client
		own, foreign contract.SubmissionUploadView
	}{{ca, sa, sb}, {cb, sb, sa}} {
		var ownList contract.Page[contract.SubmissionView]
		if err := tc.c.JSON(t.Context(), "GET", "/api/v1/inbox-submissions?state=uploading", "", nil, &ownList); err != nil || len(ownList.Items) != 1 || ownList.Items[0].SubmissionID != tc.own.SubmissionID {
			t.Fatal("submission list crossed Agent boundary")
		}
		var lookup contract.SubmissionUploadView
		if err := tc.c.JSON(t.Context(), "POST", "/api/v1/inbox-submissions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "same-key"}, &lookup); err != nil || lookup.SubmissionID != tc.own.SubmissionID {
			t.Fatal("lookup crossed Agent identity")
		}
		for _, path := range []string{"/api/v1/inbox-submissions/" + tc.foreign.SubmissionID, "/api/v1/operations/" + tc.foreign.OperationID} {
			if err := tc.c.JSON(t.Context(), "GET", path, "", nil, nil); err == nil {
				t.Fatal("foreign recovery allowed")
			}
		}
		if err := tc.c.JSON(t.Context(), "POST", "/api/v1/operations/"+tc.foreign.OperationID+"/complete", "", nil, nil); err == nil {
			t.Fatal("foreign completion allowed")
		}
	}
	var humanList contract.Page[contract.SubmissionView]
	if status := owner.call(t, "GET", "/inbox-submissions?state=uploading", nil, &humanList); status != 200 || len(humanList.Items) != 2 {
		t.Fatal("owner submission list omitted source")
	}
	if status := other.call(t, "GET", "/inbox-submissions", nil, &humanList); status != 200 || len(humanList.Items) != 0 {
		t.Fatal("foreign owner submission list leaked data")
	}
	var firstPage, secondPage contract.Page[contract.SubmissionView]
	if status := owner.call(t, "GET", "/inbox-submissions?limit=1", nil, &firstPage); status != 200 || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatal("submission first page failed")
	}
	nextPage := "/inbox-submissions?limit=1&cursor=" + url.QueryEscape(firstPage.NextCursor)
	if status := owner.call(t, "GET", nextPage, nil, &secondPage); status != 200 || len(secondPage.Items) != 1 || secondPage.Items[0].SubmissionID == firstPage.Items[0].SubmissionID || secondPage.NextCursor != "" {
		t.Fatal("submission second page failed")
	}
	if err := ca.JSON(t.Context(), "GET", "/api/v1"+nextPage, "", nil, nil); err == nil || err.Error() != "cursor_expired" {
		t.Fatal("Agent reused human submission cursor")
	}
	workspaceMCPCall(t, ma, "operation_get", map[string]any{"operation_id": sb.OperationID}, true)
	workspaceMCPCall(t, mb, "complete_upload", map[string]any{"operation_id": sa.OperationID}, true)
	if status := other.call(t, "GET", "/inbox-submissions/"+sa.SubmissionID, nil, nil); status != 404 {
		t.Fatalf("foreign human status %d", status)
	}
	if len(payload) > 0 {
		put := func(token, operation string) int {
			t.Helper()
			req, err := http.NewRequest("PUT", owner.base+"/api/v1/operations/"+operation+"/parts/0", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			return res.StatusCode
		}
		for _, tc := range []struct{ token, own, foreign string }{{a.Credential.Secret, sa.OperationID, sb.OperationID}, {b.Credential.Secret, sb.OperationID, sa.OperationID}} {
			if status := put(tc.token, tc.foreign); status != 403 {
				t.Fatalf("foreign part status %d", status)
			}
		}
		var count int64
		if err := f.Service.DB.Model(&service.Part{}).Where("operation_id IN ?", []string{sa.OperationID, sb.OperationID}).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("denied cross-Agent upload retained part")
		}
		workspaceMCPCall(t, ma, "complete_upload", map[string]any{"operation_id": sa.OperationID}, true)
		if status := put(a.Credential.Secret, sa.OperationID); status != 200 {
			t.Fatalf("own A part status %d", status)
		}
		if status := put(b.Credential.Secret, sb.OperationID); status != 200 {
			t.Fatalf("own B part status %d", status)
		}
		if err := f.Service.DB.Model(&service.Part{}).Where("operation_id IN ?", []string{sa.OperationID, sb.OperationID}).Count(&count).Error; err != nil || count != 2 {
			t.Fatal("own parts not independently persisted")
		}
	}
	workspaceMCPCall(t, ma, "complete_upload", map[string]any{"operation_id": sa.OperationID}, false)
	if status := owner.call(t, "GET", nextPage, nil, nil); status != 410 {
		t.Fatalf("changed submission retained cursor: %d", status)
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", a.Grant.GrantID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	workspaceMCPRevoked(t, ma)
	workspaceMCPCall(t, mb, "complete_upload", map[string]any{"operation_id": sb.OperationID}, false)
	for _, id := range []string{sa.SubmissionID, sb.SubmissionID} {
		var view contract.SubmissionView
		if status := owner.call(t, "GET", "/inbox-submissions/"+id, nil, &view); status != 200 || view.State != "submitted" {
			t.Fatal("owner recovery lost independent result")
		}
	}
}
