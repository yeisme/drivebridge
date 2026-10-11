package service

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceGroupDeleteUnbindsWithoutChangingPinsOrAuthority(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Agent{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	group := WorkspaceAgentGroup{ID: "group", Owner: "human", TenantID: "tenant", Name: "Research", NormalizedName: "research", Revision: 2}
	org := WorkspaceAgentOrganization{ID: "org", Owner: "human", TenantID: "tenant", AgentID: "agent", GroupID: &group.ID, Pinned: true, Revision: 4}
	foreign := WorkspaceAgentOrganization{ID: "foreign", Owner: "other", TenantID: "tenant", AgentID: "other-agent", GroupID: &group.ID, Pinned: true, Revision: 7}
	agent := Agent{ID: "agent", Owner: "human", Name: "Original", Scopes: "read", TokenHash: "fixture", Expires: now.Add(time.Hour)}
	for _, row := range []any{&User{ID: "human", Issuer: "fixture", Subject: "human"}, &group, &org, &foreign, &agent} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	remove := func(principal Principal, revision uint64) error {
		return db.Transaction(func(tx *gorm.DB) error { return s.deleteWorkspaceGroupOn(tx, principal, group.ID, revision, now) })
	}
	ap := p
	ap.Agent = &agent
	if err := remove(ap, 2); err == nil {
		t.Fatal("Agent deleted group")
	}
	other := p
	other.TenantID = "other"
	if err := remove(other, 2); err == nil {
		t.Fatal("foreign tenant deleted group")
	}
	if err := remove(p, 1); err == nil {
		t.Fatal("stale revision deleted group")
	}
	abort := errors.New("fixture abort")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := s.deleteWorkspaceGroupOn(tx, p, group.ID, 2, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var retained WorkspaceAgentOrganization
	if err := db.First(&retained, "id = ?", org.ID).Error; err != nil || retained.GroupID == nil || retained.Revision != 4 {
		t.Fatal("rollback unbound organization")
	}
	if err := remove(p, 2); err != nil {
		t.Fatal(err)
	}
	retained = WorkspaceAgentOrganization{}
	if err := db.First(&retained, "id = ?", org.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retained.GroupID != nil || !retained.Pinned || retained.Revision != 5 {
		t.Fatal("unbind lost pin or revision")
	}
	var untouched WorkspaceAgentOrganization
	if err := db.First(&untouched, "id = ?", foreign.ID).Error; err != nil || untouched.GroupID == nil || untouched.Revision != 7 {
		t.Fatal("delete mutated foreign metadata")
	}
	var original Agent
	if err := db.First(&original, "id = ?", agent.ID).Error; err != nil || original.TokenHash != agent.TokenHash || original.Scopes != agent.Scopes || original.Revoked {
		t.Fatal("group deletion changed authority")
	}
	var count int64
	if err := db.Model(&WorkspaceAgentGroup{}).Where("id = ?", group.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("group not deleted")
	}
}
