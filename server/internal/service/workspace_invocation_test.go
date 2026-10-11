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

func TestWorkspaceInvocationObservedTransportsAndFailures(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	raw, _ := json.Marshal(map[string]any{"name": "Observer", "protocol": "http", "space_id": h.space.ID, "connection_id": h.space.ConnectionID, "prefix": "refs", "actions": []string{"files.read"}})
	req, err := http.NewRequest("POST", h.base+"/api/v1/agent-grants", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "observed-agent")
	response, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var issued contract.DataEnvelope[contract.GrantIssuedView]
	decodeErr := json.NewDecoder(response.Body).Decode(&issued)
	response.Body.Close()
	if decodeErr != nil || response.StatusCode != 201 {
		t.Fatal("grant fixture failed")
	}
	read := func(path string) int {
		t.Helper()
		request, err := http.NewRequest("GET", h.base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+issued.Data.Credential.Secret)
		request.Header.Set("X-Request-ID", "caller-injected-request")
		res, err := h.http.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if status := read("/api/v1/files?space_id=" + h.space.ID + "&prefix=refs"); status != 200 {
		t.Fatalf("agent read %d", status)
	}
	// MCP tool invocation is captured at the tool boundary, not as HTTP /mcp.
	mc := workspaceMCP(t, h.base, issued.Data.Credential.Secret, "capabilities", "stat", "files", "search")
	workspaceMCPCall(t, mc, "capabilities", nil, false)
	rows := []service.WorkspaceInvocation{}
	if err = f.Service.DB.Where("agent_id = ?", issued.Data.Grant.AgentID).Order("started_at").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	httpSeen, mcpSeen := false, false
	for _, row := range rows {
		if row.Owner != h.user.ID || row.GrantID != issued.Data.Grant.GrantID || row.SpaceID != h.space.ID || len(row.RequestID) != 48 || row.RequestID == "caller-injected-request" || row.FinishedAt == nil || row.FinishedAt.Before(row.StartedAt) || row.Result != "unknown" {
			t.Fatal("fabricated or misbound observation")
		}
		if row.Protocol == "http" && row.Action == "files.list" {
			httpSeen = true
		}
		if row.Protocol == "mcp" && row.Action == "capabilities" {
			mcpSeen = true
		}
		encoded, _ := json.Marshal(row)
		for _, secret := range []string{issued.Data.Credential.Secret, "caller-injected-request", "Authorization", "prefix=refs"} {
			if bytes.Contains(encoded, []byte(secret)) {
				t.Fatal("private transport input persisted")
			}
		}
	}
	if !httpSeen || !mcpSeen {
		t.Fatal("missing actual transport observation")
	}
	var observed contract.InvocationPage
	if status := h.call(t, "GET", "/agent-invocations?agent_id="+issued.Data.Grant.AgentID, nil, &observed); status != 200 || len(observed.Items) != len(rows) || observed.Coverage != "best_effort" {
		t.Fatal("real observations not readable", status)
	}
	for _, item := range observed.Items {
		if len(item.ResourceRefs) != 0 || item.OperationRef != "" || item.Result != "unknown" {
			t.Fatal("unproven business references exposed")
		}
	}
	if status := read("/api/v1/agent-invocations"); status != 401 {
		t.Fatal("Agent read owner call catalogue")
	}
	// Start failure never blocks the existing business request.
	const createFault = "invocation_create_failure"
	if err = f.Service.DB.Callback().Create().Before("gorm:create").Register(createFault, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*service.WorkspaceInvocation); ok {
			tx.AddError(errors.New("synthetic observation store failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status := read("/api/v1/files?space_id=" + h.space.ID + "&prefix=refs")
	if err = f.Service.DB.Callback().Create().Remove(createFault); err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatal("telemetry allocation changed read result")
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceInvocation{}).Count(&count).Error; err != nil || count != int64(len(rows)) {
		t.Fatal("failed start fabricated record")
	}
	// Finish failure must preserve unknown with a missing finished timestamp.
	const updateFault = "invocation_finish_failure"
	if err = f.Service.DB.Callback().Update().Before("gorm:update").Register(updateFault, func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_invocations" {
			tx.AddError(errors.New("synthetic finish failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status = read("/api/v1/files?space_id=" + h.space.ID + "&prefix=refs")
	if err = f.Service.DB.Callback().Update().Remove(updateFault); err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatal("telemetry finish changed read result")
	}
	var unfinished service.WorkspaceInvocation
	if err = f.Service.DB.Where("finished_at IS NULL").First(&unfinished).Error; err != nil || unfinished.Result != "unknown" {
		t.Fatal("failed finish invented completion")
	}
	// A safe handler-level failure is observed, without recording path input.
	if status = read("/api/v1/files/" + strings.Repeat("a", 48)); status != 404 {
		t.Fatalf("missing file %d", status)
	}
	var missing service.WorkspaceInvocation
	if err = f.Service.DB.Where("action = ? AND error_code = ?", "files.stat", "not_found").First(&missing).Error; err != nil || missing.FinishedAt == nil || missing.Result != "unknown" {
		t.Fatal("handler error lost or claimed rollback")
	}
}
