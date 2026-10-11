package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionReadMemberBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Budget", Kind: "manual", Scope: "private"}, "budget")
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]service.CollectionMember, 10001)
	for i := range rows {
		rows[i] = service.CollectionMember{ID: service.ID(), CollectionID: collection.CollectionID, FileID: service.ID(), Owner: h.user.ID, Decision: "include"}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadFileCollection(p, collection.CollectionID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("oversized member scan not refused")
	}
}

func TestCollectionReadIsObservationOnly(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	op := orgUpload(t, c, h.space.ID, "collection/read.txt", "original")
	collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Read only", Kind: "manual", Scope: "private"}, "readonly")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: collection.Revision, Include: []string{op.FileID}}); err != nil {
		t.Fatal(err)
	}
	var before service.FileCollection
	if err = f.Service.DB.First(&before, "id = ?", collection.CollectionID).Error; err != nil {
		t.Fatal(err)
	}
	writes := 0
	callback := "collection_read_no_writes"
	if err = f.Service.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) { writes++; tx.AddError(errors.New("read attempted mutation")) }); err != nil {
		t.Fatal(err)
	}
	read, readErr := f.Service.ReadFileCollection(p, collection.CollectionID)
	if err = f.Service.DB.Callback().Update().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || writes != 0 || read.VisibleCount != 1 || read.Items[0].VersionID != op.VersionID || read.ComputedAt.IsZero() {
		t.Fatal("read mutated domain or lost original result")
	}
	var after service.FileCollection
	if err = f.Service.DB.First(&after, "id = ?", collection.CollectionID).Error; err != nil {
		t.Fatal(err)
	}
	if before.Revision != after.Revision || !before.UpdatedAt.Equal(after.UpdatedAt) || after.ComputedAt != nil {
		t.Fatal("GET persisted computation")
	}
}

func TestCollectionReadQueryFailureIsNotEmpty(t *testing.T) {
	for _, kind := range []string{"manual", "rule"} {
		t.Run(kind, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			c := mustSDK(t, h)
			op := orgUpload(t, c, h.space.ID, "query/a.txt", "a")
			rule := ""
			if kind == "rule" {
				rule = `{"mode":"exact"}`
			}
			collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Failure", Kind: kind, Scope: "private", Rule: json.RawMessage(rule)}, "failure")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.SetCollectionMembers(p, collection.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: collection.Revision, Include: []string{op.FileID}}); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"collection_members", "files"} {
				hits := 0
				name := "collection_read_query_fault"
				if err = f.Service.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						hits++
						tx.AddError(errors.New("synthetic query failure"))
					}
				}); err != nil {
					t.Fatal(err)
				}
				_, readErr := f.Service.ReadFileCollection(p, collection.CollectionID)
				if err = f.Service.DB.Callback().Query().Remove(name); err != nil {
					t.Fatal(err)
				}
				if hits == 0 || readErr == nil || readErr.Error() != "database_unavailable" {
					t.Fatalf("%s failure became empty success: %v", table, readErr)
				}
			}
			if kind == "rule" {
				for _, bad := range []string{"", `{broken`, `{"mode":"exact","filters":[{"field":"unsupported","op":"eq","value":"x"}]}`} {
					if err = f.Service.DB.Model(&service.FileCollection{}).Where("id = ?", collection.CollectionID).Update("rule", bad).Error; err != nil {
						t.Fatal(err)
					}
					if _, err = f.Service.ReadFileCollection(p, collection.CollectionID); err == nil || err.Error() != "invalid_query" {
						t.Fatal("bad rule expanded to whole space")
					}
				}
			}
		})
	}
}
