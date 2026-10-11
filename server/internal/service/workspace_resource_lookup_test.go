package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceResourceLookupOriginalReceiptAndNoWrites(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, mustSDK(t, h), h.space.ID, "resource/lookup.txt", "synthetic lookup")
	var file service.File
	if err := f.Service.DB.First(&file, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	input := service.RegisterWorkspaceResourceRequest{SpaceID: h.space.ID, FileRef: f.Service.FileRef(file), Kind: "template", DisplayName: "Lookup"}
	original, err := f.Service.RegisterWorkspaceResource(p, input, "lookup-original")
	if err != nil {
		t.Fatal(err)
	}
	var stored service.WorkspaceRequest
	if err = f.Service.DB.Where("action = ? AND request_key = ?", "resource.register", "lookup-original").First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"idempotency_key": "lookup-original"}
	// Guard every GORM write, including the generic HTTP audit/usage layer.
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("lookup attempted write")) }
	if err = f.Service.DB.Callback().Create().Before("gorm:create").Register("lookup_no_create", guard); err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Callback().Update().Before("gorm:update").Register("lookup_no_update", guard); err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Callback().Delete().Before("gorm:delete").Register("lookup_no_delete", guard); err != nil {
		t.Fatal(err)
	}
	var receipt contract.ResourceRegistrationReceipt
	status := h.call(t, "POST", "/resource-descriptors/lookup", body, &receipt)
	f.Service.DB.Callback().Create().Remove("lookup_no_create")
	f.Service.DB.Callback().Update().Remove("lookup_no_update")
	f.Service.DB.Callback().Delete().Remove("lookup_no_delete")
	if status != 200 || writes != 0 || receipt.Resource != original || receipt.ResultRevision != stored.ResultRevision || !receipt.CommittedAt.Equal(stored.CreatedAt) {
		t.Fatal("lookup not original read-only receipt", status, writes)
	}
	if status = bob.call(t, "POST", "/resource-descriptors/lookup", body, nil); status != 404 {
		t.Fatal("foreign key found", status)
	}
	if status = h.call(t, "POST", "/resource-descriptors/lookup", map[string]string{"idempotency_key": "missing"}, nil); status != 404 {
		t.Fatal("missing receipt not distinguished", status)
	}
	for _, raw := range []string{`{}`, `{"idempotency_key":null}`, `{"idempotency_key":"x","idempotency_key":"y"}`, `{"idempotency_key":"x","owner":"spoof"}`, `{"idempotency_key":"bad key"}`} {
		if status, _ := fpRawCall(t, h, "POST", "/resource-descriptors/lookup", []byte(raw)); status != 400 {
			t.Fatal("closed lookup accepted", status)
		}
	}
	// A normal current detail cannot prove which original request committed.
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", original.DescriptorID).Update("display_name", "Drift").Error; err != nil {
		t.Fatal(err)
	}
	if status = h.call(t, "POST", "/resource-descriptors/lookup", body, nil); status != 409 {
		t.Fatal("current row masqueraded as original receipt", status)
	}
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Where("id = ?", original.DescriptorID).Update("display_name", input.DisplayName).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"workspace_requests", "workspace_resource_descriptors", "versions"} {
		hits := 0
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("lookup_db_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic lookup source failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		status = h.call(t, "POST", "/resource-descriptors/lookup", body, nil)
		f.Service.DB.Callback().Query().Remove("lookup_db_failure")
		if hits == 0 || status != 500 {
			t.Fatal("DB failure mistaken for missing", table, status)
		}
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if status = h.call(t, "POST", "/resource-descriptors/lookup", body, nil); status != 404 {
		t.Fatal("receipt bypassed current target authority", status)
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceResourceDescriptor{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("lookup allocated descriptor", err)
	}
}
