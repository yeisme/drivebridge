package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestWorkspaceGovernanceSpaceFailureIsNotVisibilityDenial(t *testing.T) {
	s, _, p := blobRig(t)
	if err := s.BootstrapGovernance(p.UserID); err != nil {
		t.Fatal(err)
	}
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	p.TenantID = s.Instance
	if err := s.governanceAllow(s.DB, p, sp.ID, "file.txt", "read"); err != nil {
		t.Fatal(err)
	}
	const callback = "fixture_authorization_space_failure"
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*Space); ok {
			tx.AddError(errors.New("fixture query failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed := s.governanceAllow(s.DB, p, sp.ID, "file.txt", "read")
	if err := s.DB.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if failed == nil || failed.Error() != "database_unavailable" || workspaceVisibilityDenied(failed) {
		t.Fatalf("database failure hidden as visibility denial: %v", failed)
	}
	missing := s.governanceAllow(s.DB, p, "missing-space", "file.txt", "read")
	if missing == nil || missing.Error() != "not_found" || !workspaceVisibilityDenied(missing) {
		t.Fatalf("missing space misclassified: %v", missing)
	}
}
