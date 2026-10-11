package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionListCurrentAdmissionAndCountFailures(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Private collection", Kind: "manual", Scope: "private"}, "list-admission")
	if err != nil {
		t.Fatal(err)
	}
	var listed []service.FileCollectionView
	if status := h.call(t, "GET", "/file-collections?space_id="+h.space.ID, nil, &listed); status != 200 || len(listed) != 1 || listed[0].CollectionID != collection.CollectionID {
		t.Fatal("valid list failed", status)
	}
	for _, principal := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: "missing"}, {}} {
		if _, err = f.Service.ListFileCollections(principal, h.space.ID); err == nil {
			t.Fatal("collection list crossed current scope")
		}
	}
	for _, table := range []string{"users", "spaces", "file_collections", "collection_members"} {
		hits := 0
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("collection_list_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic collection list dependency failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.ListFileCollections(p, h.space.ID)
		if err = f.Service.DB.Callback().Query().Remove("collection_list_failure"); err != nil {
			t.Fatal(err)
		}
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("list failure became empty or zero", table, failure)
		}
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ListFileCollections(p, h.space.ID); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled user listed collections", err)
	}
}

func TestCollectionListBoundedAndScoped(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	_, prefix := agent(t, h, "limited", "read")
	if err := prefix.JSON(t.Context(), "GET", "/api/v1/file-collections?space_id="+h.space.ID, "", nil, nil); err == nil {
		t.Fatal("prefix agent obtained space-wide collection names/counts")
	}
	first, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Bounded", Kind: "manual", Scope: "private"}, "bounded-list")
	if err != nil {
		t.Fatal(err)
	}
	var base service.FileCollection
	if err = f.Service.DB.First(&base, "id = ?", first.CollectionID).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]service.FileCollection, 1000)
	for i := range rows {
		row := base
		row.ID = service.ID()
		row.KeyID = service.ID()
		rows[i] = row
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ListFileCollections(p, h.space.ID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("unbounded list accepted", err)
	}
	// Foreign tenant candidates must not consume this tenant's scan budget.
	if err = f.Service.DB.Model(&service.FileCollection{}).Where("id <> ?", base.ID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	list, err := f.Service.ListFileCollections(p, h.space.ID)
	if err != nil || len(list) != 1 || list[0].CollectionID != base.ID {
		t.Fatal("foreign tenant rows exposed or charged to budget", err)
	}
}
