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

func TestWorkspaceGroupCreateHTTPReceiptsAndRollback(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	call := func(key, name string) (int, contract.AgentGroupView) {
		t.Helper()
		body, err := json.Marshal(contract.CreateGroupRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", h.base+"/api/v1/agent-groups", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.AgentGroupView]
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	const callback = "fixture_group_create_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.group.create" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call("key", " Research ")
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit status %d", failed)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceAgentGroup{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed audit retained group")
	}
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("action = ?", "group.create").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed audit retained receipt")
	}
	status, created := call("key", " Research ")
	if status != 201 || created.Name != "Research" || created.Revision != 1 {
		t.Fatalf("creation status %d", status)
	}
	status, replay := call("key", " Research ")
	if status != 201 || replay.GroupID != created.GroupID {
		t.Fatal("replay created group")
	}
	if status, _ := call("key", "Different"); status != 409 {
		t.Fatalf("changed key status %d", status)
	}
	if status, _ := call("another", "research"); status != 409 {
		t.Fatalf("duplicate name status %d", status)
	}
	var read contract.AgentGroupView
	if status := h.call(t, "GET", "/agent-groups/"+created.GroupID, nil, &read); status != 200 || read.GroupID != created.GroupID {
		t.Fatal("created group unreadable")
	}
	if err := f.Service.DB.Model(&service.Audit{}).Where("action = ?", "workspace.group.create").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("replay duplicated audit")
	}
}
