package service

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceOrganizationPatchPreservesPinsAndAuthority(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	agent := Agent{ID: "agent", Owner: "human", SpaceID: "space", Scopes: "read", TokenHash: "fixture"}
	group := WorkspaceAgentGroup{ID: "group", Owner: "human", TenantID: "tenant", Name: "Group", NormalizedName: "group", Revision: 1}
	for _, row := range []any{&User{ID: "human", Issuer: "fixture", Subject: "human"}, &Space{ID: "space", Owner: "human", TenantID: "tenant"}, &agent, &group} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	now := time.Now().UTC()
	patch := func(input contract.PatchOrganizationRequest) (WorkspaceAgentOrganization, error) {
		var org WorkspaceAgentOrganization
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			org, err = s.patchWorkspaceOrganizationOn(tx, p, agent.ID, input, now)
			return err
		})
		return org, err
	}
	for _, principal := range []Principal{{UserID: "human", TenantID: "other"}, {UserID: "human", TenantID: "tenant", Agent: &agent}, {UserID: "human", TenantID: "tenant", ScopeSpace: "other"}} {
		if err := db.Transaction(func(tx *gorm.DB) error {
			_, err := s.patchWorkspaceOrganizationOn(tx, principal, agent.ID, contract.PatchOrganizationRequest{Pinned: contract.Field[bool]{Present: true, Value: true}}, now)
			return err
		}); err == nil {
			t.Fatal("foreign context or Agent changed organization")
		}
	}
	first := contract.PatchOrganizationRequest{GroupID: contract.Field[string]{Present: true, Value: group.ID}, Pinned: contract.Field[bool]{Present: true, Value: true}}
	org, err := patch(first)
	if err != nil || org.Revision != 1 || !org.Pinned || org.GroupID == nil {
		t.Fatalf("initial organization failed: %v", err)
	}
	if _, err := patch(first); err == nil {
		t.Fatal("stale creation overwrote organization")
	}
	org, err = patch(contract.PatchOrganizationRequest{ExpectedRevision: 1, GroupID: contract.Field[string]{Present: true, Null: true}})
	if err != nil || org.GroupID != nil || !org.Pinned || org.Revision != 2 {
		t.Fatal("unbind lost pinned state")
	}
	org, err = patch(contract.PatchOrganizationRequest{ExpectedRevision: 2, GroupID: first.GroupID})
	if err != nil || org.GroupID == nil || !org.Pinned {
		t.Fatal("rebind lost pin")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return s.deleteWorkspaceGroupOn(tx, p, group.ID, 1, now) }); err != nil {
		t.Fatal(err)
	}
	org = WorkspaceAgentOrganization{}
	if err := db.First(&org, "agent_id = ?", agent.ID).Error; err != nil || org.GroupID != nil || !org.Pinned || org.Revision != 4 {
		t.Fatal("delete did not unbind organization")
	}
	if _, err := patch(contract.PatchOrganizationRequest{ExpectedRevision: 4, GroupID: first.GroupID}); err == nil || err.Error() != "not_found" {
		t.Fatal("unproven missing group disclosed deletion")
	}
	receipt := WorkspaceRequest{ID: "foreign-delete", Owner: "other", TenantID: p.TenantID, ActorID: "other", Action: "group.delete", RequestKey: "delete", ResourceID: group.ID, RequestDigest: "fixture", ResultRevision: 1}
	if err := db.Create(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := patch(contract.PatchOrganizationRequest{ExpectedRevision: 4, GroupID: first.GroupID}); err == nil || err.Error() != "not_found" {
		t.Fatal("foreign receipt leaked deleted group")
	}
	receipt.ID = "own-delete"
	receipt.Owner = p.UserID
	receipt.ActorID = p.UserID
	if err := db.Create(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := patch(contract.PatchOrganizationRequest{ExpectedRevision: 4, GroupID: first.GroupID}); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("owned deleted group did not request revision refresh")
	}
	org, err = patch(contract.PatchOrganizationRequest{ExpectedRevision: 4, Pinned: contract.Field[bool]{Present: true, Value: false}})
	if err != nil || org.Pinned || org.GroupID != nil || org.Revision != 5 {
		t.Fatal("explicit false ignored")
	}
	var unchanged Agent
	if err := db.First(&unchanged, "id = ?", agent.ID).Error; err != nil || unchanged.TokenHash != agent.TokenHash || unchanged.Scopes != agent.Scopes {
		t.Fatal("organization changed authority")
	}
}
