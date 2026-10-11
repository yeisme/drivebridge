package service

import "testing"

// An older disposable table without the new uniqueness constraint forces a
// real late AutoMigrate failure. No production data repair is performed here.
type unconstrainedPropertyFixture struct {
	ID          string `gorm:"primaryKey"`
	Owner       string
	TenantID    string
	SpaceID     string
	Key         string
	DisplayName string
	Type        string
	Revision    uint64
}

func (unconstrainedPropertyFixture) TableName() string { return "workspace_property_definitions" }

func TestWorkspaceMigrationFailureRollsBackAndRetries(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &unconstrainedPropertyFixture{}); err != nil {
		t.Fatal(err)
	}
	original := Agent{ID: "legacy", Owner: "human", TokenHash: "fixture-hash", Scopes: "read", Name: "Original"}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "duplicate"} {
		row := unconstrainedPropertyFixture{ID: id, Owner: "human", TenantID: "tenant", SpaceID: "space", Key: "status", DisplayName: "Status", Type: "text", Revision: 1}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateWorkspaceModels(db); err == nil {
		t.Fatal("duplicate stable keys did not fail migration")
	}
	if db.Migrator().HasTable(&WorkspaceGrant{}) || db.Migrator().HasTable(&WorkspaceRequest{}) {
		t.Fatal("failed migration retained earlier workspace tables")
	}
	var agent Agent
	if err := db.First(&agent, "id = ?", original.ID).Error; err != nil || agent.TokenHash != original.TokenHash || agent.Scopes != original.Scopes || agent.Name != original.Name {
		t.Fatal("failed migration changed legacy identity")
	}
	var count int64
	if err := db.Model(&unconstrainedPropertyFixture{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("failure destroyed conflicting fixture rows")
	}
	// Explicit fixture-only repair makes retry valid; migration never deletes it.
	if err := db.Where("id = ?", "duplicate").Delete(&unconstrainedPropertyFixture{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var definition WorkspacePropertyDefinition
	if err := db.First(&definition, "id = ?", "first").Error; err != nil || definition.Key != "status" || definition.Revision != 1 {
		t.Fatal("retry lost definition")
	}
	if !db.Migrator().HasTable(&WorkspaceGrant{}) || !db.Migrator().HasTable(&WorkspaceFilePropertyProjection{}) {
		t.Fatal("retry missing workspace tables")
	}
}
