package service

import (
	"testing"
	"time"
)

func TestWorkspaceFileSearchScopeAndStagingIsolation(t *testing.T) {
	s, _, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	visible := uploadBytes(t, s, human, sp.ID, "refs/report.txt", []byte("visible"))
	uploadBytes(t, s, human, sp.ID, "refs-other/report.txt", []byte("outside"))
	uploadBytes(t, s, human, sp.ID, "refs/.inbox-staging/report.txt", []byte("staging"))
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", TokenHash: "fixture-search", RestrictionProfile: "personal_workspace_v1"}
	g := WorkspaceGrant{ID: ID(), Owner: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &g} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	files, err := s.searchWorkspaceFiles(p, sp.ID, "REPORT", "", "", "", false)
	if err != nil || len(files) != 1 || files[0].ID != visible.FileID {
		t.Fatalf("search leaked or lost visible result: %v", err)
	}
	for _, prefix := range []string{"refs-other", "refs/../private", "refs/.inbox-staging"} {
		if _, err := s.searchWorkspaceFiles(p, sp.ID, "", prefix, "", "", false); err == nil {
			t.Fatalf("invalid scope accepted: %s", prefix)
		}
	}
	if _, err := s.searchWorkspaceFiles(p, sp.ID, "", "", "", "invalid", false); err == nil {
		t.Fatal("invalid time accepted")
	}
	if _, err := s.searchWorkspaceFiles(p, sp.ID, "", "", "", "", true); err == nil {
		t.Fatal("trash enumeration allowed")
	}
	if err := s.DB.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.searchWorkspaceFiles(p, sp.ID, "", "", "", "", false); err == nil {
		t.Fatal("revoked list returned success")
	}
}
