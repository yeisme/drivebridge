package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionReadAdmissionAndPrefixAgent(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Scoped", Kind: "manual", Scope: "private"}, "scoped")
	if err != nil {
		t.Fatal(err)
	}
	for _, principal := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: "other"}} {
		if _, err = f.Service.ReadFileCollection(principal, collection.CollectionID); err == nil {
			t.Fatal("empty collection leaked across identity or scope")
		}
	}
	_, client := agent(t, h, "allowed", "read")
	if err = client.JSON(t.Context(), "GET", "/api/v1/file-collections/"+collection.CollectionID, "", nil, nil); err != nil {
		t.Fatal("valid prefix reader cannot read empty collection", err)
	}
	a := orgUpload(t, c, h.space.ID, "allowed/a.txt", "a")
	b := orgUpload(t, c, h.space.ID, "outside/b.txt", "b")
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: collection.Revision, Include: []string{a.FileID, b.FileID}}); err != nil {
		t.Fatal(err)
	}
	var view service.CollectionReadView
	if err = client.JSON(t.Context(), "GET", "/api/v1/file-collections/"+collection.CollectionID, "", nil, &view); err != nil || len(view.Items) != 1 || view.Items[0].FileID != a.FileID {
		t.Fatal("prefix reader not filtered to its members", err)
	}
	if err = f.Service.DB.Model(&service.Agent{}).Where("owner = ?", p.UserID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if err = client.JSON(t.Context(), "GET", "/api/v1/file-collections/"+collection.CollectionID, "", nil, nil); err == nil {
		t.Fatal("revoked reader admitted")
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadFileCollection(p, collection.CollectionID); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled actor read collection")
	}
}

func TestCollectionReadAdmissionDatabaseErrors(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "DB", Kind: "manual", Scope: "private"}, "db")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"users", "file_collections", "spaces"} {
		hits := 0
		name := "collection_auth_db"
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic authorization dependency failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.ReadFileCollection(p, collection.CollectionID)
		if err = f.Service.DB.Callback().Query().Remove(name); err != nil {
			t.Fatal(err)
		}
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("authorization dependency failure hidden", table, failure)
		}
	}
}

func TestFileQueryCatalogueFailureIsNotEmptyGeneration(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	for _, table := range []string{"files", "file_label_assignments"} {
		hits := 0
		name := "catalogue_generation_db"
		if err := f.Service.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic generation dependency failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.QueryFiles(p, service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}})
		if err := f.Service.DB.Callback().Query().Remove(name); err != nil {
			t.Fatal(err)
		}
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("catalogue failure hidden", table, failure)
		}
	}
}
