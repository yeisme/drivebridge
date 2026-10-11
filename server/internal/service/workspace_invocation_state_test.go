package service

import (
	"errors"
	"testing"
	"time"
)

func TestWorkspaceInvocationConservativeOutcomeAndMigration(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	for _, test := range []struct {
		name         string
		err          error
		result, code string
	}{
		{"returned", nil, "unknown", ""},
		{"denied", errors.New("forbidden"), "denied", "forbidden"},
		{"write_error", errors.New("database_unavailable"), "unknown", "database_unavailable"},
		{"private_error", errors.New("private_user_supplied_value"), "unknown", "invocation_error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := WorkspaceInvocation{ID: ID(), Owner: "owner", TenantID: "tenant", SpaceID: "space", AgentID: "agent", GrantID: "grant", Protocol: "http", Action: "files.list", RequestID: ID(), Result: "unknown", StartedAt: time.Now().UTC()}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			// Additive migration may be repeated while preserving an unfinished call.
			if err := migrateWorkspaceModels(db); err != nil {
				t.Fatal(err)
			}
			var unfinished WorkspaceInvocation
			if err := db.First(&unfinished, "id = ?", row.ID).Error; err != nil || unfinished.FinishedAt != nil || unfinished.Result != "unknown" {
				t.Fatal("migration fabricated completion")
			}
			s.finishWorkspaceInvocation(&row, test.err)
			var got WorkspaceInvocation
			if err := db.First(&got, "id = ?", row.ID).Error; err != nil || got.Result != test.result || got.ErrorCode != test.code || got.FinishedAt == nil {
				t.Fatal("unsafe outcome classification")
			}
			finished := *got.FinishedAt
			s.finishWorkspaceInvocation(&row, errors.New("forbidden"))
			if err := db.First(&got, "id = ?", row.ID).Error; err != nil || got.Result != test.result || !got.FinishedAt.Equal(finished) {
				t.Fatal("duplicate finish rewrote observation")
			}
		})
	}
	if s.startWorkspaceInvocation(Principal{UserID: "human"}, "http", "files.list") != nil {
		t.Fatal("human call recorded as Agent")
	}
}
