package service

import (
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceGrantScopePatchAtomicAuthority(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	expiry := now.Add(time.Hour)
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "conn", Prefix: "refs", TokenHash: "fixture", RestrictionProfile: "personal_workspace_v1", Expires: now}
	g := WorkspaceGrant{ID: "grant", Owner: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, TenantID: "tenant", Prefix: a.Prefix, Actions: "labels.suggest,inbox.submit", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "duration", ExpiresAt: &expiry, IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &Space{ID: a.SpaceID, Owner: a.Owner, TenantID: g.TenantID, ConnectionID: a.ConnectionID}, &a, &g} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: a.Owner, TenantID: g.TenantID}
	input := contract.PatchGrantRequest{ExpectedRevision: 1, Prefix: contract.Field[string]{Present: true, Value: "refs/narrow"}, Actions: contract.Field[[]string]{Present: true, Value: []string{"labels.suggest"}}}
	abort := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.patchWorkspaceGrantScopeOn(tx, p, g.ID, input, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var stored Agent
	if err := db.First(&stored, "id = ?", a.ID).Error; err != nil || stored.Prefix != a.Prefix {
		t.Fatal("rollback changed Agent prefix")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { _, err := s.patchWorkspaceGrantScopeOn(tx, p, g.ID, input, now); return err }); err != nil {
		t.Fatal(err)
	}
	var updated WorkspaceGrant
	if err := db.First(&updated, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&stored, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Prefix != stored.Prefix || stored.Prefix != "refs/narrow" || stored.TokenHash != a.TokenHash || updated.CredentialGeneration != 1 || updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expiry) {
		t.Fatal("scope mutation changed identity/expiry")
	}
	ap := Principal{UserID: a.Owner, TenantID: g.TenantID, Agent: &stored}
	if _, err := s.authorizeWorkspaceActionOn(db, ap, a.SpaceID, a.ConnectionID, "refs/narrow/file", "labels.suggest", now); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, action string }{{"refs/outside", "labels.suggest"}, {"refs/narrow/file", "inbox.submit"}} {
		if _, err := s.authorizeWorkspaceActionOn(db, ap, a.SpaceID, a.ConnectionID, tc.path, tc.action, now); err == nil {
			t.Fatal("old scope retained authority")
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { _, err := s.patchWorkspaceGrantScopeOn(tx, p, g.ID, input, now); return err }); err == nil {
		t.Fatal("stale revision accepted")
	}
	input.ExpectedRevision = 2
	input.Prefix.Value = "refs/../secret"
	if err := db.Transaction(func(tx *gorm.DB) error { _, err := s.patchWorkspaceGrantScopeOn(tx, p, g.ID, input, now); return err }); err == nil {
		t.Fatal("traversal prefix accepted")
	}
}
