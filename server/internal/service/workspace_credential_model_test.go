package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkspaceCredentialHistoryConstraints(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	first := WorkspaceCredentialHistory{ID: "first", GrantID: "grant", Generation: 1, TokenHash: Hash("synthetic-first")}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.ID = "duplicate"
	duplicate.TokenHash = Hash("synthetic-other")
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate generation accepted")
	}
	reused := first
	reused.ID = "reused"
	reused.Generation = 2
	if err := db.Create(&reused).Error; err == nil {
		t.Fatal("historical hash reused")
	}
	next := WorkspaceCredentialHistory{ID: "next", GrantID: "grant", Generation: 2, TokenHash: Hash("synthetic-next")}
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), first.TokenHash) || strings.Contains(string(wire), "TokenHash") {
		t.Fatal("hash leaked into serialized history")
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&WorkspaceCredentialHistory{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("repeated migration changed credential history")
	}
}
