package service

import "testing"

func TestEnumerationToolsAreHiddenForNonAdminUsers(t *testing.T) {
	for _, name := range []string{"files", "spaces", "search", "changes", "inbox", "operations", "usage_summary", "usage_series", "usage_breakdown", "usage_scans", "audit_events"} {
		if !isEnumerationTool(name) {
			t.Fatalf("%q must be classified as enumeration", name)
		}
	}
	for _, name := range []string{"capabilities", "upload", "create_upload", "complete_upload", "operation_get", "stat", "prepare_reference"} {
		if isEnumerationTool(name) {
			t.Fatalf("%q must remain usable by ordinary users", name)
		}
	}
}
