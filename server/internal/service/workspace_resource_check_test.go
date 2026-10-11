package service_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

func TestResourceIntegrityFixedBytesAndReceipt(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	op := orgUpload(t, c, h.space.ID, "checks/original.md", "# Synthetic\nNever executed.")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	descriptor, err := f.Service.RegisterWorkspaceResource(p, service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "skill", DisplayName: "Encoding only"}, "integrity-register")
	if err != nil {
		t.Fatal(err)
	}
	// New head is invalid UTF-8; the check must still consume the captured version.
	if _, err = c.Upload(t.Context(), sdk.UploadRequest{SpaceID: h.space.ID, Path: file.Path, FileID: file.ID, ExpectedVersion: file.VersionID}, "integrity-new-head", strings.NewReader("\xff"), nil); err != nil {
		t.Fatal(err)
	}
	first, err := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || first.State != "passed" || first.CheckerVersion != "utf8-integrity-v1" || first.CheckID == "" {
		t.Fatal("fixed-byte check failed", err)
	}
	second, err := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || second != first {
		t.Fatal("replay did not preserve immutable fact", err)
	}
	// Descriptors still do not advertise broad content safety from this narrow check.
	view, err := f.Service.WorkspaceResource(p, descriptor.DescriptorID)
	if err != nil || view.CheckState != "unknown" {
		t.Fatal("encoding became general safety", err)
	}
	for _, principal := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, Agent: &service.Agent{ID: "agent"}}} {
		if _, err = f.Service.CheckResourceIntegrity(t.Context(), principal, descriptor.DescriptorID, 1); err == nil {
			t.Fatal("receipt bypassed identity")
		}
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 2); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale expected revision accepted", err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil {
		t.Fatal("historical check bypassed current access")
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceResourceCheck{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("check replay duplicated fact", err)
	}
}

func TestResourceIntegrityInvalidEncodingAndAtomicAudit(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	for _, tc := range []struct{ name, body, state string }{{"utf8", "bad\xff", "failed"}, {"nul", "bad\x00text", "failed"}, {"empty", "", "passed"}} {
		op := orgUpload(t, c, h.space.ID, "checks/"+tc.name+".txt", tc.body)
		var file service.File
		if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
			t.Fatal(err)
		}
		descriptor, err := f.Service.RegisterWorkspaceResource(p, service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "template", DisplayName: tc.name}, "integrity-"+tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.Service.DB.Callback().Create().Before("gorm:create").Register("check_audit_failure", func(tx *gorm.DB) {
			if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.resource.check" {
				tx.AddError(errors.New("synthetic check audit failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
		f.Service.DB.Callback().Create().Remove("check_audit_failure")
		if failure == nil || failure.Error() != "audit_unavailable" {
			t.Fatal("check audit failure ignored", failure)
		}
		var count int64
		if err = f.Service.DB.Model(&service.WorkspaceResourceCheck{}).Where("descriptor_id = ?", descriptor.DescriptorID).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("audit failure retained check", err)
		}
		result, err := f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
		if err != nil || result.State != tc.state {
			t.Fatal("wrong encoding outcome", tc.name, err)
		}
	}
}

func TestResourceIntegrityBudgetAndTargetDrift(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c := mustSDK(t, h)
	op := orgUpload(t, c, h.space.ID, "checks/budget.txt", "budget fixture")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	descriptor, err := f.Service.RegisterWorkspaceResource(p, service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "template", DisplayName: "Budget"}, "check-budget")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", op.VersionID).Update("size", (2<<20)+1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("byte budget ignored", err)
	}
	if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", op.VersionID).Update("size", file.Size).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", op.VersionID).Update("sha256", strings.Repeat("a", 64)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil || err.Error() != "dependency_unavailable" {
		t.Fatal("bytes were not verified against fixed hash", err)
	}
	var checks int64
	if err = f.Service.DB.Model(&service.WorkspaceResourceCheck{}).Count(&checks).Error; err != nil || checks != 0 {
		t.Fatal("unverified bytes produced a receipt", err)
	}
	if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", op.VersionID).Update("sha256", service.Hash("budget fixture")).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", descriptor.DescriptorID).Update("display_name", "Drift").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("old receipt attached to changed descriptor", err)
	}
}
