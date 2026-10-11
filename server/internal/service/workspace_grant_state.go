package service

import "time"

// workspaceGrantState is fail-closed for inconsistent persisted profiles and
// expiry combinations. It never changes legacy Agent expiry semantics.
func workspaceGrantState(grant WorkspaceGrant, now time.Time) string {
	if grant.RestrictionProfile != "personal_workspace_v1" {
		return "invalid"
	}
	switch grant.ExpiryMode {
	case "never":
		if grant.ExpiresAt != nil {
			return "invalid"
		}
	case "duration":
		if grant.ExpiresAt == nil {
			return "invalid"
		}
	default:
		return "invalid"
	}
	if grant.Revoked {
		return "revoked"
	}
	if grant.ExpiryMode == "duration" && !now.Before(*grant.ExpiresAt) {
		return "expired"
	}
	if grant.Disabled {
		return "disabled"
	}
	return "active"
}
