package service

import (
	"testing"
)

func TestWorkspaceArchiveConflictDoesNotRenameOrReserve(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	source := uploadBytes(t, s, p, sp.ID, "source.txt", []byte("source bytes"))
	existing := uploadBytes(t, s, p, sp.ID, "target.txt", []byte("existing bytes"))
	q := ArchiveRequest{SourceFileID: source.FileID, SourceVersionID: source.VersionID, ExpectedSourceVersion: source.VersionID, TargetSpaceID: sp.ID, TargetPath: "target.txt"}
	var before Space
	if err := s.DB.First(&before, "id = ?", sp.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.archiveVersion(t.Context(), p, q, "conflict", false); err == nil || err.Error() != "name_conflict" {
		t.Fatalf("expected conflict, got %v", err)
	}
	var count int64
	if err := s.DB.Model(&Operation{}).Where("owner = ? AND key = ?", p.UserID, "conflict").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("conflict created operation")
	}
	var after Space
	if err := s.DB.First(&after, "id = ?", sp.ID).Error; err != nil || after.Reserved != before.Reserved || after.Used != before.Used {
		t.Fatal("conflict changed quota")
	}
	var target File
	if err := s.DB.First(&target, "id = ?", existing.FileID).Error; err != nil || target.VersionID != existing.VersionID {
		t.Fatal("conflict overwrote target")
	}
	q.TargetPath = "archived.txt"
	archived, err := s.archiveVersion(t.Context(), p, q, "archive", false)
	if err != nil || archived.Status != "completed" || archived.RenameOnConflict || archived.Path != "archived.txt" {
		t.Fatalf("explicit archive failed: %v", err)
	}
	replay, err := s.archiveVersion(t.Context(), p, q, "archive", false)
	if err != nil || replay.ID != archived.ID || replay.VersionID != archived.VersionID {
		t.Fatal("archive replay changed identity")
	}
	q.TargetPath = "target.txt"
	legacy, err := s.ArchiveVersion(t.Context(), p, q, "legacy")
	if err != nil || legacy.Path == "target.txt" || !legacy.RenameOnConflict {
		t.Fatalf("legacy rename regressed: %v", err)
	}
}
