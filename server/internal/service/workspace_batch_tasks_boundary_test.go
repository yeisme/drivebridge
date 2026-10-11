package service_test

import (
	"fmt"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceBatchTaskBoundaries(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	batch, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "boundary")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ControlUploadBatch(p, batch.BatchID, "cancel"); err != nil {
		t.Fatal(err)
	}
	var page contract.Page[contract.TaskProjection]
	if status := h.call(t, "GET", "/workspace/tasks?kind=upload_batch&state=cancelled", nil, &page); status != 200 || len(page.Items) != 1 || page.Items[0].UIState != "cancelled" {
		t.Fatal("cancelled batch projection missing")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("agent_id", "authenticated-agent").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=upload_batch&agent_id=authenticated-agent", nil, &page); status != 200 || len(page.Items) != 1 || page.Items[0].AgentID == nil || *page.Items[0].AgentID != "authenticated-agent" {
		t.Fatal("anchor actor filter drift")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("space_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=upload_batch", nil, nil); status != 503 {
		t.Fatalf("mismatched anchor status %d", status)
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("space_id", h.space.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.UploadBatch{}).Where("id = ?", batch.BatchID).Update("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=upload_batch", nil, &page); status != 200 || len(page.Items) != 0 {
		t.Fatal("foreign tenant batch leaked")
	}
}

func TestWorkspaceBatchTaskRequestItemBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	// Two individually small batches jointly exceed the one-request budget.
	for index, count := range []int{600, 401} {
		batch, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, fmt.Sprintf("budget-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]service.UploadBatchItem, count)
		for i := range rows {
			rows[i] = service.UploadBatchItem{ID: service.ID(), Owner: h.user.ID, BatchID: batch.BatchID, ItemKey: fmt.Sprintf("item-%d", i), RelativePath: fmt.Sprintf("file-%d", i), Status: "pending"}
		}
		if err := f.Service.DB.CreateInBatches(rows, 50).Error; err != nil {
			t.Fatal(err)
		}
	}
	if status := h.call(t, "GET", "/workspace/tasks?kind=upload_batch&state=completed", nil, nil); status != 429 {
		t.Fatalf("aggregate budget status %d", status)
	}
}
