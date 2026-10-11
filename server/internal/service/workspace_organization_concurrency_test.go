package service

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Concurrent callers share the supported single-connection SQLite fixture.
// This proves local serialization, not PostgreSQL or multi-instance locking.
func TestWorkspaceConcurrentGroupDeleteAndAssignment(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{&User{ID: "human", Issuer: "fixture", Subject: "human"}, &Space{ID: "space", Owner: "human", TenantID: "tenant"}, &Agent{ID: "agent", Owner: "human", SpaceID: "space", TokenHash: "fixture"}} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	now := time.Now().UTC()
	group := WorkspaceAgentGroup{ID: "group", Owner: p.UserID, TenantID: p.TenantID, Name: "Group", NormalizedName: "group", Revision: 1}
	org := WorkspaceAgentOrganization{ID: "org", Owner: p.UserID, TenantID: p.TenantID, AgentID: "agent", Pinned: true, Revision: 1}
	for _, row := range []any{&group, &org} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	assigned, deleted := make(chan error, 1), make(chan error, 1)
	go func() {
		<-start
		assigned <- db.Transaction(func(tx *gorm.DB) error {
			_, err := s.patchWorkspaceOrganizationOn(tx, p, "agent", contract.PatchOrganizationRequest{ExpectedRevision: 1, GroupID: contract.Field[string]{Present: true, Value: group.ID}}, now)
			return err
		})
	}()
	go func() {
		<-start
		deleted <- db.Transaction(func(tx *gorm.DB) error { return s.deleteWorkspaceGroupOn(tx, p, group.ID, 1, now) })
	}()
	close(start)
	assignmentErr, deleteErr := <-assigned, <-deleted
	if deleteErr != nil {
		t.Fatal(deleteErr)
	}
	if assignmentErr != nil && assignmentErr.Error() != "not_found" {
		t.Fatalf("unexpected assignment outcome: %v", assignmentErr)
	}
	var final WorkspaceAgentOrganization
	if err := db.First(&final, "id = ?", org.ID).Error; err != nil {
		t.Fatal(err)
	}
	if final.GroupID != nil || !final.Pinned {
		t.Fatal("deleted group retained binding or lost pin")
	}
	expected := uint64(1)
	if assignmentErr == nil {
		expected = 3
	}
	if final.Revision != expected {
		t.Fatalf("revision %d expected %d", final.Revision, expected)
	}
	var count int64
	if err := db.Model(&WorkspaceAgentGroup{}).Where("id = ?", group.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("group remained")
	}
}
