package service

import (
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWorkspaceDedicatedActionGate(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &User{}, &Space{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&User{ID: "human", Issuer: "fixture", Subject: "human"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Space{ID: "space", Owner: "human", TenantID: "tenant", ConnectionID: "connection"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := Agent{RestrictionProfile: "personal_workspace_v1", ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "connection", Prefix: "refs", TokenHash: "generation-one", Scopes: "write", Expires: now.Add(-time.Hour)}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	g := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read,inbox.submit", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	authenticatedAgent := a
	p := Principal{UserID: a.Owner, TenantID: g.TenantID, Agent: &authenticatedAgent}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.authorizeWorkspaceActionOn(tx, p, "space", "connection", "refs/file.txt", "files.read", now); err != nil {
			t.Fatalf("valid narrow read: %v", err)
		}
		for _, tc := range []struct{ space, connection, path, action string }{
			{"other", "connection", "refs/file.txt", "files.read"}, {"space", "other", "refs/file.txt", "files.read"},
			{"space", "connection", "refs-other/file.txt", "files.read"}, {"space", "connection", "refs/../secret", "files.read"},
			{"space", "connection", "refs/file.txt", "labels.suggest"}, {"space", "connection", "refs/file.txt", "write"},
		} {
			if _, err := s.authorizeWorkspaceActionOn(tx, p, tc.space, tc.connection, tc.path, tc.action, now); err == nil {
				t.Fatalf("allowed %+v", tc)
			}
		}
		other := p
		other.TenantID = "other"
		if _, err := s.authorizeWorkspaceActionOn(tx, other, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
			t.Fatal("tenant mismatch allowed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Prove staging denial even when the grant explicitly has all actions.
	if err := db.Model(&g).Update("actions", "files.read,labels.suggest,inbox.submit").Error; err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"files.read", "labels.suggest"} {
		if _, err := s.authorizeWorkspaceActionOn(db, p, "space", "connection", "refs/file.txt", action, now); err != nil {
			t.Fatalf("ordinary %s rejected: %v", action, err)
		}
		if _, err := s.authorizeWorkspaceActionOn(db, p, "space", "connection", "refs/.inbox-staging/other/file.txt", action, now); err == nil {
			t.Fatalf("staging admitted %s", action)
		}
	}
	if err := db.Model(&g).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.authorizeWorkspaceActionOn(db, p, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
		t.Fatal("disabled grant allowed")
	}
	if err := db.Model(&g).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&User{}).Where("id = ?", a.Owner).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.authorizeWorkspaceActionOn(db, p, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
		t.Fatal("disabled owner allowed")
	}
	if err := db.Model(&User{}).Where("id = ?", a.Owner).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	legacySnapshot := authenticatedAgent
	legacySnapshot.RestrictionProfile = ""
	legacyPrincipal := p
	legacyPrincipal.Agent = &legacySnapshot
	if _, err := s.authorizeWorkspaceActionOn(db, legacyPrincipal, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
		t.Fatal("unmarked principal entered dedicated action gate")
	}
	emptyAgent := a
	emptyAgent.ID = ""
	emptyPrincipal := p
	emptyPrincipal.Agent = &emptyAgent
	if _, err := s.authorizeWorkspaceActionOn(db, emptyPrincipal, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
		t.Fatal("empty identity allowed")
	}
	if err := db.Model(&a).Update("token_hash", "generation-two").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.authorizeWorkspaceActionOn(db, p, "space", "connection", "refs/file.txt", "files.read", now); err == nil {
		t.Fatal("stale principal survived rotation")
	}
}
