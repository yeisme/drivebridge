package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceResourceFixedFileAndReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, c, h.space.ID, "resource/SKILL.md", "# Synthetic skill\nNever execute this fixture.")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	input := service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "skill", DisplayName: "Fixture skill"}
	view, err := f.Service.RegisterWorkspaceResource(p, input, "resource-original")
	if err != nil || view.FileRef != input.FileRef || view.CheckState != "unknown" {
		t.Fatal("registration failed", err)
	}
	replay, err := f.Service.RegisterWorkspaceResource(p, input, "resource-original")
	if err != nil || replay != view {
		t.Fatal("original replay changed", err)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", view.DescriptorID).Update("display_name", "Drift").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.RegisterWorkspaceResource(p, input, "resource-original"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("drifted descriptor masqueraded as original receipt", err)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", view.DescriptorID).Update("display_name", input.DisplayName).Error; err != nil {
		t.Fatal(err)
	}
	for _, bad := range []service.RegisterWorkspaceResourceRequest{
		{SpaceID: h.space.ID, Kind: "skill", DisplayName: "Empty"},
		{SpaceID: h.space.ID, Kind: "skill", DisplayName: "Both", FileRef: input.FileRef, SnapshotID: "snapshot"},
		{SpaceID: h.space.ID, Kind: "executable", DisplayName: "Wrong", FileRef: input.FileRef},
		{SpaceID: h.space.ID, Kind: "reference_pack", DisplayName: "Not a pack", FileRef: input.FileRef},
		{SpaceID: h.space.ID, Kind: "skill", DisplayName: "bad\nname", FileRef: input.FileRef},
	} {
		if _, err = f.Service.RegisterWorkspaceResource(p, bad, "invalid"); err == nil {
			t.Fatal("invalid descriptor admitted")
		}
	}
	changed := input
	changed.DisplayName = "Other"
	if _, err = f.Service.RegisterWorkspaceResource(p, changed, "resource-original"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("key conflict ignored", err)
	}
	for _, other := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: p.UserID, Agent: &service.Agent{ID: "agent"}}} {
		if _, err = f.Service.WorkspaceResource(other, view.DescriptorID); err == nil {
			t.Fatal("descriptor crossed authority")
		}
		if _, err = f.Service.RegisterWorkspaceResource(other, input, "resource-original"); err == nil {
			t.Fatal("registration crossed authority")
		}
	}
	// Advance the head with the real upload CAS, leaving the descriptor fixed.
	replacement, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: file.Path, FileID: file.ID, ExpectedVersion: file.VersionID, Size: 0, SHA256: service.Hash("")}, "resource-new-head")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.Complete(t.Context(), p, replacement.ID); err != nil {
		t.Fatal(err)
	}
	reread, err := f.Service.WorkspaceResource(p, view.DescriptorID)
	if err != nil || reread.FileRef != input.FileRef {
		t.Fatal("descriptor followed latest head", err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.RegisterWorkspaceResource(p, input, "resource-original"); err == nil {
		t.Fatal("receipt bypassed current access")
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("replay created descriptor", err)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceCheck{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("registration invented check", err)
	}
}

func TestWorkspaceResourceSnapshotAndAtomicFailure(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	op := orgUpload(t, c, h.space.ID, "pack/reference.txt", "synthetic reference")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Pack", Kind: "manual", Scope: "private"}, "resource-collection")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: collection.Revision, Include: []string{op.FileID}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.Service.CreateFileSnapshot(p, collection.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	input := service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, SnapshotID: snapshot.SnapshotID, Kind: "reference_pack", DisplayName: "Fixed pack"}
	const fault = "resource_audit_failure"
	if err = f.Service.DB.Callback().Create().Before("gorm:create").Register(fault, func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*service.Audit); ok && row.Action == "workspace.resource.register" {
			tx.AddError(errors.New("synthetic audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure := f.Service.RegisterWorkspaceResource(p, input, "pack-original")
	f.Service.DB.Callback().Create().Remove(fault)
	if failure == nil || failure.Error() != "audit_unavailable" {
		t.Fatal("audit failure not propagated", failure)
	}
	for _, model := range []any{&service.WorkspaceResourceDescriptor{}, &service.WorkspaceRequest{}} {
		var count int64
		query := f.Service.DB.Model(model)
		if _, ok := model.(*service.WorkspaceRequest); ok {
			query = query.Where("action = ?", "resource.register")
		}
		if err = query.Count(&count).Error; err != nil || count != 0 {
			t.Fatal("failed register retained partial state", err)
		}
	}
	view, err := f.Service.RegisterWorkspaceResource(p, input, "pack-original")
	if err != nil || view.SnapshotID != snapshot.SnapshotID || view.FileRef != "" {
		t.Fatal("snapshot registration failed", err)
	}
	for _, table := range []string{"workspace_resource_descriptors", "collection_snapshots", "collection_snapshot_entries", "files", "versions"} {
		hits := 0
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("resource_read_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic read failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure = f.Service.WorkspaceResource(p, view.DescriptorID)
		f.Service.DB.Callback().Query().Remove("resource_read_failure")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("source failure ignored", table, failure)
		}
	}
	if err = f.Service.DB.Model(&service.CollectionSnapshot{}).Where("id = ?", snapshot.SnapshotID).Update("entries_total", 2).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.WorkspaceResource(p, view.DescriptorID); err == nil || err.Error() != "dependency_unavailable" {
		t.Fatal("partial snapshot admitted", err)
	}
}
