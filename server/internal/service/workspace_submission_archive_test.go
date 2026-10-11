package service

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestWorkspaceArchivePreparationRequiresAcceptedOwnedVersion(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &File{}, &Version{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	fileID, versionID := "file", "version"
	sub := WorkspaceSubmission{ID: "sub", Owner: "human", TenantID: "tenant", AgentID: "agent", GrantID: "grant", SpaceID: "source", UploadOperationID: "op", CreationKey: "key", RequestDigest: "digest", State: "accepted", Revision: 3, FileID: &fileID, VersionID: &versionID}
	for _, row := range []any{&User{ID: "human", Issuer: "fixture", Subject: "human"}, &Space{ID: "source", Owner: "human", TenantID: "tenant"}, &Space{ID: "target", Owner: "human", TenantID: "tenant"}, &Space{ID: "foreign", Owner: "other", TenantID: "tenant"}, &Space{ID: "other-tenant", Owner: "human", TenantID: "other"}, &File{ID: fileID, Owner: "human", SpaceID: "source", Path: "refs/.inbox-staging/grant/sub/result.txt", VersionID: versionID}, &Version{ID: versionID, Owner: "human", FileID: fileID}, &sub} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: "human", TenantID: "tenant"}
	input := contract.ArchiveSubmissionRequest{ExpectedRevision: 3, VersionID: versionID, TargetSpaceID: "target", TargetPrefix: "accepted"}
	q, err := s.prepareWorkspaceArchiveOn(db, p, sub.ID, input)
	if err != nil || q.TargetPath != "accepted/result.txt" || q.SourceVersionID != versionID {
		t.Fatalf("valid preparation failed: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*contract.ArchiveSubmissionRequest)
	}{
		{"foreign owner", func(q *contract.ArchiveSubmissionRequest) { q.TargetSpaceID = "foreign" }},
		{"foreign tenant", func(q *contract.ArchiveSubmissionRequest) { q.TargetSpaceID = "other-tenant" }},
		{"staging", func(q *contract.ArchiveSubmissionRequest) { q.TargetPrefix = "refs/.inbox-staging" }},
		{"traversal", func(q *contract.ArchiveSubmissionRequest) { q.TargetPrefix = "a/../b" }},
		{"wrong version", func(q *contract.ArchiveSubmissionRequest) { q.VersionID = "other" }},
		{"stale revision", func(q *contract.ArchiveSubmissionRequest) { q.ExpectedRevision = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := input
			tc.mutate(&bad)
			if _, err := s.prepareWorkspaceArchiveOn(db, p, sub.ID, bad); err == nil {
				t.Fatal("invalid archive accepted")
			}
		})
	}
	agent := p
	agent.Agent = &Agent{ID: "agent"}
	if _, err := s.prepareWorkspaceArchiveOn(db, agent, sub.ID, input); err == nil {
		t.Fatal("Agent archived")
	}
	if err := db.Model(&WorkspaceSubmission{}).Where("id = ?", sub.ID).Update("state", "submitted").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.prepareWorkspaceArchiveOn(db, p, sub.ID, input); err == nil {
		t.Fatal("unaccepted submission archived")
	}
}
