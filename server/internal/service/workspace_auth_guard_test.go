package service

import (
	"context"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWorkspaceIdentityCannotFallThroughLegacyAuthentication(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: "agent-fixture", Owner: "human", SpaceID: "space", Scopes: "read,write", TokenHash: Hash("fixture-only"), Expires: time.Now().Add(time.Hour)}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	request := httptest.NewRequest("GET", "/api/v1/files", nil)
	request.Header.Set("Authorization", "Bearer fixture-only")
	if _, err := s.authenticate(request); err != nil {
		t.Fatalf("legacy authentication broken: %v", err)
	}
	grant := WorkspaceGrant{ID: "grant-fixture", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: time.Now(), Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.authenticate(request); err == nil {
		t.Fatal("grant-bound credential inherited legacy broad scopes")
	}
	if err := rejectWorkspaceLegacyAccess(db, a); err == nil {
		t.Fatal("application guard allowed legacy fallback")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, action := range []string{"read", "write", "delete", "import", "export"} {
			if err := s.allowOn(tx, Principal{UserID: a.Owner, Agent: &a}, a.SpaceID, "reference.txt", action); err == nil {
				t.Fatalf("legacy application action allowed: %s", action)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// A malformed or revoked profile must not be reclassified as a legacy agent.
	if err := db.Model(&grant).Updates(map[string]any{"restriction_profile": "unknown", "revoked": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := rejectWorkspaceLegacyAccess(db, a); err == nil {
		t.Fatal("invalid profile fell back to legacy")
	}
}

func TestWorkspaceProfileMarkerRejectsMissingGrantAndStalePrincipal(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	agent := Agent{ID: "marked", Owner: "human", SpaceID: "space", Scopes: "read,write", TokenHash: Hash("fixture-marked"), Expires: time.Now().Add(time.Hour)}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatal(err)
	}
	stale := agent
	if err := db.Model(&agent).Update("restriction_profile", "personal_workspace_v1").Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	req := httptest.NewRequest("GET", "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer fixture-marked")
	if _, err := s.authenticate(req); err == nil {
		t.Fatal("marked identity without grant authenticated as legacy")
	}
	if err := s.allowOn(db, Principal{UserID: "human", Agent: &stale}, "space", "file.txt", "read"); err == nil {
		t.Fatal("stale principal bypassed persistent profile")
	}
	for _, tool := range []string{"capabilities", "spaces", "operation_get", "create_upload"} {
		if _, err := s.CallTool(t.Context(), Principal{UserID: "human", Agent: &stale}, tool, map[string]any{}); err == nil || err.Error() != "forbidden" {
			t.Fatalf("stale legacy dispatch reached %s: %v", tool, err)
		}
	}
	ctx := context.WithValue(t.Context(), principalKey{}, Principal{UserID: "human", Agent: &stale})
	if tools := s.workspaceToolFilter(ctx, ToolDefinitions()); len(tools) != 0 {
		t.Fatal("stale legacy discovery exposed tools")
	}
	if err := db.Model(&agent).Update("restriction_profile", "unknown").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.authenticate(req); err == nil {
		t.Fatal("unknown marker fell back to legacy")
	}
}

func TestWorkspaceLegacyGuardRejectsMissingIdentity(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	if err := rejectWorkspaceLegacyAccess(db, Agent{}); err == nil {
		t.Fatal("empty agent identity accepted")
	}
	if err := rejectWorkspaceLegacyAccess(db, Agent{ID: "legacy"}); err != nil {
		t.Fatalf("absent grant should retain legacy classification: %v", err)
	}
}

func TestWorkspaceLegacyGuardFailsClosedOnDatabaseError(t *testing.T) {
	db := workspaceTestDB(t)
	// No workspace table: missing schema is not an absent grant record.
	if err := rejectWorkspaceLegacyAccess(db, Agent{ID: "agent"}); err == nil {
		t.Fatal("database error allowed fallback")
	}
}
