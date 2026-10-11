package service_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionMemberDecisionsCompleteOrUnavailable(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	a := orgUpload(t, client, h.space.ID, "decisions/included.txt", "a")
	b := orgUpload(t, client, h.space.ID, "decisions/excluded.txt", "b")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Decisions", Kind: "manual", Scope: "private"}, "decision-base")
	if err != nil {
		t.Fatal(err)
	}
	path := "/file-collections/" + c.CollectionID + "/members/decisions"
	var empty contract.CollectionMemberDecisions
	if status := h.call(t, "GET", path, nil, &empty); status != 200 || empty.Revision != 1 || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal("empty base failed", status)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{a.FileID}, Exclude: []string{b.FileID}}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("decision read attempted write")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("decisions_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("decisions_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("decisions_no_write", guard)
	var out contract.CollectionMemberDecisions
	status := h.call(t, "GET", path, nil, &out)
	f.Service.DB.Callback().Create().Remove("decisions_no_write")
	f.Service.DB.Callback().Update().Remove("decisions_no_write")
	f.Service.DB.Callback().Delete().Remove("decisions_no_write")
	if status != 200 || writes != 0 || out.Revision != 2 || len(out.Items) != 2 {
		t.Fatal("incomplete base or write", status, writes)
	}
	decisions := map[string]string{}
	for _, item := range out.Items {
		decisions[item.FileID] = item.Decision
		if item.SpaceID != h.space.ID || item.Path == "" {
			t.Fatal("missing authorized metadata")
		}
	}
	if decisions[a.FileID] != "include" || decisions[b.FileID] != "exclude" {
		t.Fatal("lost excluded row")
	}
	if status = bob.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatal("foreign base visible", status)
	}
	if err = client.JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("agent editing base allowed")
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.CollectionMemberDecisions(t.Context(), bad, c.CollectionID); err == nil {
			t.Fatal("scope bypass")
		}
	}
	if status = h.call(t, "GET", path+"?limit=1", nil, nil); status != 400 {
		t.Fatal("partial paging allowed")
	}
	for _, table := range []string{"users", "spaces", "file_collections", "collection_members", "files"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("decisions_db_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic dependency failure"))
			}
		})
		status = h.call(t, "GET", path, nil, nil)
		f.Service.DB.Callback().Query().Remove("decisions_db_fail")
		if hits == 0 || status != 500 {
			t.Fatal("failure hidden as empty", table, status)
		}
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", b.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	partial, err := f.Service.CollectionMemberDecisions(t.Context(), p, c.CollectionID)
	if err == nil || err.Error() != "not_found" || partial.Items != nil {
		t.Fatal("hidden exclude silently filtered", err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", b.FileID).Update("trashed", false).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.CollectionMember{}).Where("collection_id = ? AND file_id = ?", c.CollectionID, b.FileID).Update("decision", "invalid").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CollectionMemberDecisions(t.Context(), p, c.CollectionID); err == nil || err.Error() != "dependency_unavailable" {
		t.Fatal("invalid persisted decision accepted", err)
	}
}

func TestCollectionMemberDecisionsBudgetsAndRevision(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Budget", Kind: "manual", Scope: "private"}, "decision-budget")
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]service.CollectionMember, 10001)
	for i := range rows {
		rows[i] = service.CollectionMember{ID: service.ID(), CollectionID: c.CollectionID, FileID: fmt.Sprintf("%048x", i+1), Owner: p.UserID, Decision: "include"}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CollectionMemberDecisions(t.Context(), p, c.CollectionID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("candidate limit lost", err)
	}
	if err = f.Service.DB.Where("collection_id = ?", c.CollectionID).Delete(&service.CollectionMember{}).Error; err != nil {
		t.Fatal(err)
	}
	a := orgUpload(t, mustSDK(t, h), h.space.ID, "budget/a.txt", "a")
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1, Include: []string{a.FileID}}); err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", a.FileID).Update("path", strings.Repeat("x", 1024*1024)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CollectionMemberDecisions(t.Context(), p, c.CollectionID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("wire limit lost", err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", a.FileID).Update("path", "budget/a.txt").Error; err != nil {
		t.Fatal(err)
	}
	fired := false
	f.Service.DB.Callback().Query().After("gorm:query").Register("decisions_revision_drift", func(tx *gorm.DB) {
		if tx.Statement.Table == "collection_members" && !fired {
			fired = true
			if err := tx.Session(&gorm.Session{NewDB: true}).Model(&service.FileCollection{}).Where("id = ?", c.CollectionID).Update("revision", 3).Error; err != nil {
				tx.AddError(err)
			}
		}
	})
	_, failure := f.Service.CollectionMemberDecisions(t.Context(), p, c.CollectionID)
	f.Service.DB.Callback().Query().Remove("decisions_revision_drift")
	if !fired || failure == nil || failure.Error() != "query_changed" {
		t.Fatal("mixed revision accepted", failure)
	}
}
