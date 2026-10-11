package main

import (
	"os"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceDowngradePreflightCLI(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := service.Open(service.Config{State: dir, Enabled: true, Development: true})
	if err != nil {
		t.Fatal(err)
	}
	g := service.WorkspaceGrant{ID: "fixture", AgentID: "fixture", Owner: "human", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", Revision: 1, CredentialGeneration: 1}
	if err := s.DB.Create(&g).Error; err != nil {
		s.Close()
		t.Fatal(err)
	}
	s.Close()
	out, err := capture(t, func() error { return runCLI("metadata", "check-workspace-downgrade", "--state", dir, "--json") })
	if err == nil || err.Error() != "workspace_downgrade_blocked" || strings.Contains(out, "workspace_grants_retired") {
		t.Fatalf("CLI failed to block: %v", err)
	}
	s, err = service.Open(service.Config{State: dir, Enabled: true, Development: true})
	if err != nil {
		t.Fatal(err)
	}
	var got service.WorkspaceGrant
	if err := s.DB.First(&got, "id = ?", g.ID).Error; err != nil || got.Revoked || got.Revision != 1 {
		s.Close()
		t.Fatal("preflight changed grant")
	}
	if err := s.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		s.Close()
		t.Fatal(err)
	}
	s.Close()
	out, err = capture(t, func() error { return runCLI("metadata", "check-workspace-downgrade", "--state", dir, "--json") })
	if err != nil || !strings.Contains(out, `"workspace_grants_retired":true`) {
		t.Fatalf("retired preflight failed: %v %s", err, out)
	}
}
