package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceSubmissionCompleteVerifiesAndPublishesOnce(t *testing.T) {
	s, store, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture"}
	grant := WorkspaceGrant{ID: ID(), Owner: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, RestrictionProfile: a.RestrictionProfile, Actions: "inbox.submit", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &grant} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	body := []byte("verified inbox bytes")
	hash := sha256.Sum256(body)
	input := contract.CreateSubmissionRequest{Name: "result.txt", MIMEType: "text/plain", SizeBytes: int64(len(body)), SHA256: hex.EncodeToString(hash[:])}
	var sub WorkspaceSubmission
	var op Operation
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, op, _, err = s.createWorkspaceSubmissionOn(tx, p, "key", input, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.completeAuthorized(t.Context(), p, op.ID, true); err == nil {
		t.Fatal("missing parts completed")
	}
	if _, err := s.putPartAuthorized(p, op.ID, 0, bytes.NewReader(body), false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Complete(t.Context(), p, op.ID); err == nil {
		t.Fatal("legacy complete allowed staging")
	}
	const callback = "fixture_complete_submission_audit"
	if err := s.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*Audit); ok && audit.Action == "workspace.submission.submitted" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failed := s.completeAuthorized(t.Context(), p, op.ID, true)
	if err := s.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "audit_unavailable" {
		t.Fatalf("expected audit failure: %v", failed)
	}
	var pending WorkspaceSubmission
	if err := s.DB.First(&pending, "id = ?", sub.ID).Error; err != nil || pending.State != "uploading" || pending.FileID != nil {
		t.Fatal("failed complete published submission")
	}
	for _, model := range []any{&File{}, &Version{}, &Blob{}} {
		var count int64
		if err := s.DB.Model(model).Where("owner = ?", p.UserID).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("failed complete published metadata")
		}
	}
	var reserved Space
	if err := s.DB.First(&reserved, "id = ?", sp.ID).Error; err != nil || reserved.Reserved != int64(len(body)) || reserved.Used != 0 {
		t.Fatal("failed complete settled quota")
	}
	completed, err := s.completeAuthorized(t.Context(), p, op.ID, true)
	if err != nil || completed.Status != "completed" {
		t.Fatalf("dedicated completion failed: %v", err)
	}
	replay, err := s.completeAuthorized(t.Context(), p, op.ID, true)
	if err != nil || replay.FileID != completed.FileID || replay.VersionID != completed.VersionID {
		t.Fatal("completion replay changed identity")
	}
	toolResult, err := s.CallTool(t.Context(), p, "complete_upload", map[string]any{"operation_id": op.ID})
	if err != nil {
		t.Fatal(err)
	}
	progress, ok := toolResult.(map[string]any)
	if !ok || progress["id"] != op.ID || progress["status"] != "completed" {
		t.Fatal("MCP application completion lost operation identity")
	}
	for _, field := range []string{"path", "file_id", "version_id", "connection_id"} {
		if _, present := progress[field]; present {
			t.Fatalf("MCP application completion exposed %s", field)
		}
	}
	var result WorkspaceSubmission
	if err := s.DB.First(&result, "id = ?", sub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if result.State != "submitted" || result.Revision != 2 || result.FileID == nil || *result.FileID != completed.FileID || result.VersionID == nil || *result.VersionID != completed.VersionID {
		t.Fatal("fixed submission missing")
	}
	var v Version
	if err := s.DB.First(&v, "id = ?", completed.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	stored := append([]byte(nil), store.data[v.ObjectKey]...)
	store.mu.Unlock()
	if !bytes.Equal(stored, body) {
		t.Fatal("stored bytes differ")
	}
	var current Space
	if err := s.DB.First(&current, "id = ?", sp.ID).Error; err != nil || current.Reserved != 0 || current.Used != int64(len(body)) {
		t.Fatal("quota not settled exactly once")
	}
	legacy := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, Prefix: "refs", Scopes: "read,write", TokenHash: "legacy-fixture", Expires: now.Add(time.Hour)}
	if err := s.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyPrincipal := Principal{UserID: human.UserID, Agent: &legacy}
	if _, err := s.file(legacyPrincipal, completed.FileID, "read"); err == nil {
		t.Fatal("legacy file access reached staging")
	}
	view, err := s.QueryFiles(legacyPrincipal, FileQueryRequest{Mode: "exact", SpaceIDs: []string{sp.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 0 || (view.TotalCount != nil && *view.TotalCount != 0) {
		t.Fatal("legacy query exposed staging items or count")
	}
	var count int64
	if err := s.DB.Model(&Version{}).Where("file_id = ?", completed.FileID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("replay duplicated version")
	}
}
