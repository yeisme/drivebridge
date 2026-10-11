package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func workspaceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "workspace.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func TestWorkspaceMigrationPreservesLegacyAgent(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	original := Agent{ID: "legacy", Owner: "human", SpaceID: "space", Scopes: "read", Expires: expiry, TokenHash: "synthetic-hash"}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateWorkspaceModels(db); err != nil {
			t.Fatal(err)
		}
	}
	var got Agent
	if err := db.First(&got, "id = ?", "legacy").Error; err != nil {
		t.Fatal(err)
	}
	if got.TokenHash != original.TokenHash || !got.Expires.Equal(expiry) || got.Scopes != "read" {
		t.Fatal("legacy identity changed")
	}
}

func TestWorkspaceMigrationAfterPartialSchema(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&WorkspaceGrant{}); err != nil {
		t.Fatal(err)
	}
	// Simulate interruption after one additive table, before group tables.
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&WorkspaceGrant{}, &WorkspaceAgentGroup{}, &WorkspaceAgentOrganization{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table for %T", model)
		}
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceGroupNamesAreScoped(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	first := WorkspaceAgentGroup{ID: "a", Owner: "alice", TenantID: "tenant", Name: "Research", NormalizedName: "research", Revision: 1}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	next := first
	next.ID = "b"
	if err := db.Create(&next).Error; err == nil {
		t.Fatal("duplicate group name accepted")
	}
	next.ID = "c"
	next.TenantID = "other"
	if err := db.Create(&next).Error; err != nil {
		t.Fatal("tenant name isolation broken")
	}
}

func TestWorkspaceGrantExpiryConstraint(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, tc := range []struct {
		id, mode string
		expires  *time.Time
		valid    bool
	}{
		{"never", "never", nil, true}, {"duration", "duration", &now, true},
		{"missing", "duration", nil, false}, {"sentinel", "never", &now, false}, {"unknown", "other", nil, false},
	} {
		g := WorkspaceGrant{ID: tc.id, AgentID: tc.id, Owner: "human", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: tc.mode, ExpiresAt: tc.expires, IssuedAt: now, Revision: 1, CredentialGeneration: 1}
		err := db.Create(&g).Error
		if (err == nil) != tc.valid {
			t.Fatalf("%s valid=%v error=%v", tc.id, tc.valid, err)
		}
	}
}

func TestWorkspaceOrganizationScopedUniqueness(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	first := WorkspaceAgentOrganization{ID: "one", Owner: "alice", TenantID: "tenant", AgentID: "agent"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.ID = "two"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate organization allowed")
	}
	other := first
	other.ID = "three"
	other.Owner = "bob"
	if err := db.Create(&other).Error; err != nil {
		t.Fatal("scope incorrectly global")
	}
}
