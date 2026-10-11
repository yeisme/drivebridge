package service_test

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceBatchTaskItems(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: alice.user.ID}
	batch, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: alice.space.ID}, "task-items")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Service.AddUploadBatchItems(p, batch.BatchID, []service.UploadBatchItemInput{{RelativePath: "a.txt", Size: 1, SHA256: service.Hash("a")}, {RelativePath: "b.txt", Size: 1, SHA256: service.Hash("b")}})
	if err != nil {
		t.Fatal(err)
	}
	base := "/workspace/tasks/upload_batch/" + batch.BatchID + "/items"
	read := func(query string) contract.BatchTaskItems {
		t.Helper()
		var out contract.BatchTaskItems
		if status := alice.call(t, "GET", base+query, nil, &out); status != 200 {
			t.Fatalf("status %d", status)
		}
		return out
	}
	first := read("?limit=1")
	if len(first.Items) != 1 || first.NextCursor == "" || first.Items[0].OperationID != nil || first.Items[0].ResultCertainty != "unknown" {
		t.Fatal("invented pending operation")
	}
	second := read("?limit=1&cursor=" + url.QueryEscape(first.NextCursor))
	if len(second.Items) != 1 || second.Items[0].ItemID == first.Items[0].ItemID || second.NextCursor != "" || second.Revision != first.Revision {
		t.Fatal("invalid stable item page")
	}
	raw, _ := json.Marshal(first)
	for _, secret := range []string{"item_key", "source_fingerprint", "sha256\":", "error\":", service.Hash("a")} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("private item data leaked")
		}
	}
	if status := bob.call(t, "GET", base, nil, nil); status != 404 {
		t.Fatalf("foreign items %d", status)
	}
	for _, tc := range []struct {
		query  string
		status int
	}{{"?limit=201", 400}, {"?cursor=x&cursor=y", 400}, {"?source=true", 400}, {"?limit=2&cursor=" + url.QueryEscape(first.NextCursor), 410}} {
		if status := alice.call(t, "GET", base+tc.query, nil, nil); status != tc.status {
			t.Fatalf("query status %d want %d", status, tc.status)
		}
	}
	_, client := agent(t, alice, "", "read,write")
	if err := client.JSON(t.Context(), "GET", "/api/v1"+base, "", nil, nil); err == nil {
		t.Fatal("Agent entered human task item projection")
	}
	other, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: alice.space.ID}, "task-items-other")
	if err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks/upload_batch/"+other.BatchID+"/items?limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("cross batch cursor accepted")
	}
	if _, err = f.Service.SealUploadBatch(p, batch.BatchID); err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", base+"?limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("seal kept stale cursor")
	}
	op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: alice.space.ID, Path: "a.txt", Size: 1, SHA256: service.Hash("a"), BatchID: batch.BatchID, ItemKey: "a.txt\x00" + service.Hash("a")}, "task-item-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString("a")); err != nil {
		t.Fatal(err)
	}
	done, err := f.Service.Complete(t.Context(), p, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	all := read("")
	found := false
	for _, item := range all.Items {
		if item.RelativePath == "a.txt" {
			found = item.OperationID != nil && *item.OperationID == op.ID && item.ResultCertainty == "confirmed" && item.OwnerStatus == "completed"
		}
	}
	if !found {
		t.Fatal("published original operation not bound")
	}
	if err = f.Service.DB.Delete(&service.Version{}, "id = ?", done.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	retained := read("")
	if retained.Revision == all.Revision {
		t.Fatal("lost publication evidence retained generation")
	}
	for _, item := range retained.Items {
		if item.RelativePath == "a.txt" && item.ResultCertainty != "unknown" {
			t.Fatal("missing historical result confirmed")
		}
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update("batch_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", base, nil, nil); status != 503 {
		t.Fatalf("wrong operation binding %d", status)
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update("batch_id", batch.BatchID).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Migrator().DropTable(&service.UploadBatchItem{}); err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", base, nil, nil); status != 500 {
		t.Fatalf("items DB fault %d", status)
	}
}
