package service_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestPropertySidecarFixedVersionAndOriginalReceipt(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	file := orgUpload(t, client, h.space.ID, "sidecar/image.png", "synthetic binary fixture")
	_, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "caption", DisplayName: "Caption", Type: "text"}, {Key: "source", DisplayName: "Source", Type: "reference"}, {Key: "rank", DisplayName: "Rank", Type: "number"}}}, "sidecar-definitions")
	if err != nil {
		t.Fatal(err)
	}
	base, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || base.Revision != 0 || base.DefinitionRevision != 1 || len(base.Values) != 0 {
		t.Fatal("empty sidecar not virtual", err)
	}
	q := contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"caption": json.RawMessage(`"Original"`), "rank": json.RawMessage(`1.250`)}, RemoveKeys: []string{}}
	first, err := f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "sidecar-first")
	if err != nil || first.ResultRevision != 1 {
		t.Fatal("patch failed", err)
	}
	current, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || string(current.Values["caption"]) != `"Original"` || string(current.Values["rank"]) != "1.250" {
		t.Fatal("value coerced", err)
	}
	// Unknown legacy keys remain opaque and byte-preserved, never surfaced as refs.
	var row service.WorkspacePropertySidecar
	if err = f.Service.DB.First(&row, "file_id = ?", file.FileID).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&row).Update("values_json", `{"caption":"Original","rank":1.250,"legacy":{ "nested" : [true, null] }}`).Error; err != nil {
		t.Fatal(err)
	}
	next := contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, ExpectedRevision: 1, Updates: map[string]json.RawMessage{"caption": json.RawMessage(`"Changed"`)}, RemoveKeys: []string{"rank"}}
	second, err := f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, next, "sidecar-second")
	if err != nil || second.ResultRevision != 2 {
		t.Fatal("remove/update failed", err)
	}
	if err = f.Service.DB.First(&row, "id = ?", row.ID).Error; err != nil || !strings.Contains(row.ValuesJSON, `{ "nested" : [true, null] }`) {
		t.Fatal("unknown value lost", err)
	}
	current, err = f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || len(current.UnknownKeys) != 1 || current.UnknownKeys[0] != "legacy" || current.Values["legacy"] != nil || current.Values["rank"] != nil {
		t.Fatal("unknown raw leaked or remove failed", err)
	}
	replay, err := f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "sidecar-first")
	if err != nil || replay != first {
		t.Fatal("old receipt replaced current", err)
	}
	if _, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "stale"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale value write", err)
	}
	// Simulate the existing upload publisher advancing head with a real Version row.
	var fixed service.Version
	if err = f.Service.DB.First(&fixed, "id = ?", file.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	fixed.ID = service.ID()
	if err = f.Service.DB.Create(&fixed).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.FileID).Update("version_id", fixed.ID).Error; err != nil {
		t.Fatal(err)
	}
	old, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	if err != nil || old.Revision != 2 || string(old.Values["caption"]) != `"Changed"` {
		t.Fatal("old version followed head", err)
	}
	fresh, err := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, fixed.ID)
	if err != nil || fresh.Revision != 0 || len(fresh.Values) != 0 {
		t.Fatal("new version copied old values", err)
	}
	next.ExpectedRevision = 2
	if _, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, next, "old-head"); err == nil || err.Error() != "version_conflict" {
		t.Fatal("old version mutated", err)
	}
	recovered, err := f.Service.LookupFilePropertySidecar(t.Context(), p, file.FileID, file.VersionID, "sidecar-first")
	if err != nil || recovered != first {
		t.Fatal("history lost after head change", err)
	}
	replay, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "sidecar-first")
	if err != nil || replay != first {
		t.Fatal("replay rewrote old version", err)
	}
}

func TestPropertySidecarAuthorityReferenceAndRollback(t *testing.T) {
	f := rig(t)
	f.Service.Config.PropertyEditing = true
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	file := orgUpload(t, client, h.space.ID, "sidecar/owned.pdf", "synthetic PDF fixture")
	refFile := orgUpload(t, client, h.space.ID, "sidecar/reference.txt", "synthetic reference")
	markdown := orgUpload(t, client, h.space.ID, "sidecar/note.md", "# Owner note")
	definition := contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "source", DisplayName: "Source", Type: "reference"}, {Key: "caption", DisplayName: "Caption", Type: "text"}}}
	if _, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, definition, "refs-definitions"); err != nil {
		t.Fatal(err)
	}
	ref, _ := json.Marshal(f.Service.FileRef(service.File{ID: refFile.FileID, VersionID: refFile.VersionID}))
	q := contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"source": ref}, RemoveKeys: []string{}}
	for _, table := range []string{"workspace_property_sidecars", "workspace_requests"} {
		hits := 0
		f.Service.DB.Callback().Create().Before("gorm:create").Register("sidecar_write_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic sidecar failure"))
			}
		})
		_, failure := f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "sidecar-atomic")
		f.Service.DB.Callback().Create().Remove("sidecar_write_failure")
		if hits != 1 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("write failure ignored", table, failure)
		}
		var count int64
		if err := f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("partial sidecar persisted", count, err)
		}
	}
	first, err := f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, q, "sidecar-atomic")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.FilePropertySidecar(t.Context(), p, markdown.FileID, markdown.VersionID); err == nil || err.Error() != "unsupported_action" {
		t.Fatal("Markdown sidecar bypass", err)
	}
	agentRow, _ := agent(t, h, "", "read,write")
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, Agent: &agentRow}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.FilePropertySidecar(t.Context(), bad, file.FileID, file.VersionID); err == nil {
			t.Fatal("unauthorized values")
		}
		if _, err = f.Service.LookupFilePropertySidecar(t.Context(), bad, file.FileID, file.VersionID, "sidecar-atomic"); err == nil {
			t.Fatal("unauthorized receipt")
		}
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("read attempted write")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("sidecar_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("sidecar_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("sidecar_no_write", guard)
	view, readErr := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
	receipt, lookupErr := f.Service.LookupFilePropertySidecar(t.Context(), p, file.FileID, file.VersionID, "sidecar-atomic")
	f.Service.DB.Callback().Create().Remove("sidecar_no_write")
	f.Service.DB.Callback().Update().Remove("sidecar_no_write")
	f.Service.DB.Callback().Delete().Remove("sidecar_no_write")
	if writes != 0 || readErr != nil || lookupErr != nil || view.Revision != 1 || receipt != first {
		t.Fatal("read wrote or lost fact", writes, readErr, lookupErr)
	}
	// References are re-authorized on read, not exposed on shape validation alone.
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", refFile.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID); err == nil {
		t.Fatal("hidden reference exposed")
	}
	foreign := q
	foreign.ExpectedRevision = 1
	foreign.Updates = map[string]json.RawMessage{"source": json.RawMessage(strings.Replace(string(ref), f.Service.Instance, "foreign-instance", 1))}
	if _, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, foreign, "foreign-ref"); err == nil || err.Error() != "cross_instance_rejected" {
		t.Fatal("foreign ref authorized by shape", err)
	}
	// Removing a now-hidden reference is safe and discloses no target metadata.
	removal := contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, ExpectedRevision: 1, Updates: map[string]json.RawMessage{}, RemoveKeys: []string{"source"}}
	if _, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, removal, "remove-hidden-ref"); err != nil {
		t.Fatal(err)
	}
	definition.ExpectedRevision = 1
	definition.Definitions[0].DisplayName = "Updated source"
	if _, err = f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, definition, "refs-definition-update"); err != nil {
		t.Fatal(err)
	}
	removal.ExpectedRevision = 2
	if _, err = f.Service.PatchFilePropertySidecar(t.Context(), p, file.FileID, removal, "old-dictionary"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("dictionary drift accepted", err)
	}
	for _, table := range []string{"users", "spaces", "files", "versions", "workspace_property_catalogs", "workspace_property_sidecars"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("sidecar_query_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic dependency failure"))
			}
		})
		_, failure := f.Service.FilePropertySidecar(t.Context(), p, file.FileID, file.VersionID)
		f.Service.DB.Callback().Query().Remove("sidecar_query_failure")
		if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
			t.Fatal("dependency failure hidden", table, failure)
		}
	}
}
