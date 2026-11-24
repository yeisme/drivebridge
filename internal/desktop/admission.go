package desktop

import (
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

// Admission is the owner-side managed-vault seam. It is the only way a host can
// turn a native selection into a binding: without it every preview stays
// `blocked/managed_scope_unverifiable` and no confirmation can ever produce a
// receipt, an operation or a session.
//
// Implementations must be the owner's own guard (for the local CLI that is
// internal/bridge). A rendering adapter must not re-implement the domain check.
type Admission interface {
	// Preview reports the protection domain the selected root would join.
	// It never persists, never creates directories and never grants a session.
	Preview(root string, scopes []desktopcontract.ReaderScope) (AdmissionPreview, error)
	// Confirm installs the guard under compare-and-swap and returns the
	// owner receipt so a lost response can be reconciled with the same key.
	Confirm(root, bindingID string, expected uint64, digest string, scopes []desktopcontract.ReaderScope, key string) (AdmissionReceipt, error)
	// SetVisibility hides or restores navigation. Hiding must not release
	// protection; only Release does that.
	SetVisibility(root, bindingID string, expected uint64, hidden bool) (AdmissionReceipt, error)
	// Release revokes binding authority. It requires an explicit confirmation,
	// a matching revision and no in-flight work in the domain.
	Release(root, bindingID string, expected uint64, key string) (AdmissionReceipt, error)
	// Acquire serializes one bounded owner call with release and rechecks the
	// binding plus physical root immediately before the call.
	Acquire(root, bindingID string, epoch uint64, scopes []string) (func() error, error)
}

// AdmissionPreview mirrors the owner's redacted preflight projection.
type AdmissionPreview struct {
	BindingID             string
	Digest                string
	Decision              string
	Reason                string
	ExpectedGuardRevision uint64
	OverlappingRoots      uint32
	PendingMutations      uint32
}

// AdmissionReceipt mirrors the owner's confirm receipt.
type AdmissionReceipt struct {
	OperationID string
	BindingID   string
	SourceID    string
	InstanceID  string
	Revision    uint64
	State       string
	UpdatedAt   time.Time
	Epoch       uint64
}

// validate keeps a misbehaving adapter from handing the host an unusable
// projection: only the owner contract's own decision/reason pairs are accepted.
func (p AdmissionPreview) validate() error {
	switch p.Decision {
	case "ready":
		if p.Reason != "none" || p.PendingMutations != 0 || p.Digest == "" || p.BindingID == "" {
			return errors.New("binding_preview_invalid")
		}
	case "blocked":
		switch p.Reason {
		case "managed_scope_busy", "managed_scope_unverifiable", "owner_unavailable", "guard_unavailable":
		default:
			return errors.New("binding_preview_invalid")
		}
	default:
		return errors.New("binding_preview_invalid")
	}
	return nil
}

func (r AdmissionReceipt) validate() error {
	if r.OperationID == "" || r.BindingID == "" || r.SourceID == "" || r.InstanceID == "" || r.Revision == 0 || r.UpdatedAt.IsZero() {
		return errors.New("binding_receipt_invalid")
	}
	switch r.State {
	case "active", "hidden", "released":
	default:
		return errors.New("binding_receipt_invalid")
	}
	return nil
}

// bindingView projects an owner receipt into the closed renderer DTO.
func (r AdmissionReceipt) bindingView(sourceID, instanceID string) (desktopcontract.BindingView, error) {
	if err := r.validate(); err != nil {
		return desktopcontract.BindingView{}, err
	}
	view := desktopcontract.BindingView{
		BindingID:        r.BindingID,
		SourceID:         sourceID,
		InstanceID:       instanceID,
		State:            r.State,
		Revision:         r.Revision,
		GuardRevision:    r.Revision,
		ProtectionActive: r.State != "released",
		UpdatedAt:        r.UpdatedAt,
	}
	if err := view.Validate(); err != nil {
		return desktopcontract.BindingView{}, errors.New("binding_receipt_invalid")
	}
	return view, nil
}
