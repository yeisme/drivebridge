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

func TestWorkspaceGroupRenameHTTPRevisionAndReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	group := service.WorkspaceAgentGroup{ID: service.ID(), Owner: h.user.ID, Name: "Original", NormalizedName: "original", Revision: 1}
	if err := f.Service.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	call := func(user *human, key, name string, revision uint64) (int, contract.AgentGroupView) {
		t.Helper()
		body, err := json.Marshal(contract.RenameGroupRequest{Name: name, ExpectedRevision: revision})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("PATCH", user.base+"/api/v1/agent-groups/"+group.ID, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", user.base)
		req.Header.Set("X-CSRF-Token", user.csrf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := user.http.Do(req)
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
	if status, _ := call(bob, "key", "Foreign", 1); status != 404 {
		t.Fatalf("foreign status %d", status)
	}
	const callback = "group_rename_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if a, ok := tx.Statement.Dest.(*service.Audit); ok && a.Action == "workspace.group.rename" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed, _ := call(h, "key", " Renamed ", 1)
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed != 503 {
		t.Fatalf("audit status %d", failed)
	}
	var retained service.WorkspaceAgentGroup
	if err := f.Service.DB.First(&retained, "id = ?", group.ID).Error; err != nil || retained.Name != "Original" || retained.Revision != 1 {
		t.Fatal("failed audit committed rename")
	}
	for i := 0; i < 2; i++ {
		status, view := call(h, "key", " Renamed ", 1)
		if status != 200 || view.Name != "Renamed" || view.Revision != 2 {
			t.Fatal("rename or replay failed")
		}
	}
	if status, _ := call(h, "key", "Different", 1); status != 409 {
		t.Fatalf("changed request status %d", status)
	}
	if status, _ := call(h, "stale", "Other", 1); status != 409 {
		t.Fatalf("stale revision status %d", status)
	}
	var count int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("action = ?", "group.rename").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("rename receipt duplicated")
	}
	if err := f.Service.DB.Model(&service.Audit{}).Where("action = ?", "workspace.group.rename").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("rename audit duplicated")
	}
	if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := call(h, "key", " Renamed ", 1); status != 403 {
		t.Fatalf("disabled replay status %d", status)
	}
}
