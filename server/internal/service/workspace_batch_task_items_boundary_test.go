package service_test

import (
	"fmt"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceBatchTaskItemsBudgetAndIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	batch, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "items-budget")
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]service.UploadBatchItem, 1001)
	for i := range rows {
		rows[i] = service.UploadBatchItem{ID: service.ID(), Owner: h.user.ID, BatchID: batch.BatchID, ItemKey: fmt.Sprintf("item-%d", i), RelativePath: fmt.Sprintf("file-%d", i), Status: "pending"}
	}
	if err = f.Service.DB.CreateInBatches(rows, 50).Error; err != nil {
		t.Fatal(err)
	}
	path := "/workspace/tasks/upload_batch/" + batch.BatchID + "/items?limit=1"
	if status := h.call(t, "GET", path, nil, nil); status != 429 {
		t.Fatalf("budget status %d", status)
	}
	if err = f.Service.DB.Delete(&rows[1000]).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", path, nil, nil); status != 200 {
		t.Fatalf("bounded result %d", status)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", path, nil, nil); status != 403 {
		t.Fatalf("disabled user %d", status)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.UploadBatch{}).Where("id = ?", batch.BatchID).Update("tenant_id", "other").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatalf("cross tenant %d", status)
	}
}
