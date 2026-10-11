package service

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWorkspaceRotationAtomicityAndExpiry(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&Agent{}, &UserKey{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Hour)
	oldHash, newHash := Hash("synthetic-old"), Hash("synthetic-new")
	agent := Agent{ID: "agent", Owner: "alice", TokenHash: oldHash, Expires: deadline, Scopes: "read,write"}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatal(err)
	}
	grant := WorkspaceGrant{ID: "grant", AgentID: agent.ID, Owner: "alice", TenantID: "tenant", SpaceID: "space", ConnectionID: "connection", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", ExpiryMode: "duration", ExpiresAt: &deadline, IssuedAt: now, Revision: 1, CredentialGeneration: 1}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	abort := errors.New("fixture abort")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 1, newHash, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var got Agent
	if err := db.First(&got, "id = ?", agent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.TokenHash != oldHash || got.RestrictionProfile != "" || got.Scopes != "read,write" || !got.Expires.Equal(deadline) {
		t.Fatal("rollback changed credential")
	}
	var histories int64
	if err := db.Model(&WorkspaceCredentialHistory{}).Count(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if histories != 0 {
		t.Fatal("rollback retained history")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		updated, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 1, newHash, now)
		if err != nil {
			return err
		}
		if updated.Revision != 2 || updated.CredentialGeneration != 2 || !updated.ExpiresAt.Equal(deadline) || updated.ExpiryChangedAt != nil {
			t.Fatal("rotation changed expiry or identity")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got = Agent{}
	if err := db.First(&got, "id = ?", agent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.TokenHash != newHash || got.RestrictionProfile != "personal_workspace_v1" || got.Scopes != "" || got.Expires.After(now) {
		t.Fatal("new hash not active")
	}
	var previous WorkspaceCredentialHistory
	if err := db.Where("grant_id = ? AND generation = ?", "grant", 1).First(&previous).Error; err != nil {
		t.Fatal(err)
	}
	if previous.TokenHash != oldHash || previous.RevokedAt == nil {
		t.Fatal("old generation not retired")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 2, oldHash, now)
		return err
	}); err == nil {
		t.Fatal("historical credential reused")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 1, Hash("another"), now)
		return err
	}); err == nil {
		t.Fatal("stale revision rotated")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 2, Hash("expired-attempt"), deadline)
		return err
	}); err == nil {
		t.Fatal("expired grant rotated")
	}
	if err := db.Model(&grant).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		updated, err := rotateWorkspaceCredentialOn(tx, "alice", "tenant", "grant", 2, Hash("synthetic-third"), now)
		if err != nil {
			return err
		}
		if !updated.Disabled || updated.CredentialGeneration != 3 || !updated.ExpiresAt.Equal(deadline) {
			t.Fatal("disabled rotation changed lifetime or state")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
