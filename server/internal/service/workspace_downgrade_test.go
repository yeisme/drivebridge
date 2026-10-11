package service

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// The old-column projection models the legacy credential checks, not an old
// binary execution. New credentials must remain unusable through those fields.
func TestWorkspaceCredentialLifecycleKeepsLegacyColumnsClosed(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	var grant WorkspaceGrant
	var agent Agent
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		grant, agent, _, err = createWorkspaceGrantOn(tx, "human", "tenant", "human", "create", Hash("fixture-initial"), contract.CreateGrantRequest{Name: "Restricted", Protocol: "http", SpaceID: "space", ConnectionID: "connection", Prefix: "refs", Actions: []string{"files.read", "inbox.submit"}}, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertClosed := func(hash string, at time.Time) {
		t.Helper()
		var old legacyAgentSchema
		if err := db.Where("id = ?", agent.ID).First(&old).Error; err != nil {
			t.Fatal(err)
		}
		if old.TokenHash != hash {
			t.Fatal("legacy projection did not observe current hash")
		}
		if old.Scopes != "" || at.Before(old.Expires) {
			t.Fatal("new credential became usable by legacy authorization")
		}
	}
	assertClosed(Hash("fixture-initial"), now)
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		grant, err = rotateWorkspaceCredentialOn(tx, "human", "tenant", grant.ID, grant.Revision, Hash("fixture-rotated"), now.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertClosed(Hash("fixture-rotated"), now.Add(time.Minute))
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := changeWorkspaceExpiryOn(tx, "human", "tenant", "human", grant.ID, "duration", grant.Revision, contract.ExpiryInput{ExpiryMode: contract.Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: contract.Field[int64]{Present: true, Value: 31536000}}, now.Add(2*time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	assertClosed(Hash("fixture-rotated"), now.Add(2*time.Minute))
	var final WorkspaceGrant
	if err := db.First(&final, "id = ?", grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if final.ExpiresAt == nil || !final.ExpiresAt.After(now.Add(24*time.Hour)) || final.CredentialGeneration != 2 {
		t.Fatal("new-profile lifecycle did not remain independently valid")
	}
}
