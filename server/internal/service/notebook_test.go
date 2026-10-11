package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
	"testing"
)

func TestNotebookCreationRelationsIsolationAndRestart(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	q := contract.NotebookCreate{SpaceID: h.space.ID, Title: "父页面", Icon: "book"}
	parent, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, q, "note-parent")
	if e != nil || parent.Status != "completed" || parent.Page == nil {
		t.Fatal(parent, e)
	}
	childQ := q
	childQ.Title = "子页面"
	childQ.ParentFileID = parent.Page.FileID
	child, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, childQ, "note-child")
	if e != nil || child.Page == nil || child.Page.ParentFileID != parent.Page.FileID {
		t.Fatal(child, e)
	}
	replay, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, q, "note-parent")
	if e != nil || replay.OperationID != parent.OperationID {
		t.Fatal(replay, e)
	}
	q.Title = "changed"
	if _, e = f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, q, "note-parent"); e == nil {
		t.Fatal("changed request accepted")
	}
	cycle := contract.NotebookPagePatch{ExpectedRevision: 1, ParentFileID: child.Page.FileID, Icon: "book"}
	if _, e = f.Service.PatchNotebookPage(t.Context(), p, parent.Page.FileID, cycle, "note-cycle"); e == nil {
		t.Fatal("cycle accepted")
	}
	patch := contract.NotebookPagePatch{ExpectedRevision: 1, Icon: "idea"}
	receipt, e := f.Service.PatchNotebookPage(t.Context(), p, child.Page.FileID, patch, "note-move")
	if e != nil || receipt.ResultRevision != 2 {
		t.Fatal(receipt, e)
	}
	if _, e = f.Service.PatchNotebookPage(t.Context(), p, child.Page.FileID, patch, "stale-move"); e == nil {
		t.Fatal("stale revision accepted")
	}
	same, e := f.Service.PatchNotebookPage(t.Context(), p, child.Page.FileID, patch, "note-move")
	if e != nil || same != receipt {
		t.Fatal(same, e)
	}
	page, e := f.Service.NotebookPage(t.Context(), p, child.Page.FileID)
	if e != nil || page.Path != child.Page.Path || page.VersionID != child.Page.VersionID || page.ParentFileID != "" {
		t.Fatal("metadata moved bytes", page, e)
	}
	for _, other := range []service.Principal{{UserID: bob.user.ID}, {UserID: h.user.ID, TenantID: "foreign"}, {UserID: h.user.ID, ExternalRead: true}, {UserID: h.user.ID, ScopeSpace: "foreign"}} {
		if _, e = f.Service.NotebookPage(t.Context(), other, parent.Page.FileID); e == nil {
			t.Fatal("foreign page visible")
		}
		if _, e = f.Service.LookupNotebookCreation(t.Context(), other, h.space.ID, contract.NotebookCreateLookup{IdempotencyKey: "note-parent"}); e == nil {
			t.Fatal("foreign creation visible")
		}
	}
	dailyQ := contract.NotebookCreate{SpaceID: h.space.ID, DailyDate: "2026-10-10"}
	daily, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, dailyQ, "daily-one")
	if e != nil {
		t.Fatal(e)
	}
	daily2, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, dailyQ, "daily-two")
	if e != nil || daily2.OperationID != daily.OperationID {
		t.Fatal("duplicate diary", e)
	}
	rows, e := f.Service.NotebookPages(t.Context(), p, h.space.ID, contract.PageQuery{Limit: 1})
	if e != nil || len(rows.Items) != 1 || rows.NextCursor == "" {
		t.Fatal(rows, e)
	}
	cfg := f.Service.Config
	f.App.Close()
	f.Service.Close()
	reopened, e := service.Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	recovered, e := reopened.LookupNotebookCreation(t.Context(), p, h.space.ID, contract.NotebookCreateLookup{IdempotencyKey: "note-child"})
	if e != nil || recovered.Status != "completed" || recovered.OperationID != child.OperationID {
		t.Fatal(recovered, e)
	}
}

func TestNotebookPublicationMetadataFailureResumesOriginalOperation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := contract.NotebookCreate{SpaceID: h.space.ID, Title: "Recovery", Icon: "file"}
	f.Service.DB.Callback().Create().Before("gorm:create").Register("notebook-fail", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "NotebookPageMetadata" {
			tx.AddError(gorm.ErrInvalidTransaction)
		}
	})
	if _, e := f.Service.CreateNotebookPage(t.Context(), p, h.space.ID, q, "recover-note"); e == nil {
		t.Fatal("fault not injected")
	}
	f.Service.DB.Callback().Create().Remove("notebook-fail")
	lookup := contract.NotebookCreateLookup{IdempotencyKey: "recover-note"}
	pending, e := f.Service.LookupNotebookCreation(t.Context(), p, h.space.ID, lookup)
	if e != nil || !pending.Found || pending.Status == "completed" || pending.OperationID == "" {
		t.Fatal(pending, e)
	}
	resumed, e := f.Service.ResumeNotebookCreation(t.Context(), p, h.space.ID, lookup)
	if e != nil || resumed.Status != "completed" || resumed.OperationID != pending.OperationID {
		t.Fatal(resumed, e)
	}
	var count int64
	f.Service.DB.Model(&service.File{}).Where("owner = ?", p.UserID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate file", count)
	}
}
