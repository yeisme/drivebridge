package service

import (
	"testing"
	"time"
)

func TestWorkspaceDowngradeRequiresExplicitGrantRetirement(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	if err := s.CheckWorkspaceDowngrade(); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Hour)
	for _, state := range []string{"active", "disabled", "expired"} {
		t.Run(state, func(t *testing.T) {
			g := WorkspaceGrant{ID: state, AgentID: state, Owner: "human", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", Revision: 1, CredentialGeneration: 1}
			if state == "disabled" {
				g.Disabled = true
			}
			if state == "expired" {
				g.ExpiryMode = "duration"
				g.ExpiresAt = &expired
			}
			if err := db.Create(&g).Error; err != nil {
				t.Fatal(err)
			}
			if err := s.CheckWorkspaceDowngrade(); err == nil || err.Error() != "workspace_downgrade_blocked" {
				t.Fatalf("unretired grant accepted: %v", err)
			}
			var stored WorkspaceGrant
			if err := db.First(&stored, "id = ?", g.ID).Error; err != nil || stored.Revoked || stored.Revision != 1 {
				t.Fatal("preflight mutated grant")
			}
			if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
				t.Fatal(err)
			}
			if err := s.CheckWorkspaceDowngrade(); err != nil {
				t.Fatal(err)
			}
		})
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckWorkspaceDowngrade(); err == nil || err.Error() != "database_unavailable" {
		t.Fatalf("failed read authorized downgrade: %v", err)
	}
}
