package service

import "testing"

func TestWorkspaceSuggestionsIsolateAgentAndVersion(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	first := WorkspaceLabelSuggestion{ID: "a", Owner: "human", TenantID: "tenant", AgentID: "agent-a", GrantID: "grant-a", FileID: "file", VersionID: "v1", CreationKey: "key", RequestDigest: "digest", Labels: []string{"reference"}, State: "pending", Revision: 1}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.ID = "duplicate"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate suggestion accepted")
	}
	other := first
	other.ID = "b"
	other.AgentID = "agent-b"
	other.GrantID = "grant-b"
	other.Labels = []string{"review"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal("independent agent key collided")
	}
	version := first
	version.ID = "v2"
	version.VersionID = "v2"
	if err := db.Create(&version).Error; err != nil {
		t.Fatal("fixed version key collided")
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var stored WorkspaceLabelSuggestion
	if err := db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored.Labels) != 1 || stored.Labels[0] != "reference" || stored.State != "pending" {
		t.Fatal("another agent overwrote suggestion")
	}
	decision := WorkspaceSuggestionDecision{ID: "decision", SuggestionID: first.ID, Revision: 2, HumanID: "human", VersionID: "v1", Decision: "accept", AcceptedLabels: []string{"reference"}}
	if err := db.Create(&decision).Error; err != nil {
		t.Fatal(err)
	}
	duplicateDecision := decision
	duplicateDecision.ID = "duplicate-decision"
	if err := db.Create(&duplicateDecision).Error; err == nil {
		t.Fatal("duplicate decision revision accepted")
	}
}
