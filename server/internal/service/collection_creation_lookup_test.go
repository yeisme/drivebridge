package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionCreationLookupOriginalFact(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	input := service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Original", Kind: "manual", Scope: "private"}
	create := func(raw []byte, key string) (int, service.FileCollectionView) {
		t.Helper()
		req, _ := http.NewRequest("POST", h.base+"/api/v1/file-collections", bytes.NewReader(raw))
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[service.FileCollectionView]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	raw, _ := json.Marshal(input)
	status, created := create(raw, "lookup-create")
	if status != 200 {
		t.Fatal("human create failed", status)
	}
	for _, bad := range [][]byte{[]byte(`{}`), append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"name":"duplicate"}`)...), append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"owner":"spoof"}`)...)} {
		if status, _ := create(bad, "bad-create"); status != 400 {
			t.Fatal("create not closed", status)
		}
	}
	rule := json.RawMessage(`{ "mode" : "exact", "filters" : [] }`)
	legacyRule, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Legacy rule", Kind: "rule", Scope: "private", Rule: rule}, "legacy-rule")
	if err != nil {
		t.Fatal(err)
	}
	ruleRaw := []byte(fmt.Sprintf(`{"space_id":%q,"name":"Legacy rule","kind":"rule","scope":"private","rule":%s}`, h.space.ID, rule))
	if status, replay := create(ruleRaw, "legacy-rule"); status != 200 || replay.CollectionID != legacyRule.CollectionID {
		t.Fatal("closed decode changed legacy raw rule identity", status)
	}
	var row service.FileCollection
	if err := f.Service.DB.First(&row, "id = ?", created.CollectionID).Error; err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"idempotency_key": row.KeyID}
	var original contract.CollectionCreationReceipt
	if status := h.call(t, "POST", "/file-collections/lookup", body, &original); status != 200 {
		t.Fatal("creation receipt missing", status)
	}
	if original.CollectionID != row.ID || original.SpaceID != row.SpaceID || original.RequestDigest != row.Fingerprint || original.ResultRevision != 1 || !original.CommittedAt.Equal(row.CreatedAt) {
		t.Fatal("not original creation fact")
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err := f.Service.LookupCollectionCreation(t.Context(), bad, row.KeyID); err == nil {
			t.Fatal("lookup bypassed active scope")
		}
	}
	// The shared digest extraction must preserve the released byte sequence.
	legacy, _ := json.Marshal(struct {
		Space string `json:"space"`
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Scope string `json:"scope"`
		Rule  string `json:"rule"`
	}{input.SpaceID, input.Name, input.Kind, input.Scope, ""})
	if service.Hash(string(legacy)) != original.RequestDigest {
		t.Fatal("legacy creation digest changed")
	}
	if _, err := f.Service.SetCollectionMembers(p, row.ID, service.SetCollectionMembersRequest{ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("lookup wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("creation_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("creation_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("creation_no_write", guard)
	var replay contract.CollectionCreationReceipt
	status = h.call(t, "POST", "/file-collections/lookup", body, &replay)
	f.Service.DB.Callback().Create().Remove("creation_no_write")
	f.Service.DB.Callback().Update().Remove("creation_no_write")
	f.Service.DB.Callback().Delete().Remove("creation_no_write")
	if status != 200 || writes != 0 || replay != original {
		t.Fatal("lookup changed original revision or wrote", status, writes)
	}
	if status = bob.call(t, "POST", "/file-collections/lookup", body, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err := agentClient.JSON(t.Context(), "POST", "/api/v1/file-collections/lookup", "", body, nil); err == nil {
		t.Fatal("agent acquired human receipt")
	}
	for _, raw := range []string{`{}`, `{"idempotency_key":null}`, `{"idempotency_key":"x","idempotency_key":"y"}`, `{"idempotency_key":"x","owner":"spoof"}`, `{"idempotency_key":"bad key"}`} {
		if status, _ := fpRawCall(t, h, "POST", "/file-collections/lookup", []byte(raw)); status != 400 {
			t.Fatal("lookup not closed", status)
		}
	}
	if status = h.call(t, "POST", "/file-collections/lookup?extra=1", body, nil); status != 400 {
		t.Fatal("lookup query accepted", status)
	}
	for _, table := range []string{"workspace_requests", "file_collections", "users", "spaces"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("creation_lookup_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic lookup failure"))
			}
		})
		status = h.call(t, "POST", "/file-collections/lookup", body, nil)
		f.Service.DB.Callback().Query().Remove("creation_lookup_fail")
		if hits == 0 || status != 500 {
			t.Fatal("failure hidden as missing", table, status)
		}
	}
	if err := f.Service.DB.Model(&service.FileCollection{}).Where("id = ?", row.ID).Update("name", "Drift").Error; err != nil {
		t.Fatal(err)
	}
	if status = h.call(t, "POST", "/file-collections/lookup", body, nil); status != 409 {
		t.Fatal("mutated creation identity accepted", status)
	}
}

func TestCollectionCreationReceiptAtomicAndNoBackfill(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	input := service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic receipt", Kind: "manual", Scope: "private"}
	hits := 0
	f.Service.DB.Callback().Create().Before("gorm:create").Register("creation_receipt_fail", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			hits++
			tx.AddError(errors.New("synthetic receipt failure"))
		}
	})
	_, failure := f.Service.CreateFileCollection(p, input, "atomic-receipt")
	f.Service.DB.Callback().Create().Remove("creation_receipt_fail")
	var count int64
	f.Service.DB.Model(&service.FileCollection{}).Where("key_id = ?", "atomic-receipt").Count(&count)
	if hits != 1 || failure == nil || count != 0 {
		t.Fatal("receipt not atomic", hits, failure, count)
	}
	created, err := f.Service.CreateFileCollection(p, input, "atomic-receipt")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Where("action = ? AND resource_id = ?", "collection.create", created.CollectionID).Delete(&service.WorkspaceRequest{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateFileCollection(p, input, "atomic-receipt"); err != nil {
		t.Fatal("legacy replay broken", err)
	}
	if status := h.call(t, "POST", "/file-collections/lookup", map[string]string{"idempotency_key": "atomic-receipt"}, nil); status != 404 {
		t.Fatal("legacy row fabricated receipt", status)
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err = agentClient.JSON(t.Context(), "POST", "/api/v1/file-collections", "agent-created", input, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateFileCollection(p, input, "agent-created"); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "POST", "/file-collections/lookup", map[string]string{"idempotency_key": "agent-created"}, nil); status != 404 {
		t.Fatal("agent row acquired human receipt", status)
	}
}
