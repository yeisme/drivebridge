package desktopcontract

import (
	"strings"
	"testing"
)

func TestDesktopSafeErrorsPreserveCodesNotPayloads(t *testing.T) {
	for _, tc := range []struct {
		origin, code string
		status       int
	}{{"drivebridge", "managed_owner_required", 403}, {"drivebridge", "desktop_session_invalid", 401}, {"pinax", "revision_conflict", 409}, {"pinax", "reader_cursor_expired", 410}, {"pinax", "insufficient_scope", 403}, {"pinax", "document_too_large", 413}, {"pinax", "reader_range_invalid", 416}} {
		result, status := SafeError(tc.origin, tc.code, "")
		if status != tc.status || result.Code != tc.code || result.Origin != tc.origin {
			t.Fatal("error mapping drift", tc.code)
		}
	}
	for _, origin := range []string{"drivebridge", "pinax"} {
		value, status := SafeError(origin, "outcome_unknown", "operation-fixture")
		if status != 503 || value.Recovery != "lookup_original" || value.OperationID == nil {
			t.Fatal("lost operation identity")
		}
		value, _ = SafeError(origin, "outcome_unknown", "/private/secret")
		if value.OperationID != nil || strings.Contains(value.Message, "secret") {
			t.Fatal("private operation leaked")
		}
		value, _ = SafeError(origin, "/private/secret", "")
		if strings.Contains(value.Message, "secret") || strings.Contains(value.Code, "secret") {
			t.Fatal("unsafe error code leaked")
		}
	}
}
func TestDesktopCapabilitiesRequireAllRuntimeFacts(t *testing.T) {
	if len(AvailableMethods(nil, nil, nil)) != 0 {
		t.Fatal("planned methods became available")
	}
	yes := map[string]bool{"readDocument": true, "arbitraryFetch": true}
	if len(AvailableMethods(yes, yes, nil)) != 0 {
		t.Fatal("missing grant allowed")
	}
	got := AvailableMethods(yes, yes, yes)
	if len(got) != 1 || got[0] != "readDocument" {
		t.Fatal("unknown method or intersection")
	}
}
