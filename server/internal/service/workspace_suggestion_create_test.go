package service

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceSuggestionDedicatedActionAndReplay(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}, &File{}, &Version{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, row := range []any{
		&User{ID: "human", Issuer: "fixture", Subject: "human"},
		&Space{ID: "space", Owner: "human", TenantID: "tenant", ConnectionID: "connection"},
		&File{ID: "file", Owner: "human", SpaceID: "space", Path: "refs/file.txt", VersionID: "version"},
		&Version{ID: "version", Owner: "human", FileID: "file", ConnectionID: "connection"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "connection", Prefix: "refs", TokenHash: Hash("fixture"), RestrictionProfile: "personal_workspace_v1"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	grant := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "labels.suggest", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	p := Principal{UserID: a.Owner, TenantID: "tenant", Agent: &a}
	input := contract.LabelSuggestionRequest{VersionID: "version", Labels: []string{"reference"}}
	invoke := func(in contract.LabelSuggestionRequest) (WorkspaceLabelSuggestion, bool, error) {
		var out WorkspaceLabelSuggestion
		var created bool
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			out, created, err = s.createWorkspaceSuggestionOn(tx, p, "file", "key", in, now)
			return err
		})
		return out, created, err
	}
	first, created, err := invoke(input)
	if err != nil || !created {
		t.Fatalf("suggestion failed: %v", err)
	}
	replay, created, err := invoke(input)
	if err != nil || created || replay.ID != first.ID {
		t.Fatal("suggestion replay changed identity")
	}
	changed := input
	changed.Labels = []string{"different"}
	if _, _, err = invoke(changed); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed suggestion reused key")
	}
	otherAgent := a
	otherAgent.ID = "agent-b"
	otherAgent.TokenHash = Hash("fixture-b")
	if err = db.Create(&otherAgent).Error; err != nil {
		t.Fatal(err)
	}
	otherGrant := grant
	otherGrant.ID = "grant-b"
	otherGrant.AgentID = otherAgent.ID
	if err = db.Create(&otherGrant).Error; err != nil {
		t.Fatal(err)
	}
	otherPrincipal := p
	otherPrincipal.Agent = &otherAgent
	if err = db.Transaction(func(tx *gorm.DB) error {
		other, created, err := s.createWorkspaceSuggestionOn(tx, otherPrincipal, "file", "key", changed, now)
		if err != nil {
			return err
		}
		if !created || other.ID == first.ID || other.AgentID != otherAgent.ID {
			t.Fatal("second agent overwrote first proposal")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&grant).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err = invoke(input); err == nil {
		t.Fatal("revoked grant replay bypassed authorization")
	}
	var count int64
	if err = db.Model(&WorkspaceLabelSuggestion{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("duplicate suggestion persisted")
	}
	var stored File
	if err = db.First(&stored, "id = ?", "file").Error; err != nil {
		t.Fatal(err)
	}
	if stored.VersionID != "version" {
		t.Fatal("suggestion changed file version")
	}
}
