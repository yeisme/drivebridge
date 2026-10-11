package service_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceOperationTaskProjection(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	now := time.Now().UTC()
	rows := []service.Operation{
		{ID: "task-a", Owner: alice.user.ID, SpaceID: alice.space.ID, Kind: "upload", Path: "a.md", Status: "verifying", Size: 100, Transferred: 100, Caller: "claimed-agent", TaskRef: "claimed-task", Updated: now},
		{ID: "task-b", Owner: alice.user.ID, ActorID: alice.user.ID, AgentID: "authenticated-agent", SpaceID: alice.space.ID, Kind: "upload", Path: "b.md", Status: "completed", FileID: "file-b", VersionID: "version-b", Updated: now},
		{ID: "task-c", Owner: alice.user.ID, ActorID: alice.user.ID, SpaceID: alice.space.ID, Kind: "copy", Path: "c.md", Status: "failed", Updated: now},
		{ID: "task-d", Owner: alice.user.ID, SpaceID: alice.space.ID, Kind: "future", Path: "d.md", Status: "new-owner-state", Updated: now},
	}
	for i := range rows {
		rows[i].Key = rows[i].ID
		if err := f.Service.DB.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	get := func(query string) contract.Page[contract.TaskProjection] {
		t.Helper()
		var page contract.Page[contract.TaskProjection]
		if status := alice.call(t, "GET", "/workspace/tasks?kind=operation&"+query, nil, &page); status != 200 {
			t.Fatalf("status %d", status)
		}
		return page
	}
	first := get("limit=1")
	if len(first.Items) != 1 || first.NextCursor == "" || first.Total != nil {
		t.Fatal("invalid page")
	}
	item := first.Items[0]
	if item.OwnerID != "task-a" || item.OwnerKind != "operation" || item.ActorKind != "unknown" || item.AgentID != nil || item.UIState != "running" || item.OwnerStatus != "verifying" || !strings.HasPrefix(item.Revision, "sha256:") || item.ObservedAt.IsZero() {
		t.Fatal("invented task or identity")
	}
	if item.Progress.BytesSent != nil || item.Progress.DurableBytes != nil || item.Progress.VerifiedBytes != nil || item.Progress.Total == nil || *item.Progress.Total != 1 || item.Progress.Pending == nil || *item.Progress.Pending != 1 || item.Progress.Succeeded != nil || len(item.AllowedActions) != 0 {
		t.Fatal("invented progress or executable actions")
	}
	if get("limit=1").Items[0].Revision != item.Revision {
		t.Fatal("observation time changed revision")
	}
	next := "limit=1&cursor=" + url.QueryEscape(first.NextCursor)
	if get(next).Items[0].OwnerID != "task-b" {
		t.Fatal("wrong next owner")
	}
	agentPage := get("agent_id=authenticated-agent")
	if len(agentPage.Items) != 1 || agentPage.Items[0].ActorKind != "agent" || agentPage.Items[0].UIState != "completed" {
		t.Fatal("persisted actor projection missing")
	}
	if len(get("agent_id=claimed-agent").Items) != 0 {
		t.Fatal("caller promoted to actor")
	}
	recovery := get("state=recoverable")
	if len(recovery.Items) != 1 || recovery.Items[0].OwnerStatus != "failed" || recovery.Items[0].ActorKind != "human" {
		t.Fatal("owner status lost")
	}
	if len(get("state=unknown").Items) != 1 {
		t.Fatal("unknown state dropped")
	}
	var hidden contract.Page[contract.TaskProjection]
	if status := bob.call(t, "GET", "/workspace/tasks?kind=operation", nil, &hidden); status != 200 || len(hidden.Items) != 0 {
		t.Fatal("foreign task visible")
	}
	if status := bob.call(t, "GET", "/workspace/tasks?kind=operation&"+next, nil, nil); status != 410 {
		t.Fatalf("foreign cursor %d", status)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation&"+next+"&state=running", nil, nil); status != 410 {
		t.Fatalf("query cursor %d", status)
	}
	for _, query := range []string{"", "kind=unsupported_owner", "kind=operation&state=bad", "kind=operation&agent_id=", "kind=operation&limit=201", "kind=operation&state=running&state=completed", "kind=operation&unknown=x"} {
		if status := alice.call(t, "GET", "/workspace/tasks?"+query, nil, nil); status != 400 {
			t.Fatalf("invalid query %q status %d", query, status)
		}
	}
	_, agentClient := agent(t, alice, "", "read,write")
	if err := agentClient.JSON(t.Context(), "GET", "/api/v1/workspace/tasks?kind=operation", "", nil, nil); err == nil {
		t.Fatal("Agent admitted to human adapter")
	}
	if err := f.Service.DB.Model(&service.Operation{}).Where("id = ?", "task-a").Updates(map[string]any{"status": "completed", "updated": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation&"+next, nil, nil); status != 410 {
		t.Fatalf("changed facts cursor %d", status)
	}
	var after service.Operation
	if err := f.Service.DB.First(&after, "id = ?", "task-c").Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != "failed" || !after.Updated.Equal(now) {
		t.Fatal("read mutated owner")
	}
	if err := f.Service.DB.Migrator().DropTable(&service.Operation{}); err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation", nil, nil); status != 500 {
		t.Fatalf("database failure became empty page: %d", status)
	}
}

func TestWorkspaceOperationTaskSubmissionBinding(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	op := service.Operation{ID: service.ID(), Key: "task-submission", Owner: alice.user.ID, ActorID: alice.user.ID, AgentID: "real-agent", SpaceID: alice.space.ID, Kind: "upload", Purpose: "inbox_staging", WorkspaceGrantID: "real-grant", Path: ".inbox-staging/private-name", Status: "awaiting_upload", Updated: time.Now().UTC()}
	if err := f.Service.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation", nil, nil); status != 503 {
		t.Fatalf("unbound staging status %d", status)
	}
	sub := service.WorkspaceSubmission{ID: service.ID(), Owner: alice.user.ID, AgentID: op.AgentID, GrantID: op.WorkspaceGrantID, SpaceID: op.SpaceID, UploadOperationID: op.ID, CreationKey: "fixture", RequestDigest: "fixture", State: "uploading", Revision: 1}
	if err := f.Service.DB.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	var page contract.Page[contract.TaskProjection]
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation", nil, &page); status != 200 || len(page.Items) != 1 {
		t.Fatal("bound staging unavailable")
	}
	if page.Items[0].Title != "Agent submission" || page.Items[0].UIState != "recoverable" || page.Items[0].AgentID == nil || *page.Items[0].AgentID != op.AgentID {
		t.Fatal("staging identity or title invalid")
	}
	if err := f.Service.DB.Model(&service.WorkspaceSubmission{}).Where("id = ?", sub.ID).Update("agent_id", "other-agent").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation", nil, nil); status != 503 {
		t.Fatalf("mismatched binding status %d", status)
	}
}

func TestWorkspaceOperationTaskScanBudget(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	rows := make([]service.Operation, 1001)
	for i := range rows {
		id := service.ID()
		rows[i] = service.Operation{ID: id, Key: id, Owner: alice.user.ID, SpaceID: alice.space.ID, Kind: "upload", Status: "running", Path: "budget", Updated: time.Now().UTC()}
	}
	if err := f.Service.DB.CreateInBatches(rows, 50).Error; err != nil {
		t.Fatal(err)
	}
	// A state filter may not turn truncated scanning into a false empty result.
	if status := alice.call(t, "GET", "/workspace/tasks?kind=operation&state=completed", nil, nil); status != 429 {
		t.Fatalf("scan budget status %d", status)
	}
}
