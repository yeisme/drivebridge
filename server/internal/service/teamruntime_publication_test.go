package service

import (
	"gorm.io/gorm"
	"testing"
)

func TestOutboxPublicationOnlyTouchesCurrentBatch(t *testing.T) {
	s, _, _ := blobRig(t)
	db := s.DB
	cp := ProjectionCheckpoint{Projection: "file_catalogue", OwnerScope: "owner", Generation: "GEN_0", Watermark: 1}
	if err := db.Create(&cp).Error; err != nil {
		t.Fatal(err)
	}
	events := []OutboxEvent{{EventID: "old", Kind: "file_catalogue.upsert", ResourceID: "old", OwnerScope: "owner", SourceSequence: 1, Published: true}, {EventID: "next", Kind: "file_catalogue.upsert", ResourceID: "next", OwnerScope: "owner", SourceSequence: 2, ResourceRevision: 2}, {EventID: "later", Kind: "file_catalogue.upsert", ResourceID: "later", OwnerScope: "owner", SourceSequence: 3, ResourceRevision: 3}}
	if err := db.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	var touched int64
	if err := db.Callback().Update().After("gorm:update").Register("count-publication", func(tx *gorm.DB) {
		if tx.Statement.Table == "runtime_outbox" {
			touched += tx.RowsAffected
		}
	}); err != nil {
		t.Fatal(err)
	}
	count, watermark, err := s.ApplyOutboxEvents("file_catalogue", "owner", 1)
	if err != nil || count != 1 || watermark != 2 || touched != 1 {
		t.Fatalf("batch publication: count=%d watermark=%d writes=%d err=%v", count, watermark, touched, err)
	}
	var later OutboxEvent
	if err = db.First(&later, "event_id = ?", "later").Error; err != nil || later.Published {
		t.Fatal("published beyond the committed batch")
	}
	if _, _, err = s.ApplyOutboxEvents("file_catalogue", "owner", 1); err != nil || touched != 2 {
		t.Fatal("second batch rewrote publication history")
	}
	if _, _, err = s.ApplyOutboxEvents("file_catalogue", "owner", 1); err != nil || touched != 2 {
		t.Fatal("empty replay repeated publication")
	}
}
