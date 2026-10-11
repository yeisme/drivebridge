package desktopcontract

import (
	"errors"
	"regexp"
	"time"
)

var mutationKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// NativeSelection is private main-to-facade input, not a renderer path or
// authorization. The eventual facade must verify its session-bound origin.
type NativeSelection struct {
	NativeSelectionID string `json:"native_selection_id" jsonschema:"minLength=1,maxLength=128"`
}

// ReaderScopes is deliberately distinct from writer authorization. Confirming
// this proposal cannot create or upgrade a writer session.
type ReaderScope string

const (
	ScopeMetadata  ReaderScope = "reader.metadata"
	ScopeBody      ReaderScope = "reader.body"
	ScopeRelations ReaderScope = "reader.relations"
	ScopeAssets    ReaderScope = "reader.assets"
)

type BindingPreview struct {
	PreviewID             string        `json:"preview_id" jsonschema:"minLength=1,maxLength=128"`
	PreviewDigest         string        `json:"preview_digest" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	DisplayName           string        `json:"display_name" jsonschema:"minLength=1,maxLength=256"`
	ExpectedGuardRevision uint64        `json:"expected_guard_revision" jsonschema:"maximum=9007199254740991"`
	Scopes                []ReaderScope `json:"scopes" jsonschema:"minItems=1,maxItems=4,uniqueItems=true"`
	Decision              string        `json:"decision" jsonschema:"enum=ready,enum=blocked"`
	Reason                string        `json:"reason" jsonschema:"enum=none,enum=managed_scope_busy,enum=managed_scope_unverifiable,enum=owner_unavailable,enum=guard_unavailable"`
	// Counts represent this authorized protection domain, never a vault scan.
	OverlappingRoots uint32    `json:"overlapping_roots"`
	PendingMutations uint32    `json:"pending_mutations"`
	ObservedAt       time.Time `json:"observed_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type ConfirmBinding struct {
	PreviewID             string `json:"preview_id" jsonschema:"minLength=1,maxLength=128"`
	PreviewDigest         string `json:"preview_digest" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	ExpectedGuardRevision uint64 `json:"expected_guard_revision" jsonschema:"maximum=9007199254740991"`
	// Caller assertion only. Authorization still comes from the host session.
	Confirmed bool `json:"confirmed"`
}

type SetSourceVisibility struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	Hidden           bool   `json:"hidden"`
}

type ReleaseProtection struct {
	ExpectedRevision uint64 `json:"expected_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	Confirmed        bool   `json:"confirmed"`
}

// BindingView is a redacted projection. Canonical roots and filesystem
// identities belong in the application service's private persistence model.
type BindingView struct {
	BindingID        string    `json:"binding_id" jsonschema:"minLength=1,maxLength=128"`
	SourceID         string    `json:"source_id" jsonschema:"minLength=1,maxLength=128"`
	InstanceID       string    `json:"instance_id" jsonschema:"minLength=1,maxLength=128"`
	State            string    `json:"state" jsonschema:"enum=active,enum=hidden,enum=blocked,enum=released"`
	Revision         uint64    `json:"revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	GuardRevision    uint64    `json:"guard_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	ProtectionActive bool      `json:"protection_active"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BindingMutationLookup struct {
	Action         string `json:"action" jsonschema:"enum=confirm,enum=release"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=128,pattern=^[A-Za-z0-9][A-Za-z0-9._:-]*$"`
}

type BindingMutationReceipt struct {
	OperationID string      `json:"operation_id" jsonschema:"minLength=1,maxLength=128"`
	Action      string      `json:"action" jsonschema:"enum=confirm,enum=release"`
	Binding     BindingView `json:"binding"`
}

type BindingLookupResult struct {
	Found   bool                    `json:"found"`
	Receipt *BindingMutationReceipt `json:"receipt" jsonschema:"nullable"`
}

func validReaderScopes(scopes []ReaderScope) bool {
	if len(scopes) == 0 || len(scopes) > 4 {
		return false
	}
	seen := map[ReaderScope]bool{}
	for _, scope := range scopes {
		switch scope {
		case ScopeMetadata, ScopeBody, ScopeRelations, ScopeAssets:
		default:
			return false
		}
		if seen[scope] {
			return false
		}
		seen[scope] = true
	}
	// Every bound source remains discoverable within the approved session.
	return seen[ScopeMetadata]
}

func (p BindingPreview) Validate() error {
	if !validID(p.PreviewID) || !validDigest(p.PreviewDigest) || len(p.DisplayName) == 0 || len(p.DisplayName) > 256 || p.ExpectedGuardRevision > maxSafeRevision || !validReaderScopes(p.Scopes) || p.ObservedAt.IsZero() || !p.ExpiresAt.After(p.ObservedAt) || p.ExpiresAt.Sub(p.ObservedAt) > 5*time.Minute {
		return errors.New("invalid_binding_preview")
	}
	switch p.Decision {
	case "ready":
		if p.Reason != "none" || p.PendingMutations != 0 {
			return errors.New("invalid_binding_preview")
		}
	case "blocked":
		switch p.Reason {
		case "managed_scope_busy", "managed_scope_unverifiable", "owner_unavailable", "guard_unavailable":
		default:
			return errors.New("invalid_binding_preview")
		}
	default:
		return errors.New("invalid_binding_preview")
	}
	return nil
}

const maxSafeRevision uint64 = 9007199254740991

func validDigest(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (r ConfirmBinding) Validate() error {
	if !validID(r.PreviewID) || !validDigest(r.PreviewDigest) || !r.Confirmed || r.ExpectedGuardRevision > maxSafeRevision {
		return errors.New("invalid_binding_confirmation")
	}
	return nil
}
func (r ReleaseProtection) Validate() error {
	if r.ExpectedRevision == 0 || r.ExpectedRevision > maxSafeRevision || !r.Confirmed {
		return errors.New("invalid_protection_release")
	}
	return nil
}
func (r SetSourceVisibility) Validate() error {
	if r.ExpectedRevision == 0 || r.ExpectedRevision > maxSafeRevision {
		return errors.New("invalid_source_visibility")
	}
	return nil
}
func (r BindingMutationLookup) Validate() error {
	if (r.Action != "confirm" && r.Action != "release") || !mutationKey.MatchString(r.IdempotencyKey) {
		return errors.New("invalid_binding_lookup")
	}
	return nil
}
func (b BindingView) Validate() error {
	if !validID(b.BindingID) || !validID(b.SourceID) || !validID(b.InstanceID) || b.Revision == 0 || b.Revision > maxSafeRevision || b.GuardRevision == 0 || b.GuardRevision > maxSafeRevision || b.UpdatedAt.IsZero() {
		return errors.New("invalid_binding_view")
	}
	switch b.State {
	case "active", "hidden", "blocked":
		if !b.ProtectionActive {
			return errors.New("invalid_binding_protection")
		}
	case "released":
		if b.ProtectionActive {
			return errors.New("invalid_binding_protection")
		}
	default:
		return errors.New("invalid_binding_state")
	}
	return nil
}
func (r BindingMutationReceipt) Validate() error {
	if !validID(r.OperationID) || (r.Action != "confirm" && r.Action != "release") {
		return errors.New("invalid_binding_receipt")
	}
	if (r.Action == "confirm" && r.Binding.State != "active") || (r.Action == "release" && r.Binding.State != "released") {
		return errors.New("invalid_binding_receipt")
	}
	return r.Binding.Validate()
}
func (r BindingLookupResult) Validate() error {
	if r.Found != (r.Receipt != nil) {
		return errors.New("invalid_binding_lookup_result")
	}
	if r.Receipt != nil {
		return r.Receipt.Validate()
	}
	return nil
}
