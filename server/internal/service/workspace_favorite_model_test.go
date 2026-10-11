package service

import "testing"

func TestWorkspaceFavoriteIdentityIncludesScopeAndCanonicalTarget(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	original := WorkspaceFavorite{ID: "favorite", Owner: "human", TenantID: "tenant", SpaceID: "space", TargetKind: "directory", TargetRef: "drivebridge://fixture/connection/a/root/root-a/directory/refs", Revision: 1}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := original
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate personal target accepted")
	}
	for _, tc := range []struct{ id, owner, tenant, space, kind, ref string }{
		{"other-owner", "other", "tenant", "space", "directory", original.TargetRef},
		{"other-tenant", "human", "other", "space", "directory", original.TargetRef},
		{"other-space", "human", "tenant", "other", "directory", original.TargetRef},
		{"other-connection", "human", "tenant", "space", "directory", "drivebridge://fixture/connection/b/root/root-a/directory/refs"},
		{"other-root", "human", "tenant", "space", "directory", "drivebridge://fixture/connection/a/root/root-b/directory/refs"},
	} {
		row := WorkspaceFavorite{ID: tc.id, Owner: tc.owner, TenantID: tc.tenant, SpaceID: tc.space, TargetKind: tc.kind, TargetRef: tc.ref, Revision: 1}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	invalid := original
	invalid.ID = "unsupported"
	invalid.TargetKind = "executable"
	if err := db.Create(&invalid).Error; err == nil {
		t.Fatal("unsupported target kind accepted")
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var stored WorkspaceFavorite
	if err := db.First(&stored, "id = ?", original.ID).Error; err != nil || stored.TargetRef != original.TargetRef || stored.Revision != 1 {
		t.Fatal("migration changed favorite identity")
	}
	var count int64
	if err := db.Model(&WorkspaceFavorite{}).Count(&count).Error; err != nil || count != 6 {
		t.Fatal("distinct scope/target bindings collapsed")
	}
}
