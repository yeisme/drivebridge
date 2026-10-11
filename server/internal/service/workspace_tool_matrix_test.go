package service

import (
	"context"
	"testing"
	"time"
)

func TestWorkspaceRegisteredToolSetDeniesLegacyDispatch(t *testing.T) {
	s, _, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture-tool-matrix"}
	g := WorkspaceGrant{ID: ID(), AgentID: a.ID, Owner: a.Owner, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read,labels.suggest,inbox.submit", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &g} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	if _, err := s.CallTool(t.Context(), p, "capabilities", map[string]any{}); err != nil {
		t.Fatalf("positive control failed: %v", err)
	}
	allowed := map[string]bool{"capabilities": true, "stat": true, "files": true, "search": true, "operation_get": true, "complete_upload": true}
	ctx := context.WithValue(t.Context(), principalKey{}, p)
	listed := s.workspaceToolFilter(ctx, ToolDefinitions())
	if len(listed) != len(allowed) {
		t.Fatalf("unexpected discovery count: %d", len(listed))
	}
	seen := map[string]bool{}
	for _, tool := range listed {
		if !allowed[tool.Name] || seen[tool.Name] {
			t.Fatalf("unexpected or duplicate discovery: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
	denied := 0
	for _, tool := range ToolDefinitions() {
		if allowed[tool.Name] {
			continue
		}
		denied++
		t.Run(tool.Name, func(t *testing.T) {
			if _, err := s.CallTool(t.Context(), p, tool.Name, map[string]any{}); err == nil || err.Error() != "forbidden" {
				t.Fatalf("legacy tool reached dispatch: %v", err)
			}
		})
	}
	if denied == 0 {
		t.Fatal("no legacy tools checked")
	}
	if _, err := s.CallTool(t.Context(), p, "unregistered_future_tool", map[string]any{}); err == nil || err.Error() != "forbidden" {
		t.Fatal("unknown tool was not denied by profile")
	}
	t.Logf("checked %d registered legacy tools and %d dedicated discovery tools", denied, len(listed))
	if err := s.DB.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if tools := s.workspaceToolFilter(ctx, ToolDefinitions()); len(tools) != 0 {
		t.Fatal("revoked discovery retained tools")
	}
	if _, err := s.CallTool(t.Context(), p, "capabilities", map[string]any{}); err == nil {
		t.Fatal("revoked positive control still works")
	}
}
