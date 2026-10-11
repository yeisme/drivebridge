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

func TestCollectionMembersChangeOriginalReceipt(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	a := orgUpload(t, client, h.space.ID, "receipt/a.txt", "a")
	b := orgUpload(t, client, h.space.ID, "receipt/b.txt", "b")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Decisions", Kind: "manual", Scope: "private"}, "member-receipt-collection")
	if err != nil {
		t.Fatal(err)
	}
	path := "/file-collections/" + c.CollectionID + "/members/changes"
	change := func(raw []byte, key string) (int, contract.CollectionMembersReceipt) {
		t.Helper()
		req, _ := http.NewRequest("POST", h.base+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.CollectionMembersReceipt]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	input := contract.CollectionMembersChangeRequest{ExpectedRevision: 1, Include: []string{a.FileID}, Exclude: []string{}}
	raw, _ := json.Marshal(input)
	status, first := change(raw, "member-original")
	if status != 200 || first.CollectionID != c.CollectionID || first.ResultRevision != 2 || first.RequestDigest == "" {
		t.Fatal("change failed", status)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: 2, Include: []string{b.FileID}}); err != nil {
		t.Fatal(err)
	}
	// The old included file can disappear. Receipts disclose no member identities
	// and only attest the old commit, not current readability or membership.
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", a.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	status, replay := change(raw, "member-original")
	if status != 200 || replay != first {
		t.Fatal("replay changed old result", status)
	}
	current, err := f.Service.ReadFileCollection(p, c.CollectionID)
	if err != nil || current.Revision != 3 || len(current.Items) != 1 || current.Items[0].FileID != b.FileID {
		t.Fatal("replay rewrote current members", err)
	}
	writes := 0
	guard := func(tx *gorm.DB) { writes++; tx.AddError(errors.New("lookup attempted mutation")) }
	f.Service.DB.Callback().Create().Before("gorm:create").Register("members_lookup_write", guard)
	f.Service.DB.Callback().Update().Before("gorm:update").Register("members_lookup_write", guard)
	f.Service.DB.Callback().Delete().Before("gorm:delete").Register("members_lookup_write", guard)
	var lookup contract.CollectionMembersReceipt
	status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "member-original"}, &lookup)
	f.Service.DB.Callback().Create().Remove("members_lookup_write")
	f.Service.DB.Callback().Update().Remove("members_lookup_write")
	f.Service.DB.Callback().Delete().Remove("members_lookup_write")
	if status != 200 || writes != 0 || lookup != first {
		t.Fatal("lookup not original or wrote", status, writes)
	}
	input.Include = []string{b.FileID}
	different, _ := json.Marshal(input)
	if status, _ = change(different, "member-original"); status != 409 {
		t.Fatal("different body accepted", status)
	}
	if status, _ = change(different, "new-key-stale"); status != 409 {
		t.Fatal("stale revision committed", status)
	}
	if status = bob.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "member-original"}, nil); status != 404 {
		t.Fatal("foreign receipt visible", status)
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.LookupCollectionMembersChange(t.Context(), bad, c.CollectionID, "member-original"); err == nil {
			t.Fatal("scope bypass")
		}
	}
	_, agentClient := agent(t, h, "", "read,write")
	if err = agentClient.JSON(t.Context(), "POST", "/api/v1"+path, "agent-member-change", input, nil); err == nil {
		t.Fatal("agent acquired human mutation")
	}
	for _, bad := range []string{`{}`, `{"expected_revision":1,"include":null,"exclude":[]}`, `{"expected_revision":1,"include":[],"exclude":[],"owner":"spoof"}`, `{"expected_revision":1,"include":[],"exclude":[],"include":[]}`} {
		if status, _ = change([]byte(bad), "bad"); status != 400 {
			t.Fatal("non-closed body accepted", status)
		}
	}
	for _, table := range []string{"workspace_requests", "file_collections", "users", "spaces"} {
		hits := 0
		f.Service.DB.Callback().Query().Before("gorm:query").Register("members_lookup_fail", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				hits++
				tx.AddError(errors.New("synthetic dependency failure"))
			}
		})
		status = h.call(t, "POST", path+"/lookup", map[string]string{"idempotency_key": "member-original"}, nil)
		f.Service.DB.Callback().Query().Remove("members_lookup_fail")
		if hits == 0 || status != 500 {
			t.Fatal("dependency failure hidden", table, status)
		}
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.LookupCollectionMembersChange(t.Context(), p, c.CollectionID, "member-original"); err == nil {
		t.Fatal("disabled receipt read")
	}
}

func TestCollectionMembersChangeAtomicReceipt(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	a := orgUpload(t, mustSDK(t, h), h.space.ID, "atomic/a.txt", "a")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic members", Kind: "manual", Scope: "private"}, "atomic-member-collection")
	if err != nil {
		t.Fatal(err)
	}
	input := contract.CollectionMembersChangeRequest{ExpectedRevision: 1, Include: []string{a.FileID}, Exclude: []string{}}
	hits := 0
	f.Service.DB.Callback().Create().Before("gorm:create").Register("members_receipt_fail", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			hits++
			tx.AddError(errors.New("synthetic receipt failure"))
		}
	})
	_, failure := f.Service.ChangeCollectionMembers(t.Context(), p, c.CollectionID, input, "atomic-members")
	f.Service.DB.Callback().Create().Remove("members_receipt_fail")
	if hits != 1 || failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal("receipt write failure lost", hits, failure)
	}
	view, err := f.Service.ReadFileCollection(p, c.CollectionID)
	if err != nil || view.Revision != 1 || len(view.Items) != 0 {
		t.Fatal("failed receipt left member change", err)
	}
	if _, err = f.Service.LookupCollectionMembersChange(t.Context(), p, c.CollectionID, "atomic-members"); err == nil || err.Error() != "not_found" {
		t.Fatal("failed mutation has receipt", err)
	}
	first, err := f.Service.ChangeCollectionMembers(t.Context(), p, c.CollectionID, input, "atomic-members")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.Service.ChangeCollectionMembers(t.Context(), p, c.CollectionID, input, "atomic-members")
	if err != nil || replay != first {
		t.Fatal("identity changed", err)
	}
	second, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Other", Kind: "manual", Scope: "private"}, "other-member-collection")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.LookupCollectionMembersChange(t.Context(), p, second.CollectionID, "atomic-members"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("key substituted target", err)
	}
	input.Include = []string{a.FileID, a.FileID}
	if _, err = f.Service.ChangeCollectionMembers(t.Context(), p, c.CollectionID, input, "duplicate"); err == nil {
		t.Fatal("duplicate members accepted")
	}
}
