package service

import (
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceSuggestionAcceptancePreservesManualLabels(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &File{}, &FileLabelAssignment{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, row := range []any{
		&User{ID: "human", Issuer: "fixture", Subject: "human"},
		&Space{ID: "space", Owner: "human", TenantID: "tenant"},
		&File{ID: "file", Owner: "human", SpaceID: "space", Path: "refs/file", VersionID: "v1"},
		&FileLabelAssignment{ID: "manual", Owner: "human", FileID: "file", Namespace: "manual", Label: "keep", AuthorKind: "human"},
		&WorkspaceLabelSuggestion{ID: "suggestion", Owner: "human", TenantID: "tenant", AgentID: "agent", GrantID: "grant", FileID: "file", VersionID: "v1", CreationKey: "key", RequestDigest: "digest", Labels: []string{"keep", "new", "excluded"}, State: "pending", Revision: 1},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	input := contract.SuggestionDecisionRequest{ExpectedRevision: 1, BaseVersionID: "v1", Decision: "accept", Labels: []string{"keep", "new"}}
	agent := p
	agent.Agent = &Agent{ID: "agent"}
	if _, err := s.decideWorkspaceSuggestionOn(db, agent, "suggestion", input, now); err == nil {
		t.Fatal("Agent self-approved")
	}
	invalid := input
	invalid.Labels = []string{"not-proposed"}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := s.decideWorkspaceSuggestionOn(tx, p, "suggestion", invalid, now)
		return err
	}); err == nil {
		t.Fatal("non-subset accepted")
	}
	stale := input
	stale.BaseVersionID = "v0"
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := s.decideWorkspaceSuggestionOn(tx, p, "suggestion", stale, now)
		return err
	}); err == nil {
		t.Fatal("stale version accepted")
	}
	abort := errors.New("fixture rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.decideWorkspaceSuggestionOn(tx, p, "suggestion", input, now); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var pending WorkspaceLabelSuggestion
	if err := db.First(&pending, "id = ?", "suggestion").Error; err != nil {
		t.Fatal(err)
	}
	if pending.State != "pending" || pending.Revision != 1 {
		t.Fatal("rollback retained decision")
	}
	var decisions, manual int64
	if err := db.Model(&WorkspaceSuggestionDecision{}).Count(&decisions).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&FileLabelAssignment{}).Count(&manual).Error; err != nil {
		t.Fatal(err)
	}
	if decisions != 0 || manual != 1 {
		t.Fatal("rollback retained labels or decision record")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		result, err := s.decideWorkspaceSuggestionOn(tx, p, "suggestion", input, now)
		if err == nil && (result.State != "accepted" || result.Revision != 2) {
			t.Fatal("decision not persisted")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var labels []FileLabelAssignment
	if err := db.Where("file_id = ? AND namespace = ?", "file", "manual").Order("label").Find(&labels).Error; err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 || labels[0].ID != "manual" || labels[0].AcceptedFrom != "" || labels[1].Label != "new" || labels[1].AcceptedFrom != "suggestion" {
		t.Fatal("manual labels overwritten or unselected label added")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := s.decideWorkspaceSuggestionOn(tx, p, "suggestion", input, now)
		return err
	}); err == nil {
		t.Fatal("same revision decided twice")
	}
}
