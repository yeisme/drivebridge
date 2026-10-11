package contract

import (
	"strings"
	"unicode/utf8"
)

func validName(value string, max int) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= max
}

func validActions(actions []string) bool {
	if len(actions) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, action := range actions {
		switch action {
		case "files.read", "labels.suggest", "inbox.submit":
		default:
			return false
		}
		if seen[action] {
			return false
		}
		seen[action] = true
	}
	return true
}

// Validate checks request values only. Resource ownership, normalized prefix
// containment, revision CAS and lifecycle eligibility remain service checks.
func (in CreateGrantRequest) Validate() error {
	if !validName(in.Name, 120) || (in.Protocol != "mcp" && in.Protocol != "http") || in.SpaceID == "" || in.ConnectionID == "" || !validActions(in.Actions) {
		return ErrInvalidRequest
	}
	_, _, err := (ExpiryInput{ExpiryMode: in.ExpiryMode, ExpiresInSeconds: in.ExpiresInSeconds}).Resolve(false)
	return err
}

func (in PatchGrantRequest) Validate() error {
	if in.Name.Present && (in.Name.Null || !validName(in.Name.Value, 120)) {
		return ErrInvalidRequest
	}
	if in.Prefix.Present && in.Prefix.Null {
		return ErrInvalidRequest
	}
	if in.Enabled.Present && in.Enabled.Null {
		return ErrInvalidRequest
	}
	if in.Actions.Present && (in.Actions.Null || !validActions(in.Actions.Value)) {
		return ErrInvalidRequest
	}
	_, _, err := (ExpiryInput{ExpiryMode: in.ExpiryMode, ExpiresInSeconds: in.ExpiresInSeconds}).Resolve(true)
	return err
}

func (in PatchOrganizationRequest) Validate() error {
	if in.Pinned.Present && in.Pinned.Null {
		return ErrInvalidRequest
	}
	if in.GroupID.Present && !in.GroupID.Null && strings.TrimSpace(in.GroupID.Value) == "" {
		return ErrInvalidRequest
	}
	return nil
}
