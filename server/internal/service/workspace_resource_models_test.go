package service

import "testing"

func TestWorkspaceResourceTargetsAndCheckRevisions(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	file, version, snapshot := "file", "version", "snapshot"
	for _, tc := range []struct {
		id                      string
		file, version, snapshot *string
		valid                   bool
	}{
		{"file", &file, &version, nil, true}, {"snapshot", nil, nil, &snapshot, true},
		{"empty", nil, nil, nil, false}, {"missing-version", &file, nil, nil, false},
		{"both", &file, &version, &snapshot, false},
	} {
		descriptor := WorkspaceResourceDescriptor{ID: tc.id, Owner: "human", TenantID: "tenant", SpaceID: "space", FileID: tc.file, VersionID: tc.version, SnapshotID: tc.snapshot, Kind: "skill", DisplayName: "Fixture", Revision: 1}
		if err := db.Create(&descriptor).Error; (err == nil) != tc.valid {
			t.Fatalf("target %s valid=%t err=%v", tc.id, tc.valid, err)
		}
	}
	invalid := WorkspaceResourceDescriptor{ID: "executable", Owner: "human", TenantID: "tenant", SpaceID: "space", FileID: &file, VersionID: &version, Kind: "executable", DisplayName: "Invalid", Revision: 1}
	if err := db.Create(&invalid).Error; err == nil {
		t.Fatal("unsupported resource kind accepted")
	}
	check := WorkspaceResourceCheck{ID: "check", Owner: "human", TenantID: "tenant", DescriptorID: "file", DescriptorRevision: 1, CheckerVersion: "format-v1", State: "passed"}
	if err := db.Create(&check).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := check
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate check identity accepted")
	}
	duplicate.CheckerVersion = "format-v2"
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	duplicate.ID = "next-revision"
	duplicate.DescriptorRevision = 2
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	duplicate.ID = "invalid-state"
	duplicate.CheckerVersion = "invalid"
	duplicate.State = "installed"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("installation state admitted as check")
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var restored WorkspaceResourceDescriptor
	if err := db.First(&restored, "id = ?", "file").Error; err != nil || restored.VersionID == nil || *restored.VersionID != version {
		t.Fatal("migration lost fixed target")
	}
	var count int64
	if err := db.Model(&WorkspaceResourceCheck{}).Count(&count).Error; err != nil || count != 3 {
		t.Fatal("migration collapsed historical checks")
	}
}
