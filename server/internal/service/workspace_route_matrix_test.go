package service

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceAuthenticationRouteMatrix(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &User{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "conn", Prefix: "refs", Scopes: "read,write,delete", TokenHash: Hash("fixture-routes"), Expires: time.Now().Add(time.Hour), RestrictionProfile: "personal_workspace_v1"}
	g := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read,labels.suggest,inbox.submit", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &a, &g} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	allowed := []string{
		"GET /api/v1/files", "GET /api/v1/files/file", "GET /api/v1/files/file/content?version=v",
		"POST /api/v1/files/file/label-suggestions", "GET /api/v1/files/file/label-suggestions?version_id=v",
		"POST /api/v1/inbox-submissions", "POST /api/v1/inbox-submissions/lookup", "GET /api/v1/inbox-submissions", "GET /api/v1/inbox-submissions/submission",
		"PUT /api/v1/operations/op/parts/0", "GET /api/v1/operations/op", "POST /api/v1/operations/op/complete",
		"POST /mcp", "GET /mcp", "DELETE /mcp",
	}
	denied := []string{
		"POST /api/v1/uploads", "GET /api/v1/files/file/versions", "GET /api/v1/references?ref=fixture",
		"POST /api/v1/files/file/download-link", "PATCH /api/v1/files/file", "DELETE /api/v1/files/file",
		"POST /api/v1/files/file/restore", "POST /api/v1/files/file/archive", "PUT /api/v1/files/file/labels",
		"POST /api/v1/files/file/label-suggestions/suggestion/decision", "POST /api/v1/inbox-submissions/submission/decision", "POST /api/v1/inbox-submissions/submission/archive",
		"GET /api/v1/operations", "POST /api/v1/operations/op/resume", "POST /api/v1/operations/op/cancel",
		"POST /api/v1/agents", "GET /api/v1/agent-grants", "POST /api/v1/agent-grants", "GET /api/v1/agent-groups", "GET /api/v1/connections", "POST /api/v1/shares",
		"HEAD /api/v1/files/file/content", "PUT /mcp", "POST /mcp/", "POST /api/v1/operations/op/complete/extra",
	}
	check := func(route string, want bool) {
		t.Helper()
		method, path, _ := strings.Cut(route, " ")
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer fixture-routes")
		principal, err := s.authenticate(req)
		if (err == nil) != want {
			t.Fatalf("%s admitted=%v want=%v", route, err == nil, want)
		}
		if want && (principal.Agent == nil || principal.Agent.ID != a.ID || principal.TenantID != g.TenantID || principal.ScopeSpace != g.SpaceID) {
			t.Fatal("authentication lost restricted context")
		}
	}
	for _, route := range allowed {
		t.Run(route, func(t *testing.T) { check(route, true) })
	}
	for _, route := range denied {
		t.Run(route, func(t *testing.T) { check(route, false) })
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	for _, route := range allowed {
		check(route, false)
	}
}
