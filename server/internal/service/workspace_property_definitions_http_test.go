package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestPropertyDefinitionHTTPClosedAndRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	path := "/spaces/" + h.space.ID + "/property-definitions"
	put := func(raw []byte, keys ...string) (int, contract.PropertyDefinitionReceipt) {
		t.Helper()
		req, _ := http.NewRequest("PUT", h.base+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		for _, key := range keys {
			req.Header.Add("Idempotency-Key", key)
		}
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.PropertyDefinitionReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("dictionary response cacheable")
		}
		return res.StatusCode, env.Data
	}
	var empty contract.PropertyDefinitionView
	if status := h.call(t, "GET", path, nil, &empty); status != 200 || empty.Revision != 0 || empty.Definitions == nil {
		t.Fatal("empty HTTP dictionary failed", status)
	}
	raw := []byte(`{"expected_revision":0,"definitions":[{"key":"status","display_name":"Status","group":"Project","type":"text"}]}`)
	status, first := put(raw, "http-property-first")
	if status != 200 || first.ResultRevision != 1 {
		t.Fatal("dictionary PUT failed", status)
	}
	secondRaw := []byte(`{"expected_revision":1,"definitions":[{"key":"status","display_name":"Progress","group":"","type":"text"}]}`)
	if status, _ = put(secondRaw, "http-property-second"); status != 200 {
		t.Fatal("dictionary edit failed", status)
	}
	status, replay := put(raw, "http-property-first")
	if status != 200 || replay != first {
		t.Fatal("PUT replay not original", status)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("dictionary observation wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("property_http_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("property_http_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("property_http_no_write", guard)
	var view contract.PropertyDefinitionView
	var receipt contract.PropertyDefinitionReceipt
	readStatus := h.call(t, "GET", path, nil, &view)
	lookupStatus := h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "http-property-first"}, &receipt)
	f.Service.DB.Callback().Create().Remove("property_http_no_write")
	f.Service.DB.Callback().Update().Remove("property_http_no_write")
	f.Service.DB.Callback().Delete().Remove("property_http_no_write")
	if writes != 0 || readStatus != 200 || lookupStatus != 200 || receipt != first || view.Revision != 2 || view.Definitions[0].DisplayName != "Progress" {
		t.Fatal("lookup/current view conflated", writes, readStatus, lookupStatus)
	}
	for _, bad := range []string{
		`{}`, `{"expected_revision":2,"definitions":null}`, `{"expected_revision":2,"definitions":[],"definitions":[]}`,
		`{"expected_revision":2,"definitions":[{"key":"status","display_name":"Status","type":"text"}]}`,
		`{"expected_revision":2,"definitions":[{"key":"status","display_name":"Status","group":null,"type":"text"}]}`,
		`{"expected_revision":2,"definitions":[{"key":"status","display_name":"Status","group":"","type":"text","key":"other"}]}`,
		`{"expected_revision":2,"definitions":[{"key":"status","display_name":"Status","group":"","type":"text","owner":"spoof"}]}`,
		`{"expected_revision":2,"definitions":[{"key":"status","display_name":"Status","group":"","type":"script"}]}`,
	} {
		if status, _ = put([]byte(bad), "bad"); status != 400 {
			t.Fatal("closed nested request accepted", status)
		}
	}
	if status, _ = put(raw); status != 400 {
		t.Fatal("missing key accepted", status)
	}
	if status, _ = put(raw, "key-a", "key-b"); status != 400 {
		t.Fatal("multiple keys accepted", status)
	}
	if status, _ = put([]byte(strings.Repeat(" ", 256*1024)+string(raw)), "oversize"); status != 400 {
		t.Fatal("body budget ignored", status)
	}
	if status, _ = put([]byte(`{"expected_revision":2,"definitions":[]}`), "remove-key"); status != 422 {
		t.Fatal("deletion bypassed migration", status)
	}
	for _, method := range []string{"GET", "PUT"} {
		if status = h.call(t, method, path+"?extra=1", map[string]any{}, nil); status != 400 {
			t.Fatal("query accepted", status)
		}
	}
	if status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "missing"}, nil); status != 404 {
		t.Fatal("missing lookup not distinct", status)
	}
	for _, bad := range []string{`{}`, `{"idempotency_key":null}`, `{"idempotency_key":"x","idempotency_key":"y"}`, `{"idempotency_key":"x","owner":"spoof"}`} {
		if status, _ := fpRawCall(t, h, "POST", path+"/lookup", []byte(bad)); status != 400 {
			t.Fatal("lookup not closed", status)
		}
	}
	if status = bob.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatal("foreign catalog visible", status)
	}
	if status = bob.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "http-property-first"}, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err := agentClient.JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("agent dictionary access")
	}
	hits := 0
	f.Service.DB.Callback().Query().Before("gorm:query").Register("property_receipt_query_fail", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			hits++
			tx.AddError(errors.New("synthetic receipt query failure"))
		}
	})
	status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "http-property-first"}, nil)
	f.Service.DB.Callback().Query().Remove("property_receipt_query_fail")
	if hits == 0 || status != 500 {
		t.Fatal("receipt dependency failure hidden", status)
	}
}
