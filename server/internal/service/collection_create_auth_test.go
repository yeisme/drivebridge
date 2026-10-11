package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionCreateCurrentAuthorityAndReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	input := service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Creation boundary", Kind: "manual", Scope: "private"}
	created, err := f.Service.CreateFileCollection(p, input, "original-create")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: p.UserID, ExternalRead: true}, {UserID: p.UserID, ShareID: "foreign"}, {}} {
		for _, key := range []string{"new-create", "original-create"} {
			if _, err = f.Service.CreateFileCollection(bad, input, key); err == nil {
				t.Fatal("create or replay crossed authority")
			}
		}
	}
	_, reader := agent(t, h, "", "read")
	if err = reader.JSON(t.Context(), "POST", "/api/v1/file-collections", "reader-create", input, nil); err == nil {
		t.Fatal("read-only agent created collection")
	}
	_, writer := agent(t, h, "", "read,write")
	var view service.FileCollectionView
	if err = writer.JSON(t.Context(), "POST", "/api/v1/file-collections", "legacy-writer-create", input, &view); err != nil {
		t.Fatal("authorized legacy writer rejected", err)
	}
	team := input
	team.Scope = "team"
	if err = writer.JSON(t.Context(), "POST", "/api/v1/file-collections", "legacy-team-create", team, nil); err == nil {
		t.Fatal("agent managed team collection")
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateFileCollection(p, input, "original-create"); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled replay returned collection", err)
	}
	var count int64
	if err = f.Service.DB.Model(&service.FileCollection{}).Where("id = ?", created.CollectionID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("original collection altered", err)
	}
}

func TestCollectionCreateCountFailureRollsBack(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	input := service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic create", Kind: "manual", Scope: "private"}
	for _, table := range []string{"users", "spaces", "file_collections", "collection_members"} {
		hits := 0
		if err := f.Service.DB.Callback().Query().Before("gorm:query").Register("collection_create_dependency", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic creation dependency failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.CreateFileCollection(p, input, "atomic-create")
		f.Service.DB.Callback().Query().Remove("collection_create_dependency")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("dependency failure lost", table, failure)
		}
		var count int64
		if err := f.Service.DB.Model(&service.FileCollection{}).Where("key_id = ?", "atomic-create").Count(&count).Error; err != nil || count != 0 {
			t.Fatal("failed create persisted collection", table, err)
		}
	}
	original, err := f.Service.CreateFileCollection(p, input, "atomic-create")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.Service.CreateFileCollection(p, input, "atomic-create")
	if err != nil || replay.CollectionID != original.CollectionID {
		t.Fatal("retry lost original identity", err)
	}
	input.Name = "Different"
	if _, err = f.Service.CreateFileCollection(p, input, "atomic-create"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed request", err)
	}
}
