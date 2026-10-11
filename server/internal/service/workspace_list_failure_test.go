package service

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceGrantListReturnsAuthorizationStorageFailure(t *testing.T) {
	s, _, p := blobRig(t)
	if err := s.BootstrapGovernance(p.UserID); err != nil {
		t.Fatal(err)
	}
	p.TenantID = s.Instance
	var space Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: ID(), Owner: p.UserID, SpaceID: space.ID, ConnectionID: space.ConnectionID, TokenHash: "fixture", RestrictionProfile: "personal_workspace_v1"}
	g := WorkspaceGrant{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, AgentID: a.ID, SpaceID: space.ID, ConnectionID: space.ConnectionID, Actions: "files.read", Protocol: "http", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &g} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest("GET", "/api/v1/agent-grants", nil)
	good := httptest.NewRecorder()
	if err := s.workspaceGrantList(good, req, p); err != nil || good.Body.Len() == 0 {
		t.Fatalf("baseline list failed: %v", err)
	}
	const callback = "fixture_list_authorization_failure"
	injected := false
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		// Candidate enumeration uses *[]Space; fail only per-item authorization.
		if _, ok := tx.Statement.Dest.(*Space); ok {
			injected = true
			tx.AddError(errors.New("fixture storage failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer s.DB.Callback().Query().Remove(callback)
	response := httptest.NewRecorder()
	err := s.workspaceGrantList(response, req, p)
	if !injected {
		t.Fatal("authorization fault not exercised")
	}
	if err == nil || err.Error() != "database_unavailable" {
		t.Fatalf("list hid database failure: %v", err)
	}
	if response.Body.Len() != 0 {
		t.Fatal("list wrote partial success before authorization completed")
	}
	writeError(response, err)
	if response.Code != 500 {
		t.Fatalf("failure response status %d", response.Code)
	}
}
