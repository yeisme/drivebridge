package service_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestPropertyDefinitionsAuthorityCASAndOriginalReceipt(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	initial, err := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	if err != nil || initial.Revision != 0 || initial.Definitions == nil || len(initial.Definitions) != 0 {
		t.Fatal("empty catalog not virtual revision zero", err)
	}
	q := contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "status", DisplayName: "状态", Group: "项目", Type: "text"}, {Key: "due_date", DisplayName: "截止日期", Group: "项目", Type: "date"}}}
	first, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "property-first")
	if err != nil || first.ResultRevision != 1 {
		t.Fatal("create catalog failed", err)
	}
	latest, err := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	if err != nil || latest.Revision != 1 || len(latest.Definitions) != 2 || latest.Definitions[0].Key != "due_date" {
		t.Fatal("catalog projection failed", err)
	}
	// Definition changes must not rewrite file/version metadata projections.
	projection := service.WorkspaceFilePropertyProjection{ID: service.ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: h.space.ID, FileID: "file", VersionID: "fixed", DefinitionRevision: 1, Key: "status", StorageKind: "sidecar", ValueJSON: `{"unknown":{"nested":true}}`, Revision: 1}
	if err = f.Service.DB.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	next := contract.PropertyDefinitionUpdate{ExpectedRevision: 1, Definitions: append([]contract.PropertyDefinition{}, q.Definitions...)}
	next.Definitions[0].DisplayName = "进度"
	next.Definitions[0].Group = ""
	second, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, next, "property-second")
	if err != nil || second.ResultRevision != 2 {
		t.Fatal("presentation edit failed", err)
	}
	replay, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "property-first")
	if err != nil || replay != first {
		t.Fatal("replay changed original result", err)
	}
	recovered, err := f.Service.LookupPropertyDefinitionUpdate(t.Context(), p, h.space.ID, "property-first")
	if err != nil || recovered != first {
		t.Fatal("lookup changed original result", err)
	}
	current, err := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	if err != nil || current.Revision != 2 || current.Definitions[1].DisplayName != "进度" {
		t.Fatal("replay rewrote current catalog", err)
	}
	var unchanged service.WorkspaceFilePropertyProjection
	if err = f.Service.DB.First(&unchanged, "id = ?", projection.ID).Error; err != nil || unchanged.ValueJSON != projection.ValueJSON || unchanged.VersionID != "fixed" || unchanged.DefinitionRevision != 1 {
		t.Fatal("dictionary mutated file projection", err)
	}
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "stale"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale catalog overwritten", err)
	}
	changed := next
	changed.ExpectedRevision = 2
	changed.Definitions = append([]contract.PropertyDefinition{}, next.Definitions...)
	changed.Definitions[0].Type = "number"
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, changed, "type-migration"); err == nil || err.Error() != "unsupported_action" {
		t.Fatal("implicit type migration", err)
	}
	changed.Definitions = []contract.PropertyDefinition{}
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, changed, "delete-migration"); err == nil || err.Error() != "unsupported_action" {
		t.Fatal("implicit key deletion", err)
	}
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, next, "property-first"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("key reused for new request", err)
	}
	agentRow, _ := agent(t, h, "", "read,write")
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: p.UserID, Agent: &agentRow}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.PropertyDefinitions(t.Context(), bad, h.space.ID); err == nil {
			t.Fatal("unauthorized dictionary read")
		}
		if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), bad, h.space.ID, q, "bad-actor"); err == nil {
			t.Fatal("unauthorized dictionary write")
		}
		if _, err = f.Service.LookupPropertyDefinitionUpdate(t.Context(), bad, h.space.ID, "property-first"); err == nil {
			t.Fatal("unauthorized receipt")
		}
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PropertyDefinitions(t.Context(), p, h.space.ID); err == nil {
		t.Fatal("disabled user read")
	}
}

func TestPropertyDefinitionsAtomicAndReadOnly(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "status", DisplayName: "Status", Type: "text"}}}
	for _, table := range []string{"workspace_property_catalogs", "workspace_property_definitions", "workspace_requests"} {
		hits := 0
		f.Service.DB.Callback().Create().Before("gorm:create").Register("property_create_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic property failure"))
			}
		})
		_, failure := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "atomic-property")
		f.Service.DB.Callback().Create().Remove("property_create_fail")
		if hits != 1 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("dependency failure ignored", table, failure)
		}
		for _, model := range []any{&service.WorkspacePropertyCatalog{}, &service.WorkspacePropertyDefinition{}} {
			var count int64
			if err := f.Service.DB.Model(model).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("partial dictionary committed", table, count, err)
			}
		}
	}
	first, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "atomic-property")
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("property read wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("property_read_guard", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("property_read_guard", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("property_read_guard", guard)
	view, readErr := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	receipt, lookupErr := f.Service.LookupPropertyDefinitionUpdate(t.Context(), p, h.space.ID, "atomic-property")
	f.Service.DB.Callback().Create().Remove("property_read_guard")
	f.Service.DB.Callback().Update().Remove("property_read_guard")
	f.Service.DB.Callback().Delete().Remove("property_read_guard")
	if writes != 0 || readErr != nil || lookupErr != nil || view.Revision != 1 || receipt != first {
		t.Fatal("read or lookup wrote", writes, readErr, lookupErr)
	}
	for _, table := range []string{"users", "spaces", "workspace_property_catalogs", "workspace_property_definitions"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("property_query_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic query failure"))
			}
		})
		_, failure := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
		f.Service.DB.Callback().Query().Remove("property_query_fail")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("read failure hidden as empty", table, failure)
		}
	}
	// A pre-catalog historical row must not be silently assigned a guessed revision.
	if err = f.Service.DB.Where("space_id = ?", h.space.ID).Delete(&service.WorkspacePropertyCatalog{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PropertyDefinitions(t.Context(), p, h.space.ID); err == nil || err.Error() != "dependency_unavailable" {
		t.Fatal("legacy dictionary guessed revision", err)
	}
}

func TestPropertyDefinitionsBudgetAndReadGeneration(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	fired := false
	if err := f.Service.DB.Callback().Query().After("gorm:query").Register("property_empty_generation", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_property_definitions" && !fired {
			fired = true
			if err := tx.Session(&gorm.Session{NewDB: true}).Create(&service.WorkspacePropertyCatalog{SpaceID: h.space.ID, Owner: p.UserID, TenantID: p.TenantID, Revision: 1}).Error; err != nil {
				tx.AddError(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	f.Service.DB.Callback().Query().Remove("property_empty_generation")
	if !fired || failure == nil || failure.Error() != "query_changed" {
		t.Fatal("empty observation hid catalog generation", failure)
	}
	q := contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{}}
	for i := 0; i < 200; i++ {
		q.Definitions = append(q.Definitions, contract.PropertyDefinition{Key: fmt.Sprintf("field_%03d", i), DisplayName: "Field", Type: "text"})
	}
	if _, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "bounded"); err != nil {
		t.Fatal(err)
	}
	current, err := f.Service.PropertyDefinitions(t.Context(), p, h.space.ID)
	if err != nil || len(current.Definitions) != 200 {
		t.Fatal("200 definitions failed", err)
	}
	q.ExpectedRevision = 1
	q.Definitions = append(q.Definitions, contract.PropertyDefinition{Key: "extra", DisplayName: "Extra", Type: "text"})
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, "too-many"); err == nil || err.Error() != "invalid_request" {
		t.Fatal("write budget lost", err)
	}
	if err = f.Service.DB.Create(&service.WorkspacePropertyDefinition{ID: service.ID(), SpaceID: h.space.ID, Owner: p.UserID, TenantID: p.TenantID, Key: "extra", DisplayName: "Extra", Type: "text", Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PropertyDefinitions(t.Context(), p, h.space.ID); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("read budget hidden", err)
	}
}

func TestPropertyDefinitionsConcurrentCASAndValidation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "status", DisplayName: "Status", Type: "text"}}}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"concurrent-a", "concurrent-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, q, key)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if err.Error() == "revision_conflict" {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("same revision had multiple winners")
	}
	for _, invalid := range []contract.PropertyDefinitionUpdate{{ExpectedRevision: 1}, {ExpectedRevision: 1, Definitions: []contract.PropertyDefinition{{Key: "Status", DisplayName: "Status", Type: "text"}}}, {ExpectedRevision: 1, Definitions: []contract.PropertyDefinition{{Key: "status", DisplayName: "Status", Type: "script"}}}, {ExpectedRevision: 1, Definitions: []contract.PropertyDefinition{{Key: "status", DisplayName: " ", Type: "text"}}}, {ExpectedRevision: 1, Definitions: []contract.PropertyDefinition{q.Definitions[0], q.Definitions[0]}}} {
		if _, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, invalid, "invalid"); err == nil || err.Error() != "invalid_request" {
			t.Fatal("invalid definition accepted", err)
		}
	}
}
