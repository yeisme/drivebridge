package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Two independent test-binary processes share only a disposable SQLite file.
// This proves migration recovery across process exit, not HTTP service restart.
func TestWorkspaceMigrationRecoveryAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.sqlite")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"fail", "recover", "verify"} {
		cmd := exec.CommandContext(t.Context(), binary, "-test.run=^TestWorkspaceMigrationProcessHelper$", "-test.count=1")
		cmd.Env = append(os.Environ(), "DRIVEBRIDGE_MIGRATION_FIXTURE="+path, "DRIVEBRIDGE_MIGRATION_PHASE="+phase)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("migration phase %s: %v\n%s", phase, err, output)
		}
	}
}

func TestWorkspaceMigrationProcessHelper(t *testing.T) {
	path := os.Getenv("DRIVEBRIDGE_MIGRATION_FIXTURE")
	if path == "" {
		t.Skip("subprocess fixture only")
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	phase := os.Getenv("DRIVEBRIDGE_MIGRATION_PHASE")
	if phase == "fail" {
		if err := db.AutoMigrate(&Agent{}, &unconstrainedPropertyFixture{}); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&Agent{ID: "legacy", Owner: "human", TokenHash: "synthetic-process-hash", Scopes: "read", Name: "Original"}).Error; err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"first", "duplicate"} {
			if err := db.Create(&unconstrainedPropertyFixture{ID: id, Owner: "human", TenantID: "tenant", SpaceID: "space", Key: "status", Type: "text", Revision: 1}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := migrateWorkspaceModels(db); err == nil {
			t.Fatal("expected real unique-index migration failure")
		}
	} else if phase != "recover" && phase != "verify" {
		t.Fatal("unknown fixture phase")
	}
	var agent Agent
	if err := db.First(&agent, "id = ?", "legacy").Error; err != nil || agent.TokenHash != "synthetic-process-hash" || agent.Scopes != "read" || agent.Name != "Original" {
		t.Fatal("legacy identity lost across process boundary")
	}
	var count int64
	if err := db.Model(&unconstrainedPropertyFixture{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if phase != "verify" {
		if count != 2 || db.Migrator().HasTable(&WorkspaceGrant{}) {
			t.Fatal("failed migration lost rows or retained partial tables")
		}
	}
	if phase == "fail" {
		return
	}
	if phase == "recover" {
		// Only this explicitly created fixture is repaired; never production data.
		if err := db.Where("id = ?", "duplicate").Delete(&unconstrainedPropertyFixture{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := migrateWorkspaceModels(db); err != nil {
			t.Fatal(err)
		}
	}
	var definition WorkspacePropertyDefinition
	if err := db.First(&definition, "id = ?", "first").Error; err != nil || definition.Key != "status" || definition.Revision != 1 {
		t.Fatal("recovered definition changed")
	}
	if err := db.Model(&WorkspacePropertyDefinition{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("recovered row count changed")
	}
	for _, model := range []any{&WorkspaceGrant{}, &WorkspaceSubmission{}, &WorkspaceFavorite{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table %T", model)
		}
	}
}
