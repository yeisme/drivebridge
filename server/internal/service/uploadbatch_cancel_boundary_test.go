package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestUploadBatchCancelRejectsBrokenBinding(t *testing.T) {
	for _, field := range []string{"batch_id", "item_key", "space_id", "size", "sha256", "anchor", "tenant"} {
		t.Run(field, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			b, err := f.Service.CreateUploadBatch(p, service.UploadBatchRequest{SpaceID: h.space.ID}, "binding")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.AddUploadBatchItems(p, b.BatchID, []service.UploadBatchItemInput{{RelativePath: "a.txt", Size: 1, SHA256: service.Hash("a")}}); err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.SealUploadBatch(p, b.BatchID); err != nil {
				t.Fatal(err)
			}
			op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "a.txt", Size: 1, SHA256: service.Hash("a"), BatchID: b.BatchID, ItemKey: "a.txt\x00" + service.Hash("a")}, "item")
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "tenant":
				err = f.Service.DB.Model(&service.UploadBatch{}).Where("id = ?", b.BatchID).Update("tenant_id", "other").Error
			case "anchor":
				err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", b.OperationID).Update("kind", "upload").Error
			default:
				var value any = "other"
				if field == "size" {
					value = int64(2)
				}
				err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update(field, value).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.ControlUploadBatch(p, b.BatchID, "cancel"); err == nil {
				t.Fatal("broken binding authorized cancel")
			}
			var stored service.UploadBatch
			var child service.Operation
			if err = f.Service.DB.First(&stored, "id = ?", b.BatchID).Error; err != nil {
				t.Fatal(err)
			}
			if err = f.Service.DB.First(&child, "id = ?", op.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Phase == "cancelled" || child.Status == "cancelled" {
				t.Fatal("broken binding caused a mutation")
			}
		})
	}
}
