package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// legacyAgentSchema reproduces only the pre-profile columns needed by this
// migration fixture; it is not another production persistence model.
type legacyAgentSchema struct {
	ID           string `gorm:"primaryKey"`
	Owner        string
	Name         string
	SpaceID      string
	Prefix       string
	Scopes       string
	ConnectionID string
	TokenHash    string `gorm:"uniqueIndex"`
	Expires      time.Time
	Revoked      bool
}

func (legacyAgentSchema) TableName() string { return "agents" }

func TestWorkspaceProfileColumnUpgradePreservesLegacyIdentity(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&legacyAgentSchema{}); err != nil {
		t.Fatal(err)
	}
	expiry := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	old := legacyAgentSchema{ID: "old", Owner: "human", Name: "Legacy", SpaceID: "space", Scopes: "read", TokenHash: Hash("fixture-legacy"), Expires: expiry}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.AutoMigrate(&Agent{}); err != nil {
			t.Fatal(err)
		}
	}
	var current Agent
	if err := db.First(&current, "id = ?", old.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.RestrictionProfile != "" || current.TokenHash != old.TokenHash || !current.Expires.Equal(expiry) || current.Scopes != old.Scopes {
		t.Fatal("legacy upgrade changed identity or authority")
	}
	current.RestrictionProfile = "personal_workspace_v1"
	wire, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), "restriction_profile") || strings.Contains(string(wire), "personal_workspace_v1") {
		t.Fatal("internal marker changed legacy public response")
	}
}
