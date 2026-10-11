package service

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceHomeSubmissionOwnerFlow(t *testing.T) {
	s, _, human := blobRig(t)
	var space Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: space.ID, ConnectionID: space.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "home-fixture"}
	grant := WorkspaceGrant{ID: ID(), Owner: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, RestrictionProfile: a.RestrictionProfile, Actions: "inbox.submit", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &grant} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	var sub WorkspaceSubmission
	var op Operation
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var e error
		sub, op, _, e = s.createWorkspaceSubmissionOn(tx, p, "home-sub", contract.CreateSubmissionRequest{Name: "a.txt", MIMEType: "text/plain", SizeBytes: 1, SHA256: Hash("a")}, now)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	read := func() contract.WorkspaceHome {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/workspace/home?space_id="+space.ID, nil)
		if err := s.workspaceHome(w, r, human); err != nil {
			t.Fatal(err)
		}
		var env contract.DataEnvelope[contract.WorkspaceHome]
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("home not private")
		}
		return env.Data
	}
	if *read().PendingCounts.Submitted != 0 {
		t.Fatal("uploading counted as submitted")
	}
	if _, err = s.putPartAuthorized(p, op.ID, 0, bytes.NewBufferString("a"), false, true); err != nil {
		t.Fatal(err)
	}
	completed, err := s.completeAuthorized(t.Context(), p, op.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	home := read()
	if home.PendingCounts.Submitted == nil || *home.PendingCounts.Submitted != 1 || *home.PendingCounts.Accepted != 0 || len(home.RecentFiles) != 0 {
		t.Fatal("published staging leaked into recent files or missing pending")
	}
	if err = s.DB.Transaction(func(tx *gorm.DB) error {
		_, e := s.decideWorkspaceSubmissionOn(tx, human, sub.ID, contract.SubmissionDecisionRequest{ExpectedRevision: 2, VersionID: completed.VersionID, Decision: "accept"}, now)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	home = read()
	if *home.PendingCounts.Submitted != 0 || *home.PendingCounts.Accepted != 1 {
		t.Fatal("accepted was silently archived")
	}
	var stored WorkspaceSubmission
	if err = s.DB.First(&stored, "id = ?", sub.ID).Error; err != nil || stored.State != "accepted" || stored.ArchiveOperationID != nil {
		t.Fatal("home mutated submission")
	}
	if err = s.DB.Model(&Operation{}).Where("id = ?", op.ID).Update("workspace_grant_id", "wrong").Error; err != nil {
		t.Fatal(err)
	}
	home = read()
	if home.Sections.PendingCounts.State != "unavailable" || home.PendingCounts.Submitted != nil || home.Sections.Usage.State != "ready" {
		t.Fatal("broken binding became zero or poisoned other section")
	}
	for _, changed := range []Principal{{UserID: human.UserID, TenantID: "foreign"}, {UserID: human.UserID, ScopeSpace: "other"}} {
		if err := s.workspaceHome(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/workspace/home?space_id="+space.ID, nil), changed); err == nil {
			t.Fatal("foreign scope admitted")
		}
	}
}
