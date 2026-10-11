package service

import "testing"

func TestWorkspaceSubmissionIdentityConstraints(t *testing.T) {
	db := workspaceTestDB(t)
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	first := WorkspaceSubmission{ID: "submission-a", Owner: "human", TenantID: "tenant", GrantID: "grant-a", CreationKey: "request", RequestDigest: "digest", AgentID: "agent-a", SpaceID: "space", UploadOperationID: "operation-a", State: "uploading", Revision: 1}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.ID = "duplicate"
	duplicate.UploadOperationID = "operation-other"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("same grant key duplicated")
	}
	other := first
	other.ID = "submission-b"
	other.GrantID = "grant-b"
	other.AgentID = "agent-b"
	other.UploadOperationID = "operation-b"
	if err := db.Create(&other).Error; err != nil {
		t.Fatal("distinct grant cannot use same client key")
	}
	reused := other
	reused.ID = "submission-c"
	reused.CreationKey = "another"
	if err := db.Create(&reused).Error; err == nil {
		t.Fatal("upload operation reused across submissions")
	}
	decision := WorkspaceSubmissionDecision{ID: "decision-a", SubmissionID: first.ID, Revision: 2, HumanID: "human", VersionID: "version", Decision: "accept"}
	if err := db.Create(&decision).Error; err != nil {
		t.Fatal(err)
	}
	decision.ID = "decision-b"
	if err := db.Create(&decision).Error; err == nil {
		t.Fatal("same submission revision decided twice")
	}
	var original WorkspaceSubmission
	if err := db.First(&original, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if original.State != "uploading" || original.ArchiveOperationID != nil {
		t.Fatal("decision model silently archived submission")
	}
}
