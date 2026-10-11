package service

import (
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func TestWorkspaceSubmissionHumanDecisionKeepsFixedBytes(t *testing.T) {
	for _, decision := range []string{"accept", "reject"} {
		t.Run(decision, func(t *testing.T) {
			db := workspaceTestDB(t)
			if err := db.AutoMigrate(&User{}, &Space{}, &File{}, &Version{}); err != nil {
				t.Fatal(err)
			}
			if err := migrateWorkspaceModels(db); err != nil {
				t.Fatal(err)
			}
			fileID, versionID := "file", "version"
			sub := WorkspaceSubmission{ID: "sub", Owner: "human", TenantID: "tenant", AgentID: "agent", GrantID: "grant", SpaceID: "space", UploadOperationID: "op", CreationKey: "key", RequestDigest: "digest", State: "submitted", Revision: 2, FileID: &fileID, VersionID: &versionID}
			for _, row := range []any{&User{ID: "human", Issuer: "fixture", Subject: "human"}, &Space{ID: "space", Owner: "human", TenantID: "tenant"}, &File{ID: fileID, Owner: "human", SpaceID: "space", Path: "refs/.inbox-staging/file", VersionID: versionID, ReviewPending: true}, &Version{ID: versionID, Owner: "human", FileID: fileID}, &sub} {
				if err := db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			s := &Service{DB: db}
			p := Principal{UserID: "human", TenantID: "tenant"}
			now := time.Now().UTC()
			input := contract.SubmissionDecisionRequest{ExpectedRevision: 2, VersionID: versionID, Decision: decision, Reason: "Reviewed"}
			run := func(principal Principal, in contract.SubmissionDecisionRequest) error {
				return db.Transaction(func(tx *gorm.DB) error {
					_, err := s.decideWorkspaceSubmissionOn(tx, principal, sub.ID, in, now)
					return err
				})
			}
			agent := p
			agent.Agent = &Agent{ID: "agent"}
			if err := run(agent, input); err == nil {
				t.Fatal("Agent decided submission")
			}
			foreign := p
			foreign.TenantID = "foreign"
			if err := run(foreign, input); err == nil {
				t.Fatal("foreign tenant decided")
			}
			stale := input
			stale.VersionID = "other"
			if err := run(p, stale); err == nil {
				t.Fatal("wrong fixed version decided")
			}
			abort := errors.New("fixture rollback")
			if err := db.Transaction(func(tx *gorm.DB) error {
				if _, err := s.decideWorkspaceSubmissionOn(tx, p, sub.ID, input, now); err != nil {
					return err
				}
				return abort
			}); !errors.Is(err, abort) {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&WorkspaceSubmissionDecision{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("rollback retained decision")
			}
			if err := run(p, input); err != nil {
				t.Fatal(err)
			}
			if err := run(p, input); err == nil {
				t.Fatal("duplicate revision decided")
			}
			var result WorkspaceSubmission
			if err := db.First(&result, "id = ?", sub.ID).Error; err != nil {
				t.Fatal(err)
			}
			expected := "accepted"
			if decision == "reject" {
				expected = "rejected"
			}
			if result.State != expected || result.Revision != 3 || result.ArchiveOperationID != nil || result.VersionID == nil || *result.VersionID != versionID {
				t.Fatal("decision changed fixed reference or archived bytes")
			}
			var file File
			if err := db.First(&file, "id = ?", fileID).Error; err != nil {
				t.Fatal(err)
			}
			if file.VersionID != versionID || file.Path != "refs/.inbox-staging/file" || !file.ReviewPending || file.Trashed {
				t.Fatal("decision mutated file")
			}
		})
	}
}
