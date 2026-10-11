package sdk

import "testing"

func TestAllowedHandoffPurposesKeepReference(t *testing.T) {
	seen := map[string]bool{}
	for _, purpose := range AllowedHandoffPurposes {
		seen[purpose] = true
	}
	for _, purpose := range []string{"reference", "vault", "notes-attachments", "materials", "saveback", "owner-store"} {
		if !seen[purpose] {
			t.Fatal(purpose)
		}
	}
}
