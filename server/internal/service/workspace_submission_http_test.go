package service_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

func TestWorkspaceSubmissionCreateHTTPAndLookup(t *testing.T) {
	workspaceSubmissionHTTPFlow(t, false)
}

func TestWorkspaceSubmissionSurvivesServiceReopen(t *testing.T) {
	workspaceSubmissionHTTPFlow(t, true)
}

func workspaceSubmissionHTTPFlow(t *testing.T, reopen bool) {
	t.Helper()
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	data, _ := json.Marshal(map[string]any{"name": "Inbox only", "protocol": "http", "space_id": h.space.ID, "connection_id": h.space.ConnectionID, "prefix": "refs", "actions": []string{"inbox.submit"}})
	req, err := http.NewRequest("POST", h.base+"/api/v1/agent-grants", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "grant")
	res, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var issued contract.DataEnvelope[contract.GrantIssuedView]
	err = json.NewDecoder(res.Body).Decode(&issued)
	res.Body.Close()
	if err != nil || res.StatusCode != 201 {
		t.Fatal("grant issuance failed")
	}
	client, err := sdk.New(h.base, issued.Data.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("0123456789"))
	input := contract.CreateSubmissionRequest{Name: "result.txt", MIMEType: "text/plain", SizeBytes: 10, SHA256: hex.EncodeToString(digest[:])}
	callback := "submission_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.submission.create" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "failed", input, nil)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatal("audit failure not returned")
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceSubmission{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed audit retained submission")
	}
	var created contract.SubmissionUploadView
	if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "create", input, &created); err != nil {
		t.Fatal(err)
	}
	if created.Purpose != "inbox_staging" || created.OperationID == "" || created.PartSize != service.PartSize {
		t.Fatal("invalid upload receipt")
	}
	nativeMCP := workspaceMCP(t, h.base, issued.Data.Credential.Secret)
	capabilities := workspaceMCPCall(t, nativeMCP, "capabilities", map[string]any{}, false)
	if !bytes.Contains(capabilities, []byte("/api/v1/inbox-submissions")) || !bytes.Contains(capabilities, []byte("Idempotency-Key")) || bytes.Contains(capabilities, []byte(issued.Data.Credential.Secret)) {
		t.Fatal("MCP HTTP discovery missing or leaked secret")
	}
	workspaceMCPCall(t, nativeMCP, "create_upload", map[string]any{"space_id": h.space.ID, "path": "refs/bypass", "size_bytes": 10, "sha256": input.SHA256, "idempotency_key": "bypass"}, true)
	workspaceMCPCall(t, nativeMCP, "operation_get", map[string]any{"operation_id": created.OperationID}, false)
	var replay, lookup contract.SubmissionUploadView
	if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "create", input, &replay); err != nil || replay != created {
		t.Fatal("creation replay changed identity")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "create"}, &lookup); err != nil || lookup != created {
		t.Fatal("lookup changed identity")
	}
	var recovery struct {
		Operation     map[string]json.RawMessage `json:"operation"`
		ReceivedParts []int                      `json:"received_parts"`
	}
	recoverOperation := func() error {
		return client.JSON(t.Context(), "GET", "/api/v1/operations/"+created.OperationID, "", nil, &recovery)
	}
	if err := recoverOperation(); err != nil || len(recovery.ReceivedParts) != 0 {
		t.Fatal("empty recovery failed")
	}
	for _, key := range []string{"path", "connection_id", "workspace_grant_id", "sha256", "file_id", "version_id"} {
		if _, ok := recovery.Operation[key]; ok {
			t.Fatalf("recovery leaked %s", key)
		}
	}
	changed := input
	changed.Name = "other.txt"
	if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions", "create", changed, nil); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed replay accepted")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/uploads", "legacy", map[string]any{"space_id": h.space.ID, "path": "refs/file", "size_bytes": 10, "sha256": input.SHA256}, nil); err == nil {
		t.Fatal("legacy upload permitted")
	}
	put := func(body string) int {
		t.Helper()
		req, err := http.NewRequest("PUT", h.base+strings.ReplaceAll(created.PartPath, "{index}", "0"), strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+issued.Data.Credential.Secret)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if status := put("0123456789"); status != 200 {
		t.Fatalf("part status %d", status)
	}
	if status := put("0123456789"); status != 200 {
		t.Fatalf("part replay status %d", status)
	}
	if status := put("different!"); status != 409 {
		t.Fatalf("part conflict status %d", status)
	}
	if err := f.Service.DB.Model(&service.Part{}).Where("operation_id = ?", created.OperationID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("part not persisted exactly once")
	}
	if reopen {
		// Keep the fixture OIDC/S3 dependencies alive; restart only the service.
		// This is service reopen, not an independent OS-process restart.
		config := f.Service.Config
		address := f.App.Listener.Addr().String()
		nativeMCP.Close()
		f.App.Close()
		f.Service.Close()
		reopened, err := service.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		f.Service = reopened
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		app := httptest.NewUnstartedServer(reopened.Handler())
		app.Listener.Close()
		app.Listener = listener
		app.Start()
		f.App = app
		nativeMCP = workspaceMCP(t, h.base, issued.Data.Credential.Secret)
		if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "create"}, &lookup); err != nil || lookup != created {
			t.Fatal("service reopen changed original submission")
		}
	}
	if err := recoverOperation(); err != nil || len(recovery.ReceivedParts) != 1 || recovery.ReceivedParts[0] != 0 {
		t.Fatal("persisted part missing from recovery")
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/operations", "", nil, nil); err == nil {
		t.Fatal("submit grant listed all operations")
	}
	var submission contract.SubmissionView
	readSubmission := func() error {
		return client.JSON(t.Context(), "GET", "/api/v1/inbox-submissions/"+created.SubmissionID, "", nil, &submission)
	}
	if err := readSubmission(); err != nil || submission.State != "uploading" || submission.UploadOperationRef != created.OperationID || submission.FileRef != "" || submission.VersionID != "" {
		t.Fatal("staged bytes incorrectly reported as submitted")
	}
	var humanView contract.SubmissionView
	if status := h.call(t, "GET", "/inbox-submissions/"+created.SubmissionID, nil, &humanView); status != 200 || humanView.SubmissionID != created.SubmissionID {
		t.Fatal("owner cannot inspect submission")
	}
	if status := bob.call(t, "GET", "/inbox-submissions/"+created.SubmissionID, nil, nil); status != 404 {
		t.Fatalf("foreign submission status %d", status)
	}
	var sp service.Space
	if err := f.Service.DB.First(&sp, "id = ?", h.space.ID).Error; err != nil || sp.Reserved != 10 {
		t.Fatal("quota duplicated or rollback failed")
	}
	workspaceMCPCall(t, nativeMCP, "complete_upload", map[string]any{"operation_id": created.OperationID}, false)
	var completed service.Operation
	for i := 0; i < 2; i++ {
		if err := client.JSON(t.Context(), "POST", "/api/v1/operations/"+created.OperationID+"/complete", "", nil, &completed); err != nil || completed.Status != "completed" {
			t.Fatalf("complete failed: %v", err)
		}
	}
	if err := readSubmission(); err != nil || submission.State != "submitted" || submission.Revision != 2 || submission.FileRef == "" || submission.VersionID == "" {
		t.Fatal("completed submission not readable")
	}
	var fixed service.File
	if err := f.Service.DB.First(&fixed, "id = ?", submission.FileRef).Error; err != nil {
		t.Fatal(err)
	}
	if submission.VersionID != fixed.VersionID {
		t.Fatal("public submission lost its fixed version")
	}
	decision := contract.SubmissionDecisionRequest{ExpectedRevision: 2, VersionID: submission.VersionID, Decision: "accept", Reason: "Reviewed"}
	decisionPath := "/api/v1/inbox-submissions/" + created.SubmissionID + "/decision"
	if err := client.JSON(t.Context(), "POST", decisionPath, "self", decision, nil); err == nil {
		t.Fatal("Agent accepted own submission")
	}
	decide := func(key string) (int, contract.SubmissionView) {
		t.Helper()
		body, err := json.Marshal(decision)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+decisionPath, bytes.NewReader(body))
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
		var envelope contract.DataEnvelope[contract.SubmissionView]
		if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, envelope.Data
	}
	const decideCallback = "submission_decision_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(decideCallback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.submission.accept" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failedStatus, _ := decide("accept")
	if err := f.Service.DB.Callback().Create().Remove(decideCallback); err != nil {
		t.Fatal(err)
	}
	if failedStatus != 503 {
		t.Fatalf("decision audit status %d", failedStatus)
	}
	if err := readSubmission(); err != nil || submission.State != "submitted" || submission.Revision != 2 {
		t.Fatal("failed audit committed decision")
	}
	for i := 0; i < 2; i++ {
		status, accepted := decide("accept")
		if status != 200 || accepted.State != "accepted" || accepted.Revision != 3 || accepted.Decision == nil || accepted.Decision.VersionID != fixed.VersionID {
			t.Fatal("decision or replay failed")
		}
	}
	if status, _ := decide("new-key"); status != 409 {
		t.Fatalf("stale revision status %d", status)
	}
	decision.Decision = "reject"
	if status, _ := decide("accept"); status != 409 {
		t.Fatalf("changed idempotent decision status %d", status)
	}
	if err := f.Service.DB.Model(&service.WorkspaceSubmissionDecision{}).Where("submission_id = ?", created.SubmissionID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("decision duplicated")
	}
	_, legacy := agent(t, h, "refs", "read,write")
	if err := legacy.JSON(t.Context(), "GET", "/api/v1/files/"+fixed.ID+"/content?version="+fixed.VersionID, "", nil, nil); err == nil {
		t.Fatal("legacy Agent read staging content")
	}
	if err := f.Service.DB.First(&sp, "id = ?", h.space.ID).Error; err != nil || sp.Reserved != 0 || sp.Used != 10 {
		t.Fatal("completion quota incorrect")
	}
	if err := f.Service.DB.Model(&service.Version{}).Where("file_id = ?", fixed.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("completion duplicated version")
	}
	archiveInput := contract.ArchiveSubmissionRequest{ExpectedRevision: 3, VersionID: fixed.VersionID, TargetSpaceID: h.space.ID, TargetPrefix: "accepted"}
	archivePath := "/api/v1/inbox-submissions/" + created.SubmissionID + "/archive"
	if err := client.JSON(t.Context(), "POST", archivePath, "self-archive", archiveInput, nil); err == nil {
		t.Fatal("Agent archived own submission")
	}
	archive := func() (int, contract.ArchiveSubmissionView) {
		t.Helper()
		body, err := json.Marshal(archiveInput)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+archivePath, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "archive")
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.ArchiveSubmissionView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	// These target forms are rejected before operation/receipt allocation.
	for _, prefix := range []string{"/absolute", "a//b", "a/./b", "a/../b", "a/", "a\\b", "a\x01b", "refs/.inbox-staging", strings.Repeat("界", 342)} {
		archiveInput.TargetPrefix = prefix
		status, _ := archive()
		if status != 400 && status != 403 {
			t.Fatalf("invalid archive target status %d", status)
		}
		if err := readSubmission(); err != nil || submission.State != "accepted" || submission.Revision != 3 || submission.ArchiveOperationRef != "" {
			t.Fatal("invalid target changed submission")
		}
		var allocated, receipts int64
		if err := f.Service.DB.Model(&service.Operation{}).Where("archive_file_id = ? AND kind = ?", fixed.ID, "archive").Count(&allocated).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("owner = ? AND action = ? AND request_key = ?", h.user.ID, "submission.archive", "archive").Count(&receipts).Error; err != nil {
			t.Fatal(err)
		}
		if allocated != 0 || receipts != 0 {
			t.Fatal("invalid target allocated operation or receipt")
		}
	}
	archiveInput.TargetPrefix = "accepted"
	const archiveCallback = "http_archive_publish_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(archiveCallback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.submission.archived" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failedArchiveStatus, _ := archive()
	if err := f.Service.DB.Callback().Create().Remove(archiveCallback); err != nil {
		t.Fatal(err)
	}
	if failedArchiveStatus != 503 {
		t.Fatalf("archive audit failure status %d", failedArchiveStatus)
	}
	if err := readSubmission(); err != nil || submission.State != "archiving" || submission.ArchivedFileRef != "" {
		t.Fatal("failed archive reported target")
	}
	status, pendingArchive := archive()
	if status != 202 || pendingArchive.FileRef != nil || pendingArchive.OperationRef == "" || pendingArchive.OperationRef != submission.ArchiveOperationRef {
		t.Fatal("lost response recovery did not return original pending operation")
	}
	if status := bob.call(t, "POST", "/operations/"+pendingArchive.OperationRef+"/resume", nil, nil); status != 404 {
		t.Fatalf("foreign archive resume status %d", status)
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/operations/"+pendingArchive.OperationRef+"/resume", "", nil, nil); err == nil {
		t.Fatal("submit Agent resumed human archive")
	}
	var resumed service.Operation
	if status := h.call(t, "POST", "/operations/"+pendingArchive.OperationRef+"/resume", nil, &resumed); status != 200 || resumed.Status != "completed" || resumed.ID != pendingArchive.OperationRef {
		t.Fatalf("HTTP resume failed: %d", status)
	}
	status, archived := archive()
	if status != 202 || archived.FileRef == nil || archived.OperationRef != pendingArchive.OperationRef {
		t.Fatalf("archive failed: %d", status)
	}
	status, archiveReplay := archive()
	if status != 202 || archiveReplay.OperationRef != archived.OperationRef || archiveReplay.FileRef == nil || *archiveReplay.FileRef != *archived.FileRef {
		t.Fatal("archive replay changed result")
	}
	if err := readSubmission(); err != nil || submission.State != "archived" || submission.VersionID != fixed.VersionID || submission.ArchiveOperationRef != archived.OperationRef || submission.ArchivedFileRef != *archived.FileRef {
		t.Fatal("archived submission not readable")
	}
	if err := f.Service.DB.Model(&service.Operation{}).Where("archive_file_id = ? AND kind = ?", fixed.ID, "archive").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("HTTP recovery duplicated archive operation")
	}
	if err := f.Service.DB.Model(&service.Version{}).Where("file_id = ?", *archived.FileRef).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("HTTP recovery duplicated archive version")
	}
	archiveInput.TargetPrefix = "different"
	if status, _ := archive(); status != 409 {
		t.Fatalf("changed archive accepted: %d", status)
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", issued.Data.Grant.GrantID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/inbox-submissions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: "create"}, nil); err == nil {
		t.Fatal("revoked lookup allowed")
	}
	workspaceMCPRevoked(t, nativeMCP)
	if err := recoverOperation(); err == nil {
		t.Fatal("revoked operation recovery allowed")
	}
	if err := readSubmission(); err == nil {
		t.Fatal("revoked submission read allowed")
	}
	if status := h.call(t, "GET", "/inbox-submissions/"+created.SubmissionID, nil, &humanView); status != 200 {
		t.Fatal("revocation blocked human recovery")
	}
	if status := put("0123456789"); status != 401 {
		t.Fatalf("revoked part status %d", status)
	}
}
