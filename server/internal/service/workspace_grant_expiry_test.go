package service

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceExpiryAndRevocationCompeteOnSameRevision(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	input := contract.ExpiryInput{ExpiryMode: contract.Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: contract.Field[int64]{Present: true, Value: 3600}}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- db.Transaction(func(tx *gorm.DB) error {
			_, err := changeWorkspaceExpiryOn(tx, "alice", "tenant", "alice", "grant", "expiry", 1, input, now)
			return err
		})
	}()
	go func() {
		<-start
		results <- db.Transaction(func(tx *gorm.DB) error {
			_, err := changeWorkspaceGrantRequestOn(tx, "alice", "tenant", "alice", "grant", "revoke", 1, "revoke", now)
			return err
		})
	}()
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if err.Error() == "revision_conflict" {
			conflict++
		} else {
			t.Fatalf("unexpected failure: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	var stored WorkspaceGrant
	if err := db.First(&stored, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 2 {
		t.Fatal("revision advanced twice")
	}
	if stored.Revoked {
		if stored.ExpiresAt != nil || stored.ExpiryMode != "never" {
			t.Fatal("losing expiry mutated grant")
		}
	} else if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatal("winning expiry was not persisted")
	}
	var receipts int64
	if err := db.Model(&WorkspaceRequest{}).Count(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if receipts != 1 {
		t.Fatal("loser persisted an idempotency receipt")
	}
}

func TestWorkspaceExpiryChangeAndReplay(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	grant := WorkspaceGrant{ID: "grant", AgentID: "agent", Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never", IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	duration := contract.ExpiryInput{ExpiryMode: contract.Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: contract.Field[int64]{Present: true, Value: 3600}}
	invoke := func(key string, revision uint64, input contract.ExpiryInput, at time.Time) (WorkspaceRequest, error) {
		var receipt WorkspaceRequest
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			receipt, err = changeWorkspaceExpiryOn(tx, "alice", "tenant", "alice", "grant", key, revision, input, at)
			return err
		})
		return receipt, err
	}
	first, err := invoke("duration", 1, duration, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := invoke("duration", 1, duration, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != replay.ID {
		t.Fatal("replay replaced receipt")
	}
	var stored WorkspaceGrant
	if err = db.First(&stored, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(now.Add(time.Hour)) || stored.ExpiryChangedAt == nil || !stored.ExpiryChangedAt.Equal(now) {
		t.Fatal("replay extended expiry")
	}
	never := contract.ExpiryInput{ExpiryMode: contract.Field[string]{Present: true, Value: "never"}}
	if _, err = invoke("duration", 1, never, now); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed expiry")
	}
	if _, err = invoke("expired", 2, never, now.Add(time.Hour)); err == nil {
		t.Fatal("expired grant revived")
	}
	if _, err = invoke("never", 2, never, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored = WorkspaceGrant{}
	if err = db.First(&stored, "id = ?", "grant").Error; err != nil {
		t.Fatal(err)
	}
	if stored.ExpiresAt != nil || stored.ExpiryMode != "never" || stored.Revision != 3 {
		t.Fatal("never transition failed")
	}
}
