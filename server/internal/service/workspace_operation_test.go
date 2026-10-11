package service

import (
	"testing"
)

func TestWorkspaceOperationBindingRejectsLegacyAccess(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Operation{}); err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	p := Principal{UserID: "owner"}
	for _, tc := range []struct{ id, purpose, grant string }{
		{"legacy", "", ""},
		{"staging", "inbox_staging", "grant"},
		{"missing-grant", "inbox_staging", ""},
		{"missing-purpose", "", "grant"},
		{"unknown", "unknown", "grant"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			op := Operation{ID: tc.id, Owner: p.UserID, Key: tc.id, Purpose: tc.purpose, WorkspaceGrantID: tc.grant, Kind: "upload", SpaceID: "space", Path: "refs/file", Status: "awaiting_upload"}
			if err := db.Create(&op).Error; err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"read", "write", "transfer"} {
				_, err := s.operation(p, op.ID, action)
				if tc.id == "legacy" && err != nil {
					t.Fatalf("legacy operation blocked: %v", err)
				}
				if tc.id != "legacy" && (err == nil || err.Error() != "forbidden") {
					t.Fatalf("bound operation reached legacy %s: %v", action, err)
				}
			}
			if tc.id != "legacy" {
				if _, err := s.publishUpload(p, op, false, "text/plain", t.TempDir()); err == nil || err.Error() != "forbidden" {
					t.Fatalf("bound operation reached legacy publish: %v", err)
				}
			}
		})
	}
	if err := db.AutoMigrate(&Operation{}); err != nil {
		t.Fatal(err)
	}
	var op Operation
	if err := db.First(&op, "id = ?", "staging").Error; err != nil {
		t.Fatal(err)
	}
	if op.Purpose != "inbox_staging" || op.WorkspaceGrantID != "grant" {
		t.Fatal("repeat migration lost binding")
	}
}
