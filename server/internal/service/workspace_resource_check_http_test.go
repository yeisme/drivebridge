package service_test

import (
	"errors"
	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
	"testing"
)

func TestResourceIntegrityHTTPExecuteAndLookup(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, mustSDK(t, h), h.space.ID, "check/http.txt", "synthetic check HTTP")
	descriptor, err := f.Service.RegisterWorkspaceResource(p, service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(service.File{ID: op.FileID, VersionID: op.VersionID}), Kind: "template", DisplayName: "HTTP encoding"}, "http-check-resource")
	if err != nil {
		t.Fatal(err)
	}
	path := "/resource-descriptors/" + descriptor.DescriptorID + "/checks"
	input := contract.ResourceCheckRequest{ExpectedRevision: 1, CheckerVersion: contract.ResourceIntegrityChecker}
	if status := h.call(t, "POST", path+"/lookup", input, nil); status != 404 {
		t.Fatal("missing lookup started check", status)
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceResourceCheck{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("missing lookup mutated checks", err)
	}
	var first contract.ResourceIntegrityReceipt
	if status := h.call(t, "POST", path, input, &first); status != 200 || first.State != "passed" || first.CheckScope != contract.ResourceIntegrityScope || first.DescriptorID != descriptor.DescriptorID || first.CheckID == "" {
		t.Fatal("explicit check mismatch", status)
	}
	// A read-only receipt is available despite provider unavailability: this
	// is historical evidence, not a new storage-health claim.
	f.S3.Close()
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("receipt lookup attempted write")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("integrity_lookup_create", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("integrity_lookup_update", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("integrity_lookup_delete", guard)
	var found contract.ResourceIntegrityReceipt
	status := h.call(t, "POST", path+"/lookup", input, &found)
	f.Service.DB.Callback().Create().Remove("integrity_lookup_create")
	f.Service.DB.Callback().Update().Remove("integrity_lookup_update")
	f.Service.DB.Callback().Delete().Remove("integrity_lookup_delete")
	if status != 200 || found != first || writes != 0 {
		t.Fatal("lookup not immutable/read-only", status, writes)
	}
	var replay contract.ResourceIntegrityReceipt
	if status = h.call(t, "POST", path, input, &replay); status != 200 || replay != first {
		t.Fatal("same check identity reexecuted", status)
	}
	if status = bob.call(t, "POST", path+"/lookup", input, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	for _, suffix := range []string{"", "/lookup"} {
		for _, raw := range []string{`{}`, `{"expected_revision":1,"checker_version":"execute"}`, `{"expected_revision":0,"checker_version":"utf8-integrity-v1"}`, `{"expected_revision":1,"checker_version":null}`, `{"expected_revision":1,"checker_version":"utf8-integrity-v1","expected_revision":2}`, `{"expected_revision":1,"checker_version":"utf8-integrity-v1","owner":"spoof"}`} {
			if status, _ := fpRawCall(t, h, "POST", path+suffix, []byte(raw)); status != 400 {
				t.Fatal("invalid closed check request", status)
			}
		}
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", descriptor.DescriptorID).Update("display_name", "Drift").Error; err != nil {
		t.Fatal(err)
	}
	if status = h.call(t, "POST", path+"/lookup", input, nil); status != 409 {
		t.Fatal("target drift reused evidence", status)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", descriptor.DescriptorID).Update("display_name", descriptor.DisplayName).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("integrity_lookup_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_resource_checks" {
			tx.AddError(errors.New("synthetic check read failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status = h.call(t, "POST", path+"/lookup", input, nil)
	f.Service.DB.Callback().Query().Remove("integrity_lookup_failure")
	if status != 500 {
		t.Fatal("DB failure mistaken for missing", status)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if status = h.call(t, "POST", path+"/lookup", input, nil); status != 404 {
		t.Fatal("lookup bypassed current permission", status)
	}
}
