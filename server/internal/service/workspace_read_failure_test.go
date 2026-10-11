package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestWorkspaceReadVersionDoesNotHideQueryFailures(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&File{}, &Version{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&File{ID: "file", Owner: "human", SpaceID: "space", VersionID: "version"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Version{ID: "version", Owner: "human", FileID: "file"}).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human"}
	for _, kind := range []string{"file", "version"} {
		t.Run(kind, func(t *testing.T) {
			const callback = "fixture_fixed_read_failure"
			injected := false
			if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
				_, file := tx.Statement.Dest.(*File)
				_, version := tx.Statement.Dest.(*Version)
				if (kind == "file" && file) || (kind == "version" && version) {
					injected = true
					tx.AddError(errors.New("fixture unavailable"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			_, _, failed := s.workspaceReadVersionOn(db, p, "file", "version")
			view, statFailure := s.workspaceFileStat(p, "file")
			if view != nil || statFailure == nil || statFailure.Error() != "database_unavailable" {
				t.Fatalf("stat hid %s query failure: %v", kind, statFailure)
			}
			if err := db.Callback().Query().Remove(callback); err != nil {
				t.Fatal(err)
			}
			if !injected || failed == nil || failed.Error() != "database_unavailable" || workspaceVisibilityDenied(failed) {
				t.Fatalf("%s query failure hidden: %v", kind, failed)
			}
		})
	}
	if view, err := s.workspaceFileStat(p, "missing"); view != nil || err == nil || err.Error() != "not_found" {
		t.Fatalf("stat missing file misclassified: %v", err)
	}
	for _, pair := range [][2]string{{"missing", "version"}, {"file", "missing"}} {
		_, _, err := s.workspaceReadVersionOn(db, p, pair[0], pair[1])
		if err == nil || err.Error() != "not_found" {
			t.Fatalf("missing record misclassified: %v", err)
		}
	}
}
