package service

import (
	"context"
	"testing"
	"time"
)

func TestWorkspaceOperationRequiresCurrentGrantAndSubmissionBinding(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Agent{}, &Space{}, &Operation{}, &Part{}, &ServiceIdentityBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "conn", Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture-hash"}
	grant := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, RestrictionProfile: a.RestrictionProfile, Actions: "inbox.submit", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	op := Operation{ID: "op", Owner: a.Owner, ActorID: a.Owner, AgentID: a.ID, Key: "key", Kind: "upload", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Path: "refs/staging/file", Purpose: "inbox_staging", WorkspaceGrantID: grant.ID, VersionID: "version", Status: "awaiting_upload"}
	submission := WorkspaceSubmission{ID: "submission", Owner: a.Owner, TenantID: grant.TenantID, GrantID: grant.ID, AgentID: a.ID, SpaceID: a.SpaceID, UploadOperationID: op.ID, CreationKey: "key", RequestDigest: "digest", State: "uploading", Revision: 1}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &Space{ID: a.SpaceID, Owner: a.Owner, TenantID: grant.TenantID, ConnectionID: a.ConnectionID}, &a, &grant, &op, &submission} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: a.Owner, TenantID: grant.TenantID, Agent: &a}
	if got, sub, err := s.workspaceOperationOn(db, p, op.ID, now); err != nil || got.ID != op.ID || sub.ID != submission.ID {
		t.Fatalf("valid operation rejected: %v", err)
	}
	view, err := s.CallTool(t.Context(), p, "operation_get", map[string]any{"operation_id": op.ID})
	if err != nil {
		t.Fatal(err)
	}
	recovery, ok := view.(map[string]any)
	if !ok || recovery["operation"] == nil {
		t.Fatal("missing recovery projection")
	}
	for _, name := range []string{"create_upload", "archive", "files", "agents", "cancel"} {
		if _, err := s.CallTool(t.Context(), p, name, map[string]any{}); err == nil {
			t.Fatalf("workspace identity reached %s", name)
		}
	}
	for _, tc := range []struct {
		field        string
		bad, restore any
	}{
		{"purpose", "", op.Purpose}, {"workspace_grant_id", "other", op.WorkspaceGrantID}, {"agent_id", "other", op.AgentID},
		{"actor_id", "other", op.ActorID}, {"connection_id", "other", op.ConnectionID}, {"path", "refs-other/file", op.Path},
		{"kind", "copy", op.Kind}, {"expected_version", "old", ""}, {"batch_id", "batch", ""},
	} {
		t.Run(tc.field, func(t *testing.T) {
			if err := db.Model(&Operation{}).Where("id = ?", op.ID).Update(tc.field, tc.bad).Error; err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.workspaceOperationOn(db, p, op.ID, now); err == nil {
				t.Fatal("altered binding accepted")
			}
			if err := db.Model(&Operation{}).Where("id = ?", op.ID).Update(tc.field, tc.restore).Error; err != nil {
				t.Fatal(err)
			}
		})
	}
	other := p
	other.TenantID = "foreign"
	if _, _, err := s.workspaceOperationOn(db, other, op.ID, now); err == nil {
		t.Fatal("foreign tenant accepted")
	}
	stale := a
	stale.TokenHash = "previous"
	other = p
	other.Agent = &stale
	if _, _, err := s.workspaceOperationOn(db, other, op.ID, now); err == nil {
		t.Fatal("stale credential accepted")
	}
	caps, err := s.workspaceCapabilities(p)
	if err != nil {
		t.Fatal(err)
	}
	actions := caps["http_actions"].([]map[string]any)
	if len(actions) != 1 {
		t.Fatal("unexpected authorized HTTP actions")
	}
	list := actions[0]["list"].(map[string]any)
	if list["path"] != "/api/v1/inbox-submissions" || list["visibility"] != "own_grant" {
		t.Fatal("missing scoped inbox discovery")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("actions", "labels.suggest").Error; err != nil {
		t.Fatal(err)
	}
	caps, err = s.workspaceCapabilities(p)
	if err != nil {
		t.Fatal(err)
	}
	actions = caps["http_actions"].([]map[string]any)
	list = actions[0]["list"].(map[string]any)
	if list["visibility"] != "own_agent" || list["required_query_fields"].([]string)[0] != "version_id" {
		t.Fatal("suggestion recovery lost fixed-version requirement")
	}
	if len(caps["tools"].([]string)) != 1 {
		t.Fatal("suggest-only discovery exposed transfer tools")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("actions", "inbox.submit").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(t.Context(), principalKey{}, p)
	if tools := s.workspaceToolFilter(ctx, ToolDefinitions()); len(tools) != 3 {
		t.Fatal("submit discovery missing recovery tools")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("actions", "files.read").Error; err != nil {
		t.Fatal(err)
	}
	readTools := s.workspaceToolFilter(ctx, ToolDefinitions())
	if len(readTools) != 4 {
		t.Fatal("read discovery missing metadata tool")
	}
	for _, tool := range readTools {
		if tool.Name != "stat" && tool.Name != "capabilities" && tool.Name != "files" && tool.Name != "search" {
			t.Fatal("narrowed grant retained transfer tools")
		}
	}
	if _, err := s.CallTool(t.Context(), p, "operation_get", map[string]any{"operation_id": op.ID}); err == nil {
		t.Fatal("narrowed grant retained recovery authority")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("actions", "inbox.submit").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.workspaceOperationOn(db, p, op.ID, now); err == nil {
		t.Fatal("revoked grant recovered operation")
	}
	if _, err := s.CallTool(t.Context(), p, "operation_get", map[string]any{"operation_id": op.ID}); err == nil {
		t.Fatal("MCP application replay ignored revocation")
	}
	if tools := s.workspaceToolFilter(ctx, ToolDefinitions()); len(tools) != 0 {
		t.Fatal("revoked discovery retained tools")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&submission).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.workspaceOperationOn(db, p, op.ID, now); err == nil {
		t.Fatal("orphan operation accepted")
	}
}
