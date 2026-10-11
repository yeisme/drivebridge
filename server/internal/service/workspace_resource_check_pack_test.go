package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestResourceIntegrityPackTotalBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	first := orgUpload(t, c, h.space.ID, "checks/pack-a.txt", "first pack member")
	second := orgUpload(t, c, h.space.ID, "checks/pack-b.txt", "second\x00pack member")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Check pack", Kind: "manual", Scope: "private"}, "check-pack")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{first.FileID, second.FileID}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := f.Service.RegisterWorkspaceResource(p, service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, SnapshotID: snapshot.SnapshotID, Kind: "reference_pack", DisplayName: "Encoding pack"}, "encoding-pack")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.Version{}).Where("id IN ?", []string{first.VersionID, second.VersionID}).Update("size", (1<<20)+1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("budget reset per member", err)
	}
	for id, size := range map[string]int{first.VersionID: len("first pack member"), second.VersionID: len("second\x00pack member")} {
		if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", id).Update("size", size).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || result.State != "failed" {
		t.Fatal("one invalid member not reflected", err)
	}
	replay, err := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || replay != result {
		t.Fatal("pack receipt changed", err)
	}
}
