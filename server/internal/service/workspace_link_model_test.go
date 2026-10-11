package service

import "testing"

func TestWorkspaceLinkProjectionPinsSourceVersionAndAnchor(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	link := WorkspaceFileLinkProjection{ID: "link", Owner: "human", TenantID: "tenant", SpaceID: "space", FileID: "file", VersionID: "v1", TargetRef: "drivebridge://fixture/file/target", Anchor: "section"}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := link
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate fixed-version link accepted")
	}
	for _, tc := range []struct{ id, file, version, anchor string }{
		{"next-version", "file", "v2", "section"},
		{"other-anchor", "file", "v1", "another"},
		{"other-file", "file-two", "v1", "section"},
	} {
		row := link
		row.ID = tc.id
		row.FileID = tc.file
		row.VersionID = tc.version
		row.Anchor = tc.anchor
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var restored WorkspaceFileLinkProjection
	if err := db.First(&restored, "id = ?", link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.VersionID != link.VersionID || restored.TargetRef != link.TargetRef || restored.Anchor != link.Anchor {
		t.Fatal("migration retargeted fixed link")
	}
	var count int64
	if err := db.Model(&WorkspaceFileLinkProjection{}).Where("owner = ? AND tenant_id = ?", link.Owner, link.TenantID).Count(&count).Error; err != nil || count != 4 {
		t.Fatal("distinct link identities collapsed")
	}
}
