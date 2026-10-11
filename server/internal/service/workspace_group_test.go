package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceGroupNamesAndRevisionIsolation(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"human", "other"} {
		if err := db.Create(&User{ID: id, Issuer: "fixture", Subject: id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	now := time.Now().UTC()
	create := func(principal Principal, name string) (WorkspaceAgentGroup, error) {
		var g WorkspaceAgentGroup
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			g, err = s.createWorkspaceGroupOn(tx, principal, name, now)
			return err
		})
		return g, err
	}
	first, err := create(p, "  Research  ")
	if err != nil || first.Name != "Research" || first.Revision != 1 {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := create(p, "research"); err == nil || err.Error() != "name_conflict" {
		t.Fatal("normalized duplicate accepted")
	}
	for _, name := range []string{" ", strings.Repeat("字", 81), "bad\nname", string([]byte{0xff})} {
		if _, err := create(p, name); err == nil {
			t.Fatal("invalid name accepted")
		}
	}
	other := p
	other.UserID = "other"
	if _, err := create(other, "Research"); err != nil {
		t.Fatal(err)
	}
	tenant := p
	tenant.TenantID = "other-tenant"
	if _, err := create(tenant, "Research"); err != nil {
		t.Fatal(err)
	}
	second, err := create(p, "Second")
	if err != nil {
		t.Fatal(err)
	}
	rename := func(principal Principal, name string, revision uint64) error {
		return db.Transaction(func(tx *gorm.DB) error {
			_, err := s.renameWorkspaceGroupOn(tx, principal, first.ID, name, revision, now)
			return err
		})
	}
	if err := rename(other, "Foreign", 1); err == nil {
		t.Fatal("foreign owner renamed")
	}
	agent := p
	agent.Agent = &Agent{ID: "agent"}
	if err := rename(agent, "Agent", 1); err == nil {
		t.Fatal("Agent renamed group")
	}
	if err := rename(p, second.Name, 1); err == nil || err.Error() != "name_conflict" {
		t.Fatal("rename stole name")
	}
	abort := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.renameWorkspaceGroupOn(tx, p, first.ID, "Transient", 1, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if err := rename(p, "Final", 1); err != nil {
		t.Fatal(err)
	}
	if err := rename(p, "Stale", 1); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale revision renamed")
	}
	var stored WorkspaceAgentGroup
	if err := db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Final" || stored.Revision != 2 || stored.NormalizedName != "final" {
		t.Fatal("rename state incorrect")
	}
}
