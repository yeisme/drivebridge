package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceLabelHTTPClosedReadOnlyAndDefaultGate(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	path := "/spaces/" + h.space.ID + "/label-definitions"
	put := func(raw, key string) int {
		t.Helper()
		req, err := http.NewRequest("PUT", h.base+"/api/v1"+path, bytes.NewBufferString(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope map[string]any
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	raw := `{"expected_revision":0,"definitions":[{"key":"source","display_name":"Source","group":""}]}`
	var page contract.LabelDefinitionPage
	if status := h.call(t, "GET", path, nil, &page); status != 200 || page.ManagementEnabled || page.Revision != 0 {
		t.Fatal("default gate read", status, page)
	}
	if status := put(raw, "disabled"); status != 422 {
		t.Fatal("default mutation enabled", status)
	}
	f.Service.Config.LabelManagement = true
	if status := put(raw, "dictionary"); status != 200 {
		t.Fatal("dictionary write", status)
	}
	for _, bad := range []string{
		`{"expected_revision":1,"definitions":null}`,
		`{"expected_revision":1,"definitions":[],"definitions":[]}`,
		`{"expected_revision":1,"definitions":[{"key":"source","display_name":"Source"}]}`,
		`{"expected_revision":1,"definitions":[{"key":"source","display_name":"Source","group":"","key":"target"}]}`,
		`{"expected_revision":1,"definitions":[{"key":"source","display_name":"Source","group":"","namespace":"rule"}]}`,
	} {
		if status := put(bad, "closed"); status != 400 {
			t.Fatal("open request", status, bad)
		}
	}
	for _, suffix := range []string{"?namespace=rule", "?namespace=manual&namespace=manual", "?limit=201", "?unknown=1"} {
		if status := h.call(t, "GET", path+suffix, nil, nil); status != 400 {
			t.Fatal("invalid query", suffix, status)
		}
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("read wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("label-read", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("label-read", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("label-read", guard)
	var receipt contract.LabelDefinitionReceipt
	readStatus := h.call(t, "GET", path, nil, &page)
	lookupStatus := h.call(t, "POST", path+"/lookup", contract.IdempotencyLookupRequest{IdempotencyKey: "dictionary"}, &receipt)
	f.Service.DB.Callback().Create().Remove("label-read")
	f.Service.DB.Callback().Update().Remove("label-read")
	f.Service.DB.Callback().Delete().Remove("label-read")
	if writes != 0 || readStatus != 200 || lookupStatus != 200 || receipt.ResultRevision != 1 {
		t.Fatal("observation wrote", writes, readStatus, lookupStatus)
	}
	_, a := agent(t, h, "", "read,write")
	if err := a.JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("agent dictionary read")
	}
}
