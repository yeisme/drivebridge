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

func TestWorkspaceGroupDeleteHTTPAtomicReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	other, _ := login(t, f, "bob")
	group := service.WorkspaceAgentGroup{ID: service.ID(), Owner: h.user.ID, Name: "Research", NormalizedName: "research", Revision: 1}
	org := service.WorkspaceAgentOrganization{ID: service.ID(), Owner: h.user.ID, AgentID: "fixture-agent", GroupID: &group.ID, Pinned: true, Revision: 1}
	for _, row := range []any{&group, &org} {
		if err := f.Service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	call := func(user *human, key string, revision uint64) (int, contract.DeletedGroupView) {
		t.Helper()
		body, err := json.Marshal(contract.RevisionRequest{ExpectedRevision: revision})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("DELETE", user.base+"/api/v1/agent-groups/"+group.ID, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", user.base)
		req.Header.Set("X-CSRF-Token", user.csrf)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		res, err := user.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.DeletedGroupView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	if status, _ := call(other, "key", 1); status != 404 {
		t.Fatalf("foreign deletion status %d", status)
	}
	if status, _ := call(h, "key", 2); status != 409 {
		t.Fatalf("stale revision status %d", status)
	}
	const callback = "group_delete_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.group.delete" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call(h, "key", 1)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit failure status %d", failed)
	}
	var before service.WorkspaceAgentOrganization
	if err := f.Service.DB.First(&before, "id = ?", org.ID).Error; err != nil || before.GroupID == nil || before.Revision != 1 {
		t.Fatal("failed delete unbound organization")
	}
	for i := 0; i < 2; i++ {
		status, out := call(h, "key", 1)
		if status != 200 || !out.Deleted || out.GroupID != group.ID {
			t.Fatal("deletion replay failed")
		}
	}
	var after service.WorkspaceAgentOrganization
	if err := f.Service.DB.First(&after, "id = ?", org.ID).Error; err != nil || after.GroupID != nil || !after.Pinned || after.Revision != 2 {
		t.Fatal("replay changed pin or revision")
	}
	if status, _ := call(h, "key", 2); status != 409 {
		t.Fatalf("changed replay status %d", status)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("action = ?", "group.delete").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("delete receipt duplicated")
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := call(h, "key", 1); status != 403 {
		t.Fatalf("disabled replay status %d", status)
	}
}
