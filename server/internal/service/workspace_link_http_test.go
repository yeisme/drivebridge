package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLinksHTTPReadOnlyAndRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	client := mustSDK(t, h)
	target := orgUpload(t, client, h.space.ID, "link-http/target.md", "# Target")
	source := orgUpload(t, client, h.space.ID, "link-http/source.md", "[Target](target.md)")
	base := "/files/" + source.FileID + "/links"
	var before contract.FileLinkList
	if status := h.call(t, "GET", base+"?version_id="+source.VersionID, nil, &before); status != 200 || before.Indexed || len(before.Items) != 0 {
		t.Fatal(status, before)
	}
	raw, _ := json.Marshal(contract.FileLinkRebuild{VersionID: source.VersionID})
	request := func(key string) (int, contract.FileLinkReceipt) {
		t.Helper()
		req, _ := http.NewRequest("POST", h.base+"/api/v1"+base+"/rebuild", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.FileLinkReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	if status, _ := request(""); status != 400 {
		t.Fatal("missing key", status)
	}
	status, first := request("http-links-first")
	if status != 200 || first.FileID != source.FileID || first.VersionID != source.VersionID {
		t.Fatal(status, first)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("link read wrote")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("links_no_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("links_no_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("links_no_write", guard)
	var links, backlinks contract.FileLinkList
	var receipt contract.FileLinkReceipt
	readStatus := h.call(t, "GET", base+"?version_id="+source.VersionID, nil, &links)
	backStatus := h.call(t, "GET", "/files/"+target.FileID+"/backlinks?version_id="+target.VersionID, nil, &backlinks)
	lookupStatus := h.call(t, "POST", base+"/lookup", contract.FileLinkLookup{VersionID: source.VersionID, IdempotencyKey: "http-links-first"}, &receipt)
	f.Service.DB.Callback().Create().Remove("links_no_write")
	f.Service.DB.Callback().Update().Remove("links_no_write")
	f.Service.DB.Callback().Delete().Remove("links_no_write")
	if writes != 0 || readStatus != 200 || backStatus != 200 || lookupStatus != 200 || receipt != first || len(links.Items) != 1 || len(backlinks.Items) != 1 || links.Items[0].TargetPath != "link-http/target.md" {
		t.Fatal("read/recovery semantics", writes, readStatus, backStatus, lookupStatus, links)
	}
	for _, query := range []string{"", "?version_id=x", "?version_id=" + source.VersionID + "&version_id=" + source.VersionID, "?version_id=" + source.VersionID + "&extra=1"} {
		if status := h.call(t, "GET", base+query, nil, nil); status != 400 {
			t.Fatal("open query", query, status)
		}
	}
	p := service.Principal{UserID: h.user.ID}
	f.Service.DB.Callback().Create().Before("gorm:create").Register("links_receipt_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			tx.AddError(errors.New("synthetic receipt failure"))
		}
	})
	_, failure := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, contract.FileLinkRebuild{VersionID: source.VersionID}, "links-fail")
	f.Service.DB.Callback().Create().Remove("links_receipt_failure")
	if failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal(failure)
	}
	links, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || len(links.Items) != 1 {
		t.Fatal("failed rebuild removed old projection", links, err)
	}
	if _, err = f.Service.LookupWorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, "links-fail"); err == nil || err.Error() != "not_found" {
		t.Fatal("failed receipt committed", err)
	}
}
