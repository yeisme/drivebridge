package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestSnapshotEmptyCollectionAdmission(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Empty", Kind: "manual", Scope: "private"}, "empty-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: "other"}} {
		if _, err := f.Service.CreateFileSnapshot(other, collection.CollectionID); err == nil {
			t.Fatal("empty snapshot bypassed scope")
		}
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateFileSnapshot(p, collection.CollectionID); err == nil {
		t.Fatal("disabled user froze empty snapshot")
	}
	var count int64
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("unauthorized snapshot persisted", err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil || result.EntriesTotal != 0 || result.Items == nil || len(result.Items) != 0 {
		t.Fatal("authorized empty snapshot contract regressed", err)
	}
}
