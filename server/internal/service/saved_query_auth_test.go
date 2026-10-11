package service_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestSavedQueryCurrentScopeAndAdmission(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	q := service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}}
	saved, err := f.Service.SavedFileQueryCreate(p, "Private conditions", q)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}} {
		list, err := f.Service.SavedFileQueriesList(other)
		if err != nil || len(list) != 0 {
			t.Fatal("query name or predicates leaked across scope", err)
		}
		if _, err = f.Service.SavedFileQueryCreate(other, "Forbidden", q); err == nil {
			t.Fatal("saved query accepted foreign scope")
		}
	}
	var otherSpace service.Space
	if status := h.call(t, "POST", "/spaces", map[string]any{"name": "Other saved scope", "connection_id": "platform"}, &otherSpace); status != 200 {
		t.Fatal("second space fixture failed", status)
	}
	multi := q
	multi.SpaceIDs = []string{h.space.ID, otherSpace.ID}
	if _, err = f.Service.SavedFileQueryCreate(p, "Multi-space", multi); err != nil {
		t.Fatal(err)
	}
	narrow, err := f.Service.SavedFileQueriesList(service.Principal{UserID: p.UserID, ScopeSpace: h.space.ID})
	if err != nil || len(narrow) != 1 || narrow[0].QueryID != saved.QueryID {
		t.Fatal("multi-space query was partially exposed", err)
	}
	if _, err = f.Service.SavedFileQueryCreate(p, "Mixed owner", service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID, bob.space.ID}}); err == nil {
		t.Fatal("partially permitted query was saved")
	}
	// Full-space legacy readers retain the original SDK surface. Prefix-only
	// readers cannot inspect arbitrary saved predicates or private names.
	_, root := agent(t, h, "", "read")
	_, prefix := agent(t, h, "allowed", "read")
	list, err := root.SavedSearchesList(t.Context())
	if err != nil || len(list) != 1 || list[0].QueryID != saved.QueryID {
		t.Fatal("legacy root reader regression", err)
	}
	hidden, err := prefix.SavedSearchesList(t.Context())
	if err != nil || len(hidden) != 0 {
		t.Fatal("prefix Agent saw unrelated query metadata", err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SavedFileQueriesList(p); err == nil || err.Error() != "forbidden" {
		t.Fatal("disabled direct caller read conditions", err)
	}
	if _, err = f.Service.SavedFileQueryCreate(p, "Disabled", q); err == nil {
		t.Fatal("disabled caller created conditions")
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	// GET is observational: it must never update timestamps or canonical rows.
	const noWrite = "saved_query_no_write"
	if err = f.Service.DB.Callback().Update().Before("gorm:update").Register(noWrite, func(tx *gorm.DB) { tx.AddError(errors.New("read attempted write")) }); err != nil {
		t.Fatal(err)
	}
	got, err := f.Service.SavedFileQueriesList(p)
	f.Service.DB.Callback().Update().Remove(noWrite)
	if err != nil || len(got) != 2 || !got[0].UpdatedAt.Equal(saved.UpdatedAt) {
		t.Fatal("read changed original observation", err)
	}
}

func TestSavedQueryDependencyFailureAndInvalidRecord(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}}
	saved, err := f.Service.SavedFileQueryCreate(p, "Fixture", q)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"users", "spaces", "saved_file_queries"} {
		hits := 0
		if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("saved_query_fault", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic query failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, failure := f.Service.SavedFileQueriesList(p)
		f.Service.DB.Callback().Query().Remove("saved_query_fault")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("DB error became empty", table, failure)
		}
	}
	for _, raw := range []string{`null`, `{"mode":"exact","space_ids":[]}`, `{"mode":"exact","space_ids":["x"],"secret":"private"}`, `{"mode":"exact"} {}`} {
		if err = f.Service.DB.Model(&service.SavedFileQuery{}).Where("id = ?", saved.QueryID).Update("query", raw).Error; err != nil {
			t.Fatal(err)
		}
		if _, err = f.Service.SavedFileQueriesList(p); err == nil || err.Error() != "invalid_query" {
			t.Fatal("malformed stored query accepted", err)
		}
	}
}

func TestSavedQueryStoredPayloadBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []service.FileQueryFilter{{Field: "caller", Op: "eq", Value: strings.Repeat("x", 65536)}}}
	if _, err := f.Service.SavedFileQueryCreate(p, "Too big", q); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("create byte budget missing", err)
	}
	row := service.SavedFileQuery{ID: service.ID(), Owner: h.user.ID, Name: "Legacy large", Query: strings.Repeat(" ", 65537)}
	if err := f.Service.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.SavedFileQueriesList(p); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("stored byte budget missing", err)
	}
}

func TestSavedQueryBoundedCandidates(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	rows := make([]service.SavedFileQuery, 1001)
	for i := range rows {
		rows[i] = service.SavedFileQuery{ID: service.ID(), Owner: h.user.ID, Name: fmt.Sprint(i), Query: `{"mode":"exact","space_ids":["missing"]}`, CreatedAt: time.Now().UTC()}
	}
	if err := f.Service.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.SavedFileQueriesList(service.Principal{UserID: h.user.ID}); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("unbounded saved-query catalogue", err)
	}
}
