package service

// Only explicit visibility denials may omit a row. Infrastructure failures
// must remain errors rather than turning an unavailable list into an empty one.
func workspaceVisibilityDenied(err error) bool {
	if err == nil {
		return false
	}
	switch safeCode(err) {
	case "not_found", "forbidden", "approval_required", "team_archived":
		return true
	default:
		return false
	}
}
