package service_test

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceBatchTaskProjection(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: alice.user.ID}
	batch, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: alice.space.ID}, "batch-task")
	if err != nil {
		t.Fatal(err)
	}
	get := func(query string) contract.Page[contract.TaskProjection] {
		t.Helper()
		var page contract.Page[contract.TaskProjection]
		if status := alice.call(t, "GET", "/workspace/tasks?kind=upload_batch"+query, nil, &page); status != 200 {
			t.Fatalf("status %d", status)
		}
		return page
	}
	read := func() contract.TaskProjection {
		t.Helper()
		page := get("")
		if len(page.Items) != 1 {
			t.Fatal("missing original batch")
		}
		return page.Items[0]
	}
	initial := read()
	if initial.OwnerKind != "upload_batch" || initial.OwnerID != batch.BatchID || initial.OwnerStatus != "enumerating" || initial.Progress.Total != nil || initial.ActorKind != "human" {
		t.Fatal("enumeration invented total or identity")
	}
	_, err = f.Service.AddUploadBatchItems(p, batch.BatchID, []service.UploadBatchItemInput{{RelativePath: "a.txt", Size: 1, SHA256: service.Hash("a")}, {RelativePath: "b.txt", Size: 1, SHA256: service.Hash("b")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SealUploadBatch(p, batch.BatchID); err != nil {
		t.Fatal(err)
	}
	sealed := read()
	if sealed.Progress.Total == nil || *sealed.Progress.Total != 2 || sealed.Progress.Pending == nil || *sealed.Progress.Pending != 2 {
		t.Fatal("sealed counters incorrect")
	}
	item := service.UploadRequest{SpaceID: alice.space.ID, Path: "a.txt", Size: 1, SHA256: service.Hash("a"), BatchID: batch.BatchID, ItemKey: "a.txt\x00" + service.Hash("a")}
	op, err := f.Service.CreateUpload(p, item, "batch-task-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString("a")); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.Complete(t.Context(), p, op.ID); err != nil {
		t.Fatal(err)
	}
	partial := read()
	if partial.UIState == "completed" || partial.Progress.Succeeded == nil || *partial.Progress.Succeeded != 1 || *partial.Progress.Pending != 1 || partial.Revision == sealed.Revision {
		t.Fatal("partial batch displayed complete")
	}
	// Simulate a failed sibling and the existing owner's terminal phase; no
	// fake item is counted as success merely because the anchor is completed.
	if err = f.Service.DB.Model(&service.UploadBatchItem{}).Where("batch_id = ? AND relative_path = ?", batch.BatchID, "b.txt").Update("status", "failed").Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.UploadBatch{}).Where("id = ?", batch.BatchID).Update("phase", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	failed := read()
	if failed.OwnerStatus != "completed" || failed.UIState != "recoverable" || *failed.Progress.Failed != 1 || *failed.Progress.Succeeded != 1 {
		t.Fatal("failed batch claimed all-success")
	}
	var anchors contract.Page[contract.TaskProjection]
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation", nil, &anchors); status != 200 {
		t.Fatal(status)
	}
	for _, v := range anchors.Items {
		if v.OwnerID == batch.OperationID && (v.UIState != "unknown" || v.BlockedReason != "batch_projection_required") {
			t.Fatal("anchor claimed all-success")
		}
	}
	if len(get("&state=completed").Items) != 0 || len(get("&agent_id=untrusted-caller").Items) != 0 {
		t.Fatal("filter invented match")
	}
	var foreign contract.Page[contract.TaskProjection]
	if status := bob.call(t, "GET", "/workspace/tasks?kind=upload_batch", nil, &foreign); status != 200 || len(foreign.Items) != 0 {
		t.Fatal("foreign batch leaked")
	}
	// Restore a schedulable sibling through disposable metadata, then finish
	// its real upload to prove all-success derives from fixed owner outputs.
	if err = f.Service.DB.Model(&service.UploadBatch{}).Where("id = ?", batch.BatchID).Update("phase", "transferring").Error; err != nil {
		t.Fatal(err)
	}
	item.Path = "b.txt"
	item.SHA256 = service.Hash("b")
	item.ItemKey = "b.txt\x00" + item.SHA256
	second, err := f.Service.CreateUpload(p, item, "batch-task-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, second.ID, 0, bytes.NewBufferString("b")); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.Complete(t.Context(), p, second.ID); err != nil {
		t.Fatal(err)
	}
	complete := read()
	if complete.UIState != "completed" || *complete.Progress.Succeeded != 2 || *complete.Progress.Failed != 0 || *complete.Progress.Pending != 0 || complete.Progress.BytesSent != nil || complete.Progress.DurableBytes != nil || complete.Progress.VerifiedBytes != nil {
		t.Fatal("published batch counters incorrect")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("status", "running").Error; err != nil {
		t.Fatal(err)
	}
	if inconsistent := read(); inconsistent.UIState != "unknown" {
		t.Fatal("batch phase alone implied completion")
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", batch.OperationID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: alice.space.ID}, "batch-second"); err != nil {
		t.Fatal(err)
	}
	page := get("&limit=1")
	if page.NextCursor == "" {
		t.Fatal("missing cursor")
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation&limit=1&cursor="+url.QueryEscape(page.NextCursor), nil, nil); status != 410 {
		t.Fatal("cross-kind cursor accepted")
	}
	if err = f.Service.DB.Model(&service.UploadBatchItem{}).Where("batch_id = ? AND relative_path = ?", batch.BatchID, "a.txt").Update("certainty", "unknown").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=upload_batch&limit=1&cursor="+url.QueryEscape(page.NextCursor), nil, nil); status != 410 {
		t.Fatal("changed certainty retained cursor")
	}
	unknown := get("&state=unknown")
	if len(unknown.Items) != 1 || unknown.Items[0].Progress.Succeeded != nil {
		t.Fatal("uncertain success invented")
	}
	if err = f.Service.DB.Migrator().DropTable(&service.UploadBatchItem{}); err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=upload_batch", nil, nil); status != 500 {
		t.Fatalf("items failure status %d", status)
	}
}
