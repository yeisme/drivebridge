package service

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceRotationRequestDoesNotReplayCredential(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a := Agent{ID: "agent", Owner: "alice", TokenHash: Hash("fixture-original"), Expires: now.Add(time.Hour)}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	grant := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	invoke := func(key string, revision uint64, hash string) (WorkspaceRequest, bool, error) {
		var result WorkspaceRequest
		var issued bool
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			result, issued, err = rotateWorkspaceRequestOn(tx, "alice", "tenant", "alice", "grant", key, revision, hash, now)
			return err
		})
		return result, issued, err
	}
	first, issued, err := invoke("request", 1, Hash("fixture-new"))
	if err != nil || !issued || first.CredentialGeneration != 2 {
		t.Fatalf("first rotation failed: %v", err)
	}
	replay, issued, err := invoke("request", 1, Hash("must-not-be-used"))
	if err != nil || issued || replay.ID != first.ID || replay.CredentialGeneration != 2 {
		t.Fatal("replay reissued credential")
	}
	var stored Agent
	if err = db.First(&stored, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash != Hash("fixture-new") {
		t.Fatal("replay changed active hash")
	}
	if _, _, err = invoke("request", 2, Hash("other")); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed request accepted")
	}
	second, issued, err := invoke("explicit-next", 2, Hash("fixture-third"))
	if err != nil || !issued || second.CredentialGeneration != 3 {
		t.Fatal("explicit next rotation failed")
	}
	replay, issued, err = invoke("request", 1, "")
	if err != nil || issued || replay.CredentialGeneration != 2 {
		t.Fatal("old receipt lost original generation")
	}
}
