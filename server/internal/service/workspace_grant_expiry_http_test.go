package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceGrantExpiryHTTP(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "refs", "read")
	g := service.WorkspaceGrant{ID: service.ID(), AgentID: a.ID, Owner: h.user.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: a.Prefix, Protocol: "mcp", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	patch := func(key, body string) (int, contract.GrantView) {
		t.Helper()
		req, err := http.NewRequest("PATCH", h.base+"/api/v1/agent-grants/"+g.ID, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var env contract.DataEnvelope[contract.GrantView]
		if err = json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, env.Data
	}
	for _, body := range []string{`{"expected_revision":1,"expiry_mode":"duration"}`, `{"expected_revision":1,"expiry_mode":"never","expires_in_seconds":3600}`, `{"expected_revision":1,"expires_in_seconds":3600}`} {
		if status, _ := patch("invalid", body); status != 422 {
			t.Fatalf("invalid expiry status %d", status)
		}
	}
	body := `{"expected_revision":1,"expiry_mode":"duration","expires_in_seconds":3600}`
	before := time.Now().UTC()
	status, first := patch("duration", body)
	after := time.Now().UTC()
	if status != 200 || first.ExpiresAt == nil || first.ExpiryChangedAt == nil || first.Revision != 2 {
		t.Fatal("expiry change failed")
	}
	if first.ExpiresAt.Before(before.Add(time.Hour)) || first.ExpiresAt.After(after.Add(time.Hour)) {
		t.Fatal("expiry not based on server change time")
	}
	if !first.ExpiresAt.Equal(first.ExpiryChangedAt.Add(time.Hour)) {
		t.Fatal("inconsistent deadline")
	}
	status, replay := patch("duration", body)
	if status != 200 || !replay.ExpiresAt.Equal(*first.ExpiresAt) || !replay.ExpiryChangedAt.Equal(*first.ExpiryChangedAt) {
		t.Fatal("replay renewed expiry")
	}
	status, never := patch("never", `{"expected_revision":2,"expiry_mode":"never"}`)
	if status != 200 || never.ExpiresAt != nil || never.ExpiryMode != "never" || never.Revision != 3 {
		t.Fatal("never transition failed")
	}
	revision := uint64(3)
	for _, seconds := range []int64{3600, 86400, 604800, 2592000, 7776000, 31536000} {
		request := fmt.Sprintf(`{"expected_revision":%d,"expiry_mode":"duration","expires_in_seconds":%d}`, revision, seconds)
		status, view := patch(fmt.Sprintf("duration-%d", seconds), request)
		if status != 200 || view.Revision != revision+1 || view.ExpiresAt == nil || view.ExpiryChangedAt == nil || !view.ExpiresAt.Equal(view.ExpiryChangedAt.Add(time.Duration(seconds)*time.Second)) {
			t.Fatalf("duration %d failed", seconds)
		}
		revision++
	}
	var beforeFailure service.WorkspaceGrant
	if err := f.Service.DB.First(&beforeFailure, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	callback := "workspace_expiry_audit_failure"
	if err := f.Service.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.grant.expiry" {
			tx.AddError(errors.New("fixture audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, _ = patch("failed-expiry", fmt.Sprintf(`{"expected_revision":%d,"expiry_mode":"never"}`, revision))
	if err := f.Service.DB.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if status != 503 {
		t.Fatalf("audit failure status %d", status)
	}
	var afterFailure service.WorkspaceGrant
	if err := f.Service.DB.First(&afterFailure, "id = ?", g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailure.Revision != beforeFailure.Revision || afterFailure.ExpiresAt == nil || !afterFailure.ExpiresAt.Equal(*beforeFailure.ExpiresAt) {
		t.Fatal("audit failure committed expiry")
	}
	var failedReceipts int64
	if err := f.Service.DB.Model(&service.WorkspaceRequest{}).Where("resource_id = ? AND request_key = ?", g.ID, "failed-expiry").Count(&failedReceipts).Error; err != nil {
		t.Fatal(err)
	}
	if failedReceipts != 0 {
		t.Fatal("failed mutation retained receipt")
	}
	var audits int64
	if err := f.Service.DB.Model(&service.Audit{}).Where("resource = ? AND action = ?", g.ID, "workspace.grant.expiry").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 8 {
		t.Fatalf("expiry audit count %d", audits)
	}
}
