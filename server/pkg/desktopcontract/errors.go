package desktopcontract

import "regexp"

var errorCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`)

// SafeError takes codes only, never upstream exception text. Unrecognized
// codes remain unavailable and never imply that a mutation is safe to retry.
func SafeError(origin, code, operationID string) (DesktopError, int) {
	result := DesktopError{Origin: "drivebridge", Code: "owner_unavailable", Message: "The owner is unavailable.", Recovery: "owner_required"}
	status := 503
	if origin == "pinax" && errorCodePattern.MatchString(code) {
		result.Origin, result.Code = origin, code
		result.Message = "The note owner rejected the request."
		switch code {
		case "token_required", "invalid_token":
			status, result.Recovery = 401, "reauthorize"
		case "insufficient_scope", "reader_disabled", "writer_disabled", "unsafe_path":
			status = 403
		case "reader_object_not_found", "note_not_found":
			status, result.Recovery = 404, "none"
		case "revision_unavailable", "revision_conflict", "source_changed":
			status, result.Recovery = 409, "refresh"
		case "reader_cursor_expired":
			status, result.Recovery = 410, "refresh"
		case "document_too_large", "document_budget_exceeded", "body_too_large":
			status = 413
		case "reader_range_invalid":
			status, result.Recovery = 416, "none"
		case "reader_request_invalid", "reader_cursor_invalid", "writer_request_invalid":
			status, result.Recovery = 400, "none"
		case "reader_unsupported":
			status = 422
		}
		if (code == "outcome_unknown" || code == "reconcile_required") && mutationKey.MatchString(operationID) {
			result.OperationID = &operationID
			result.Recovery = "lookup_original"
		}
		return result, status
	}
	if origin != "drivebridge" {
		return result, status
	}
	type rule struct {
		status            int
		message, recovery string
	}
	rules := map[string]rule{
		"native_selection_invalid":     {400, "The native selection is invalid or expired.", "refresh"},
		"native_selection_changed":     {409, "The selected directory changed; choose it again.", "refresh"},
		"selection_capacity_exceeded":  {409, "Cancel a pending selection before choosing another folder.", "none"},
		"desktop_session_invalid":      {401, "The desktop session is invalid.", "reauthorize"},
		"managed_owner_required":       {403, "This resource must be accessed through its owner.", "owner_required"},
		"source_not_found":             {404, "The source is unavailable.", "none"},
		"binding_stale":                {409, "The binding changed; review it again.", "refresh"},
		"managed_scope_busy":           {409, "The protection scope has pending operations.", "owner_required"},
		"revision_conflict":            {409, "The resource revision changed.", "refresh"},
		"idempotency_conflict":         {409, "The original mutation identity conflicts.", "owner_required"},
		"name_conflict":                {409, "The selected destination already exists.", "refresh"},
		"cursor_expired":               {410, "The page cursor expired.", "refresh"},
		"binding_preview_expired":      {410, "The binding preview expired.", "refresh"},
		"managed_scope_unverifiable":   {422, "The protection scope cannot be verified.", "owner_required"},
		"desktop_contract_unsupported": {422, "The desktop contract is unsupported.", "owner_required"},
		"guard_unavailable":            {503, "The protection registry is unavailable.", "owner_required"},
		"request_invalid":              {400, "The desktop request is invalid.", "none"},
		"range_invalid":                {416, "The byte range is invalid or exceeds its budget.", "none"},
		"outcome_unknown":              {503, "The original operation must be inspected before retrying.", "owner_required"},
	}
	if value, ok := rules[code]; ok {
		result.Code, result.Message, result.Recovery, status = code, value.message, value.recovery, value.status
	}
	if code == "outcome_unknown" && mutationKey.MatchString(operationID) {
		result.OperationID = &operationID
		result.Recovery = "lookup_original"
	}
	return result, status
}

// AvailableMethods intersects runtime implementation, configuration and grant
// facts. The planned catalog alone never enables a method.
func AvailableMethods(implemented, configured, granted map[string]bool) []string {
	available := []string{}
	for _, method := range IPCMethods() {
		if implemented[method.Name] && configured[method.Name] && granted[method.Name] {
			available = append(available, method.Name)
		}
	}
	return available
}
