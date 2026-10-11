package service

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspacePublishRechecksAuthorizationInsideTransaction(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &Space{}, &Operation{}, &File{}, &Version{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: "publisher", Owner: "human", SpaceID: "space", Prefix: "refs", Scopes: "write", Expires: time.Now().Add(time.Hour)}
	sp := Space{ID: "space", Owner: "human", ConnectionID: "connection"}
	op := Operation{ID: "operation", Owner: "human", AgentID: a.ID, SpaceID: sp.ID, Path: "refs/file", Kind: "upload", Status: "verifying", VersionID: "version"}
	for _, row := range []any{&a, &sp, &op} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	// Revoke after the outer authorization check, when publish reads its space.
	// This deterministic hook avoids scheduler timing or a race-detector run.
	revoked := false
	const callback = "fixture_revoke_before_publish_transaction"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*Space); ok && !revoked {
			revoked = true
			if err := db.Model(&Agent{}).Where("id = ?", a.ID).Update("revoked", true).Error; err != nil {
				tx.AddError(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Query().Remove(callback)
	_, err := s.publishUpload(Principal{UserID: a.Owner, Agent: &a}, op, false, "text/plain", t.TempDir())
	if !revoked {
		t.Fatal("revocation hook did not execute")
	}
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("publication did not reject revoked authority: %v", err)
	}
	for _, model := range []any{&File{}, &Version{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("revoked publication persisted metadata")
		}
	}
}
