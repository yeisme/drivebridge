package service

import (
	"testing"
)

func TestCompactWorkspaceToolsExposeReadOnlyTaskSurface(t *testing.T) {
	tools := compactWorkspaceTools()
	if len(tools) != 3 {
		t.Fatalf("tool count=%d", len(tools))
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.InputSchema.Properties == nil {
			t.Fatalf("%s missing input schema", tool.Name)
		}
	}
}

func TestCompactWorkspaceSearchRejectsUnboundedOrUnknownKinds(t *testing.T) {
	s, _, human := blobRig(t)
	p := Principal{UserID: human.UserID}
	if _, err := s.workspaceCompactSearch(p, "", "", "", 50); safeCode(err) != "invalid_request" {
		t.Fatalf("empty kind error=%v", err)
	}
	if _, err := s.workspaceCompactSearch(p, "unknown", "", "", 50); safeCode(err) != "invalid_space" && safeCode(err) != "unsupported_action" {
		t.Fatalf("unknown kind error=%v", err)
	}
}

func TestCompactWorkspaceInspectRejectsMissingTarget(t *testing.T) {
	s, _, human := blobRig(t)
	p := Principal{UserID: human.UserID}
	if _, err := s.workspaceCompactInspect(p, "file", ""); safeCode(err) != "invalid_request" {
		t.Fatalf("missing target error=%v", err)
	}
	if _, err := s.workspaceCompactRecover(p, "unknown", "id"); safeCode(err) != "unsupported_action" {
		t.Fatalf("unknown recovery error=%v", err)
	}
}
