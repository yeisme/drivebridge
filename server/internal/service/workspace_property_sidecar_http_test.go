package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestPropertySidecarHTTPGateClosedAndRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "properties/http.png", "synthetic image")
	if _, err := f.Service.UpdatePropertyDefinitions(t.Context(), p, h.space.ID, contract.PropertyDefinitionUpdate{Definitions: []contract.PropertyDefinition{{Key: "caption", DisplayName: "Caption", Type: "text"}, {Key: "rank", DisplayName: "Rank", Type: "number"}}}, "http-sidecar-definitions"); err != nil {
		t.Fatal(err)
	}
	path := "/files/" + file.FileID + "/properties"
	patch := func(raw []byte, key string) (int, contract.PropertySidecarReceipt) {
		t.Helper()
		req, _ := http.NewRequest("PATCH", h.base+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.PropertySidecarReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("sidecar response cacheable")
		}
		return res.StatusCode, env.Data
	}
	input := contract.PropertySidecarPatch{BaseVersionID: file.VersionID, DefinitionRevision: 1, Updates: map[string]json.RawMessage{"caption": json.RawMessage(`"Original"`), "rank": json.RawMessage(`1.250`)}, RemoveKeys: []string{}}
	raw, _ := json.Marshal(input)
	if f.Service.Config.PropertyEditing {
		t.Fatal("sidecar write enabled by default")
	}
	if status, _ := patch(raw, "sidecar-http"); status != 422 {
		t.Fatal("disabled patch allowed", status)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspacePropertySidecar{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("disabled mutation persisted", err)
	}
	f.Service.Config.PropertyEditing = true
	status, first := patch(raw, "sidecar-http")
	if status != 200 || first.ResultRevision != 1 || first.VersionID != file.VersionID {
		t.Fatal("HTTP patch failed", status)
	}
	var view contract.PropertySidecarView
	if status = h.call(t, "GET", path+"?version_id="+file.VersionID, nil, &view); status != 200 || string(view.Values["rank"]) != "1.250" {
		t.Fatal("raw number coerced", status)
	}
	prefix := `{"base_version_id":"` + file.VersionID + `","definition_revision":1,"expected_revision":1,"remove_keys":[],"updates":`
	for _, bad := range []string{
		`{}`, prefix + `null}`, prefix + `{"caption":"a","caption":"b"}}`, prefix + `{"caption":"a","\u0063aption":"b"}}`, prefix + `{"caption":"\ud800"}}`, prefix + `{"caption":null}}`, prefix + `{"caption":"ok"},"owner":"spoof"}`, prefix + `{"rank":9007199254740991.1}}`,
	} {
		if status, _ = patch([]byte(bad), "invalid-sidecar"); status != 400 {
			t.Fatal("invalid/duplicate/surrogate accepted", status)
		}
	}
	invalidUTF8 := append([]byte(prefix+`{"caption":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}}`)...)
	if status, _ = patch(invalidUTF8, "bad-utf8"); status != 400 {
		t.Fatal("invalid UTF8 repaired", status)
	}
	if status, _ = patch([]byte(strings.Repeat(" ", 256*1024)+string(raw)), "too-large"); status != 400 {
		t.Fatal("wire budget bypass", status)
	}
	for _, query := range []string{"", "?version_id=" + file.VersionID + "&version_id=" + file.VersionID, "?version_id=bad", "?version_id=" + file.VersionID + "&extra=1"} {
		if status = h.call(t, "GET", path+query, nil, nil); status != 400 {
			t.Fatal("invalid fixed query", status)
		}
	}
	f.Service.Config.PropertyEditing = false
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("HTTP sidecar read wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("sidecar_http_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("sidecar_http_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("sidecar_http_no_write", guard)
	readStatus := h.call(t, "GET", path+"?version_id="+file.VersionID, nil, &view)
	var receipt contract.PropertySidecarReceipt
	lookupStatus := h.call(t, "POST", path+"/lookup", contract.PropertySidecarLookup{VersionID: file.VersionID, IdempotencyKey: "sidecar-http"}, &receipt)
	f.Service.DB.Callback().Create().Remove("sidecar_http_no_write")
	f.Service.DB.Callback().Update().Remove("sidecar_http_no_write")
	f.Service.DB.Callback().Delete().Remove("sidecar_http_no_write")
	if writes != 0 || readStatus != 200 || lookupStatus != 200 || receipt != first {
		t.Fatal("read recovery unavailable after gate disable", writes, readStatus, lookupStatus)
	}
	if status = bob.call(t, "GET", path+"?version_id="+file.VersionID, nil, nil); status != 404 {
		t.Fatal("foreign sidecar visible", status)
	}
	if status = bob.call(t, "POST", path+"/lookup", contract.PropertySidecarLookup{VersionID: file.VersionID, IdempotencyKey: "sidecar-http"}, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	for _, body := range []string{`{}`, `{"version_id":"` + file.VersionID + `","idempotency_key":"x","idempotency_key":"y"}`, `{"version_id":null,"idempotency_key":"x"}`} {
		if status, _ := fpRawCall(t, h, "POST", path+"/lookup", []byte(body)); status != 400 {
			t.Fatal("lookup not closed", status)
		}
	}
}
