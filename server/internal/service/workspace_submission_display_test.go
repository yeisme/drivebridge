package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceSubmissionDisplayUsesOriginalOperationAndAuthenticatedAgent(t *testing.T) {
	s, _, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: ID(), Owner: human.UserID, Name: "Visual research assistant", SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "display-fixture"}
	grant := WorkspaceGrant{ID: ID(), Owner: a.Owner, AgentID: a.ID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: a.Prefix, RestrictionProfile: a.RestrictionProfile, Actions: "inbox.submit", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &grant} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: human.UserID, Agent: &a}
	body := []byte("original submission")
	hash := sha256.Sum256(body)
	input := contract.CreateSubmissionRequest{Name: "研究成果.png", MIMEType: "image/png", SizeBytes: int64(len(body)), SHA256: hex.EncodeToString(hash[:]), Caller: "Unverified caller"}
	var sub WorkspaceSubmission
	var op Operation
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, op, _, err = s.createWorkspaceSubmissionOn(tx, p, "display", input, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	read := func(principal Principal) contract.SubmissionView {
		t.Helper()
		var view contract.SubmissionView
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			var err error
			view, err = s.workspaceSubmissionViewOn(tx, principal, sub.ID)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return view
	}
	expectedMIME := input.MIMEType
	assertDisplay := func(view contract.SubmissionView) {
		t.Helper()
		if view.FileName != input.Name || view.MIMEType != expectedMIME || view.SizeBytes == nil || *view.SizeBytes != input.SizeBytes || view.AgentName != a.Name || view.SourceClaim != input.Caller {
			t.Fatalf("invalid display metadata: %#v", view)
		}
	}
	assertDisplay(read(human))
	assertDisplay(read(p))
	if _, err := s.putPartAuthorized(p, op.ID, 0, bytes.NewReader(body), false, true); err != nil {
		t.Fatal(err)
	}
	completed, err := s.completeAuthorized(t.Context(), p, op.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	expectedMIME = "text/plain; charset=utf-8" // Published MIME is sniffed, not the Agent claim.
	// A later file head must not silently rename or retype the original submission.
	if err := s.DB.Model(&File{}).Where("id = ?", completed.FileID).Updates(map[string]any{"path": "refs/renamed.txt", "version_id": ID(), "mime": "text/plain", "size": 999}).Error; err != nil {
		t.Fatal(err)
	}
	view := read(human)
	assertDisplay(view)
	if view.VersionID != completed.VersionID {
		t.Fatal("display replaced the submitted version")
	}
	// Presentation must not bridge unrelated operations, grants or spaces.
	for _, field := range []string{"space_id", "workspace_grant_id", "agent_id", "purpose", "owner"} {
		t.Run("operation_binding_"+field, func(t *testing.T) {
			if err := s.DB.Model(&Operation{}).Where("id = ?", op.ID).Update(field, "foreign").Error; err != nil {
				t.Fatal(err)
			}
			for _, principal := range []Principal{human, p} {
				if _, err := s.workspaceSubmissionViewOn(s.DB, principal, sub.ID); err == nil {
					t.Fatal("unbound operation exposed metadata")
				}
			}
			if err := s.DB.Save(&completed).Error; err != nil {
				t.Fatal(err)
			}
			assertDisplay(read(human))
		})
	}
	for _, principal := range []Principal{{UserID: human.UserID, ScopeSpace: "foreign"}, {UserID: human.UserID, TenantID: "foreign"}, {UserID: human.UserID, DataOwner: "foreign"}} {
		if _, err := s.workspaceSubmissionViewOn(s.DB, principal, sub.ID); err == nil {
			t.Fatal("foreign scope exposed metadata")
		}
	}
	for _, field := range []string{"owner", "space_id"} {
		if err := s.DB.Model(&Agent{}).Where("id = ?", a.ID).Update(field, "foreign").Error; err != nil {
			t.Fatal(err)
		}
		if view := read(human); view.AgentName != "" {
			t.Fatal("foreign Agent name exposed")
		}
		if err := s.DB.Save(&a).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Revoking the author does not prevent the human from inspecting received bytes.
	if err := s.DB.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	assertDisplay(read(human))
	if _, err := s.workspaceSubmissionViewOn(s.DB, p, sub.ID); err == nil {
		t.Fatal("revoked Agent read submission metadata")
	}
	if _, err := s.workspaceSubmissionViewOn(s.DB, Principal{UserID: ID()}, sub.ID); err == nil {
		t.Fatal("another user read metadata")
	}
	// Missing/deleted Agent presentation data is unknown, never replaced by caller.
	if err := s.DB.Delete(&a).Error; err != nil {
		t.Fatal(err)
	}
	if view := read(human); view.AgentName != "" || view.SourceClaim != input.Caller {
		t.Fatal("caller was promoted to authenticated name")
	}
}
