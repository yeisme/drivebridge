package service

import "testing"

func TestWorkspacePropertyMigrationKeepsVersionIdentity(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	definition := WorkspacePropertyDefinition{ID: "definition", Owner: "human", TenantID: "tenant", SpaceID: "space", Key: "status", DisplayName: "Status", Type: "text", Revision: 1}
	if err := db.Create(&definition).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := definition
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate stable key accepted")
	}
	duplicate.SpaceID = "other-space"
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	invalid := definition
	invalid.ID = "invalid"
	invalid.Key = "invalid"
	invalid.Type = "script"
	if err := db.Create(&invalid).Error; err == nil {
		t.Fatal("unsupported definition type accepted")
	}
	projection := WorkspaceFilePropertyProjection{ID: "projection", Owner: "human", TenantID: "tenant", SpaceID: "space", FileID: "file", VersionID: "v1", DefinitionRevision: 1, Key: "status", StorageKind: "frontmatter", ValueJSON: `{"unknown":{"nested":true}}`, Revision: 1}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	copy := projection
	copy.ID = "duplicate-projection"
	if err := db.Create(&copy).Error; err == nil {
		t.Fatal("duplicate version projection accepted")
	}
	copy.VersionID = "v2"
	if err := db.Create(&copy).Error; err != nil {
		t.Fatal(err)
	}
	copy.ID = "next-definition"
	copy.DefinitionRevision = 2
	if err := db.Create(&copy).Error; err != nil {
		t.Fatal(err)
	}
	copy.ID = "bad-storage"
	copy.VersionID = "v3"
	copy.StorageKind = "current-file"
	if err := db.Create(&copy).Error; err == nil {
		t.Fatal("invalid projection source accepted")
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var restored WorkspaceFilePropertyProjection
	if err := db.First(&restored, "id = ?", projection.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.ValueJSON != projection.ValueJSON || restored.VersionID != "v1" || restored.DefinitionRevision != 1 {
		t.Fatal("repeat migration lost unknown value or pinned version")
	}
	var count int64
	if err := db.Model(&WorkspaceFilePropertyProjection{}).Count(&count).Error; err != nil || count != 3 {
		t.Fatal("version projections collapsed")
	}
}
