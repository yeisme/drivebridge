package service

import (
	"errors"
	"testing"
)

func TestWorkspaceVisibilityDoesNotHideInfrastructureFailure(t *testing.T) {
	for _, name := range []string{"not_found", "forbidden", "approval_required", "team_archived"} {
		if !workspaceVisibilityDenied(code(name)) {
			t.Fatalf("expected visibility denial: %s", name)
		}
	}
	for _, err := range []error{nil, code("database_unavailable"), code("unauthorized"), code("identity_unavailable"), errors.New("fixture database failure")} {
		if workspaceVisibilityDenied(err) {
			t.Fatalf("non-visibility failure would be hidden: %v", err)
		}
	}
}
