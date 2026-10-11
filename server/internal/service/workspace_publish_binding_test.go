package service

import "testing"

func TestWorkspaceLegacyPublicationRejectsStaleUnboundSnapshot(t *testing.T) {
	for _, binding := range []struct{ name, purpose, grant string }{
		{"complete", "inbox_staging", "grant"}, {"purpose-only", "inbox_staging", ""}, {"grant-only", "", "grant"},
	} {
		t.Run(binding.name, func(t *testing.T) {
			db := workspaceTestDB(t)
			if err := db.AutoMigrate(&Space{}, &Operation{}, &File{}, &Version{}); err != nil {
				t.Fatal(err)
			}
			sp := Space{ID: "space", Owner: "human", ConnectionID: "connection"}
			op := Operation{ID: "operation", Owner: sp.Owner, SpaceID: sp.ID, Path: "refs/file", Kind: "upload", Status: "verifying", VersionID: "version"}
			if err := db.Create(&sp).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&op).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"purpose": binding.purpose, "workspace_grant_id": binding.grant}).Error; err != nil {
				t.Fatal(err)
			}
			s := &Service{DB: db}
			if _, err := s.publishUpload(Principal{UserID: sp.Owner}, op, false, "text/plain", t.TempDir()); err == nil || err.Error() != "forbidden" {
				t.Fatalf("stale snapshot reached publication: %v", err)
			}
			for _, model := range []any{&File{}, &Version{}} {
				var count int64
				if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
					t.Fatal("rejected publication wrote metadata")
				}
			}
		})
	}
}
