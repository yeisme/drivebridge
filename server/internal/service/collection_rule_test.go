package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"fmt"
	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestCollectionRuleOriginalReceiptAndExecution(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	match := orgUpload(t, client, h.space.ID, "rules/needle.txt", "match")
	orgUpload(t, client, h.space.ID, "rules/other.txt", "other")
	rule := `{ "mode" : "exact", "filters" : [{"field":"path_prefix","op":"contains","value":"needle"}], "sort":[{"field":"name","direction":"asc"}] }`
	q := service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Rule", Kind: "rule", Scope: "private", Rule: json.RawMessage(rule)}
	hits := 0
	f.Service.DB.Callback().Create().Before("gorm:create").Register("rule_receipt_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			hits++
			tx.AddError(errors.New("synthetic failure"))
		}
	})
	_, err := f.Service.CreateFileCollection(p, q, "rule-original")
	f.Service.DB.Callback().Create().Remove("rule_receipt_failure")
	if err == nil || err.Error() != "database_unavailable" || hits != 1 {
		t.Fatal("receipt failure ignored", err, hits)
	}
	var count int64
	f.Service.DB.Model(&service.FileCollection{}).Count(&count)
	if count != 0 {
		t.Fatal("partial rule creation")
	}
	// Use the original nested bytes through real human HTTP, not an Agent SDK.
	raw := []byte(fmt.Sprintf(`{"space_id":%q,"name":"Rule","kind":"rule","scope":"private","rule":%s}`, h.space.ID, rule))
	req, _ := http.NewRequest("POST", h.base+"/api/v1/file-collections", bytes.NewReader(raw))
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "rule-original")
	response, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var env contract.DataEnvelope[service.FileCollectionView]
	if json.NewDecoder(response.Body).Decode(&env) != nil || response.StatusCode != 200 {
		t.Fatal("rule HTTP create failed", response.StatusCode)
	}
	created := env.Data
	if created.Rule != rule {
		t.Fatal("raw identity changed")
	}
	var receipt contract.CollectionCreationReceipt
	if status := h.call(t, "POST", "/file-collections/lookup", map[string]string{"idempotency_key": "rule-original"}, &receipt); status != 200 {
		t.Fatal("human rule receipt missing", status)
	}
	view, err := f.Service.ReadFileCollection(p, created.CollectionID)
	if err != nil || len(view.Items) != 1 || view.Items[0].FileID != match.FileID {
		t.Fatal("rule not executed", err)
	}
	// Explicit decisions can advance current revision without replacing revision-one receipt.
	if _, err = f.Service.SetCollectionMembers(p, created.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("lookup attempted write")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("rule_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("rule_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("rule_no_write", guard)
	recovered, err := f.Service.LookupCollectionCreation(t.Context(), p, "rule-original")
	f.Service.DB.Callback().Create().Remove("rule_no_write")
	f.Service.DB.Callback().Update().Remove("rule_no_write")
	f.Service.DB.Callback().Delete().Remove("rule_no_write")
	if err != nil || writes != 0 || recovered != receipt || receipt.ResultRevision != 1 {
		t.Fatal("lookup not original", err, writes)
	}
	// A pre-receipt row replays but must not get a manufactured historical fact.
	if err = f.Service.DB.Where("resource_id = ?", created.CollectionID).Delete(&service.WorkspaceRequest{}).Error; err != nil {
		t.Fatal(err)
	}
	replay, err := f.Service.CreateFileCollection(p, q, "rule-original")
	if err != nil || replay.CollectionID != created.CollectionID {
		t.Fatal("legacy replay broken", err)
	}
	if _, err = f.Service.LookupCollectionCreation(t.Context(), p, "rule-original"); err == nil || err.Error() != "not_found" {
		t.Fatal("legacy receipt backfilled", err)
	}
	// A corrupt persisted rule must fail before it can broaden the enumeration.
	if err = f.Service.DB.Model(&service.FileCollection{}).Where("id = ?", created.CollectionID).Update("rule", `{"mode":"exact","filters":[{"field":"mime_type","op":"ne","value":"text/plain"}]}`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReadFileCollection(p, created.CollectionID); err == nil || err.Error() != "invalid_query" {
		t.Fatal("stored condition silently ignored", err)
	}
}

func TestCollectionRuleClosedExecutableInput(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	bad := []string{
		`{}`, `null`, `{"mode":"duplicates"}`, `{"mode":"exact","mode":"exact"}`, `{"mode":"exact","unknown":true}`,
		`{"mode":"exact","space_ids":["foreign"]}`, `{"mode":"exact","space_ids":[]}`, `{"mode":"exact","filters":null}`, `{"mode":"exact","cursor":"old"}`, `{"mode":"exact","limit":201}`,
		`{"mode":"exact","filters":[{"field":"mime_type","op":"contains","value":"text"}]}`,
		`{"mode":"exact","filters":[{"field":"size_bytes","op":"eq","value":1}]}`,
		`{"mode":"exact","filters":[{"field":"size_bytes","op":"gt","value":"1"}]}`,
		`{"mode":"exact","filters":[{"field":"size_bytes","op":"gt","value":1.00000000000000001}]}`,
		`{"mode":"exact","filters":[{"field":"updated_at","op":"gt","value":"yesterday"}]}`,
		`{"mode":"exact","filters":[{"field":"trashed","op":"eq","value":true}]}`,
		`{"mode":"exact","filters":[{"field":"labels.manual","op":"prefix","value":"x"}]}`,
		`{"mode":"exact","filters":[{"field":"caller","op":"eq","value":"a","value":"b"}]}`,
		`{"mode":"exact","filters":[{"field":"caller","op":"eq","value":"x","extra":1}]}`,
		`{"mode":"exact","filters":[{"field":"caller","op":"eq","value":"\ud800"}]}`,
		`{"mode":"exact","sort":[{"field":"mime_type","direction":"asc"}]}`,
		`{"mode":"exact","sort":[{"field":"size_bytes","direction":"asc"},{"field":"name","direction":"asc"}]}`,
		`{"mode":"exact","sort":[{"field":"name","direction":"asc","direction":"desc"}]}`,
		`{"mode":"exact","filters":[` + strings.TrimSuffix(strings.Repeat(`{"field":"caller","op":"eq","value":"x"},`, 65), ",") + `]}`,
		strings.Repeat(" ", 64*1024) + `{"mode":"exact"}`,
	}
	for i, raw := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Invalid rule", Kind: "rule", Scope: "private", Rule: json.RawMessage(raw)}, fmt.Sprint("invalid-rule-", i))
			if err == nil || err.Error() != "invalid_query" {
				t.Fatalf("unexecutable rule admitted: %v", err)
			}
		})
	}
	var count int64
	if err := f.Service.DB.Model(&service.FileCollection{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("rejected rule persisted", count, err)
	}
	// Same nested-duplicate rejection through HTTP before any create.
	for _, rule := range []string{bad[18], bad[19], bad[20], bad[23]} {
		raw := []byte(fmt.Sprintf(`{"space_id":%q,"name":"Bad HTTP","kind":"rule","scope":"private","rule":%s}`, h.space.ID, rule))
		req, _ := http.NewRequest("POST", h.base+"/api/v1/file-collections", bytes.NewReader(raw))
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "bad-rule-http")
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 400 {
			t.Fatal("HTTP invalid nested rule accepted", res.StatusCode)
		}
	}
	valid := []string{
		`{"mode":"exact"}`,
		fmt.Sprintf(`{"mode":"exact","space_ids":[%q],"filters":[],"sort":[],"cursor":"","limit":200}`, h.space.ID),
		`{"mode":"exact","filters":[{"field":"size_bytes","op":"gte","value":1e2},{"field":"updated_at","op":"lt","value":"2030-01-01T00:00:00Z"},{"field":"trashed","op":"eq","value":false}]}`,
		`{"mode":"exact","filters":[{"field":"mime_type","op":"eq","value":"text/plain"},{"field":"caller","op":"eq","value":""},{"field":"task_ref","op":"eq","value":""}]}`,
		`{"mode":"exact","filters":[{"field":"labels.manual","op":"contains","value":"tag"},{"field":"labels.rule","op":"ne","value":"skip"},{"field":"labels.suggested","op":"eq","value":"review"}],"sort":[{"field":"updated_at","direction":"desc"}]}`,
	}
	for i, rule := range valid {
		collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Valid", Kind: "rule", Scope: "private", Rule: json.RawMessage(rule)}, fmt.Sprint("valid-rule-", i))
		if err != nil {
			t.Fatal("valid legacy rule rejected", i, err)
		}
		if _, err = f.Service.ReadFileCollection(p, collection.CollectionID); err != nil {
			t.Fatal("valid rule cannot execute", i, err)
		}
	}
}
