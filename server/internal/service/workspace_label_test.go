package service_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLabelDictionaryCASPaginationRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	f.Service.Config.LabelManagement = true
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	q := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{}}
	for i := range 60 {
		q.Definitions = append(q.Definitions, contract.LabelDefinitionInput{Key: fmt.Sprintf("label_%03d", i), DisplayName: fmt.Sprintf("Label %d", i), Group: "Manual"})
	}
	receipt, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, q, "dictionary-first")
	if err != nil || receipt.ResultRevision != 1 {
		t.Fatal(receipt, err)
	}
	page, err := f.Service.LabelDefinitions(t.Context(), p, h.space.ID, "manual", "", 0)
	if err != nil || len(page.Definitions) != 50 || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	last, err := f.Service.LabelDefinitions(t.Context(), p, h.space.ID, "manual", page.NextCursor, 0)
	if err != nil || len(last.Definitions) != 10 || last.Definitions[0].Key != "label_050" {
		t.Fatal(last, err)
	}
	q.ExpectedRevision = 1
	q.Definitions[0].DisplayName = "Renamed display"
	if _, err = f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, q, "dictionary-second"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.LabelDefinitions(t.Context(), p, h.space.ID, "manual", page.NextCursor, 0); err == nil || err.Error() != "query_changed" {
		t.Fatal("stale cursor", err)
	}
	original, err := f.Service.LookupLabelDefinitionUpdate(t.Context(), p, h.space.ID, "dictionary-first")
	if err != nil || original != receipt {
		t.Fatal("original receipt", original, err)
	}
	if _, err = f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, q, "dictionary-stale"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale CAS", err)
	}
	if _, err = f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, q, "dictionary-first"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed intent", err)
	}
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.LabelDefinitions(t.Context(), bad, h.space.ID, "manual", "", 50); err == nil {
			t.Fatal("scope leaked")
		}
		if _, err = f.Service.LookupLabelDefinitionUpdate(t.Context(), bad, h.space.ID, "dictionary-first"); err == nil {
			t.Fatal("receipt leaked")
		}
	}
}

func TestWorkspaceLabelDictionaryAtomicRollback(t *testing.T) {
	f := rig(t)
	f.Service.Config.LabelManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	q := contract.LabelDefinitionUpdate{Definitions: []contract.LabelDefinitionInput{{Key: "source", DisplayName: "Source"}}}
	for _, table := range []string{"workspace_label_catalogs", "workspace_label_definitions", "workspace_requests"} {
		hits := 0
		f.Service.DB.Callback().Create().Before("gorm:create").Register("label-failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("injected label write failure"))
			}
		})
		_, err := f.Service.UpdateLabelDefinitions(t.Context(), p, h.space.ID, q, "rollback")
		f.Service.DB.Callback().Create().Remove("label-failure")
		if hits != 1 || err == nil {
			t.Fatal(table, hits, err)
		}
		view, err := f.Service.LabelDefinitions(t.Context(), p, h.space.ID, "manual", "", 200)
		if err != nil || view.Revision != 0 || len(view.Definitions) != 0 {
			t.Fatal("partial commit", view, err)
		}
	}
}
