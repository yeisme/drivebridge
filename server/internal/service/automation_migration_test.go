package service

import "testing"

// This models only the former unique index, on a disposable test database.
type legacyAutomationIntentIdentity struct {
	ID           string `gorm:"primaryKey"`
	RuleID       string
	EventID      string `gorm:"uniqueIndex:automation_intent_identity"`
	RuleRevision uint64 `gorm:"uniqueIndex:automation_intent_identity"`
	ActorID      string `gorm:"uniqueIndex:automation_intent_identity"`
	Scope        string `gorm:"uniqueIndex:automation_intent_identity"`
}

func (legacyAutomationIntentIdentity) TableName() string { return "automation_intents" }
func TestAutomationSafetyIdentityMigrationPreservesHistory(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&legacyAutomationIntentIdentity{}); err != nil {
		t.Fatal(err)
	}
	original := legacyAutomationIntentIdentity{ID: "original", RuleID: "first", EventID: "event", RuleRevision: 1, ActorID: "actor", Scope: "auto"}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := migrateAutomationIntentIdentity(db); err != nil {
			t.Fatal(err)
		}
	}
	if db.Migrator().HasIndex(&AutomationIntent{}, "automation_intent_identity") || !db.Migrator().HasIndex(&AutomationIntent{}, "automation_intent_rule_identity") {
		t.Fatal("migration retained over-narrow identity")
	}
	var kept AutomationIntent
	if err := db.First(&kept, "id = ?", original.ID).Error; err != nil || kept.RuleID != original.RuleID || kept.EventID != original.EventID || kept.ActorID != original.ActorID || kept.Scope != original.Scope || kept.RuleRevision != original.RuleRevision {
		t.Fatal("migration changed historical identity")
	}
	other := kept
	other.ID = "other-rule-intent"
	other.RuleID = "second"
	if err := db.Create(&other).Error; err != nil {
		t.Fatal("distinct rule still collides", err)
	}
	duplicate := kept
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("same rule replay lost uniqueness")
	}
	var count int64
	if err := db.Model(&AutomationIntent{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("migration/replay changed history")
	}
}
