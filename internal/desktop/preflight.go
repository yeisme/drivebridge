// Package desktop implements local desktop application boundaries. It does
// not expose native paths, session credentials or arbitrary IO to a renderer.
package desktop

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

const selectionLifetime = 5 * time.Minute
const maxSelections = 32

type selection struct {
	path     string
	identity os.FileInfo
	expires  time.Time
}
type preview struct {
	selection selection
	view      desktopcontract.BindingPreview
}

// Preflight belongs to one authenticated main/facade session. Construct a new
// instance on every session change and Close the old one. It has no persistence
// or owner process: unsupported protection domains can never become bindings.
// Fields intentionally remain private, including in JSON projections.
type Preflight struct {
	mu         sync.Mutex
	closed     bool
	now        func() time.Time
	selections map[string]selection
	previews   map[string]preview
	// bindings keeps the session-local root and source identity for each
	// confirmed binding. It is never persisted and never returned to the renderer.
	bindings map[string]boundSource
	// admission is the owner managed-vault seam. Nil means this host can only
	// refuse: it has no way to prove a protection domain.
	admission Admission
}

func NewPreflight() *Preflight {
	return &Preflight{now: time.Now, selections: make(map[string]selection), previews: make(map[string]preview), bindings: make(map[string]boundSource)}
}

// NewPreflightWithAdmission wires the owner guard. Without it the host keeps the
// deny-only behaviour; an adapter must never substitute its own domain check.
func NewPreflightWithAdmission(admission Admission) *Preflight {
	p := NewPreflight()
	p.admission = admission
	return p
}

// Select is a trusted native-main entry point, NOT a renderer IPC or public
// HTTP method. Caller-controlled paths must never be routed to it.
func (p *Preflight) Select(nativePath string) (desktopcontract.NativeSelection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var empty desktopcontract.NativeSelection
	if p.closed {
		return empty, errors.New("desktop_session_invalid")
	}
	now := p.now()
	p.prune(now)
	if len(p.selections)+len(p.previews) >= maxSelections {
		return empty, errors.New("selection_capacity_exceeded")
	}
	identity, err := directoryIdentity(nativePath)
	if err != nil {
		return empty, err
	}
	id, err := opaqueID()
	if err != nil {
		return empty, err
	}
	p.selections[id] = selection{path: filepath.Clean(nativePath), identity: identity, expires: now.Add(selectionLifetime)}
	return desktopcontract.NativeSelection{NativeSelectionID: id}, nil
}

// Preview consumes a session-local native ticket exactly once. Only directory
// metadata is inspected, never Pinax config, index, note bodies or credentials.
// Zero counts are not coverage evidence: this currently blocked result cannot
// attest the complete protection domain or the absence of pending writers.
func (p *Preflight) Preview(ticket desktopcontract.NativeSelection) (desktopcontract.BindingPreview, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var empty desktopcontract.BindingPreview
	if p.closed {
		return empty, errors.New("desktop_session_invalid")
	}
	now := p.now()
	p.prune(now)
	selected, ok := p.selections[ticket.NativeSelectionID]
	if !ok {
		return empty, errors.New("native_selection_invalid")
	}
	delete(p.selections, ticket.NativeSelectionID)
	if err := selected.recheck(); err != nil {
		return empty, err
	}
	id, err := opaqueID()
	if err != nil {
		return empty, err
	}
	// Unpredictable opaque identity binds the projection to the in-memory
	// preview. This is not a digest of private path/metadata or a guard proof.
	digest := sha256.Sum256([]byte(id))
	scopes := []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata, desktopcontract.ScopeBody, desktopcontract.ScopeRelations, desktopcontract.ScopeAssets}
	view := desktopcontract.BindingPreview{PreviewID: id, PreviewDigest: "sha256:" + hex.EncodeToString(digest[:]), DisplayName: "Selected local folder", Scopes: scopes, Decision: "blocked", Reason: "managed_scope_unverifiable", ObservedAt: now.UTC(), ExpiresAt: selected.expires.UTC()}
	admission := p.admission
	if admission != nil {
		// The owner decides; a failed or invalid owner projection stays a
		// refusal instead of being repaired into a ready state.
		if owner, ownerErr := admission.Preview(selected.path, scopes); ownerErr == nil {
			if ownerErr = owner.validate(); ownerErr == nil {
				view.PreviewID = owner.BindingID
				view.PreviewDigest = owner.Digest
				view.Decision = owner.Decision
				view.Reason = owner.Reason
				view.ExpectedGuardRevision = owner.ExpectedGuardRevision
				view.OverlappingRoots = owner.OverlappingRoots
				view.PendingMutations = owner.PendingMutations
			}
		}
	}
	if err := view.Validate(); err != nil {
		return empty, errors.New("desktop_contract_unsupported")
	}
	p.previews[view.PreviewID] = preview{selection: selected, view: view}
	return clonePreview(view), nil
}

// ConfirmBlocked never installs a guard, writes a binding, or starts Pinax.
// This explicit denial is the only supported confirmation until writer
// admission has real evidence. There is deliberately no test/force bypass.
func (p *Preflight) ConfirmBlocked(request desktopcontract.ConfirmBinding) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("desktop_session_invalid")
	}
	if request.Validate() != nil {
		return errors.New("request_invalid")
	}
	p.prune(p.now())
	proposed, ok := p.previews[request.PreviewID]
	if !ok {
		return errors.New("binding_preview_expired")
	}
	if proposed.view.PreviewDigest != request.PreviewDigest || proposed.view.ExpectedGuardRevision != request.ExpectedGuardRevision {
		return errors.New("binding_stale")
	}
	if err := proposed.selection.recheck(); err != nil {
		return err
	}
	return errors.New("managed_scope_unverifiable")
}

// Confirm installs the owner guard under CAS and returns the receipt. Without an
// admission seam it is an explicit refusal: there is no test/force bypass and no
// path that turns a confirmation into a binding on its own.
func (p *Preflight) Confirm(request desktopcontract.ConfirmBinding, key string) (desktopcontract.BindingMutationReceipt, error) {
	var empty desktopcontract.BindingMutationReceipt
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return empty, errors.New("desktop_session_invalid")
	}
	if request.Validate() != nil {
		p.mu.Unlock()
		return empty, errors.New("request_invalid")
	}
	p.prune(p.now())
	if prior, ok := p.bindings[request.PreviewID]; ok && prior.confirmKey == key && prior.receipt.OperationID != "" {
		p.mu.Unlock()
		return prior.receipt, nil
	}
	proposed, ok := p.previews[request.PreviewID]
	if !ok {
		p.mu.Unlock()
		return empty, errors.New("binding_preview_expired")
	}
	if proposed.view.PreviewDigest != request.PreviewDigest || proposed.view.ExpectedGuardRevision != request.ExpectedGuardRevision {
		p.mu.Unlock()
		return empty, errors.New("binding_stale")
	}
	if err := proposed.selection.recheck(); err != nil {
		p.mu.Unlock()
		return empty, err
	}
	admission := p.admission
	root := proposed.selection.path
	scopes := append([]desktopcontract.ReaderScope{}, proposed.view.Scopes...)
	p.mu.Unlock()

	if admission == nil {
		return empty, errors.New("managed_scope_unverifiable")
	}
	if key == "" {
		return empty, errors.New("operation_key_required")
	}
	receipt, err := admission.Confirm(root, proposed.view.PreviewID, request.ExpectedGuardRevision, request.PreviewDigest, scopes, key)
	if err != nil {
		return empty, err
	}
	view, err := receipt.bindingView(receipt.SourceID, receipt.InstanceID)
	if err != nil {
		return empty, err
	}
	if receipt.Epoch == 0 || receipt.State != "active" {
		return empty, errors.New("binding_receipt_invalid")
	}

	// The confirmed preview is consumed; the writer session is a separate
	// object created by the caller, never an in-place upgrade of this ticket.
	result := desktopcontract.BindingMutationReceipt{OperationID: receipt.OperationID, Action: "confirm", Binding: view}
	p.mu.Lock()
	delete(p.previews, request.PreviewID)
	p.bindings[receipt.BindingID] = boundSource{BindingID: receipt.BindingID, SourceID: receipt.SourceID, Root: root, Epoch: receipt.Epoch, Scopes: readerScopeNames(scopes), confirmKey: key, receipt: result}
	p.mu.Unlock()
	return result, nil
}

func (p *Preflight) bindingRoot(bindingID string) (string, Admission, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", nil, errors.New("desktop_session_invalid")
	}
	bound, ok := p.bindings[bindingID]
	if !ok {
		return "", nil, errors.New("binding_unknown")
	}
	if p.admission == nil {
		return "", nil, errors.New("managed_scope_unverifiable")
	}
	return bound.Root, p.admission, nil
}

// SourceRoot resolves a bound source identity to its owner root. The renderer
// only ever sees the opaque source id; the path never leaves the host.
func (p *Preflight) SourceRoot(sourceID string) (string, error) {
	bound, _, err := p.sourceBinding(sourceID)
	return bound.Root, err
}

// sourceBinding returns the private owner root, binding identity and admission
// seam for one active source. Callers must acquire the seam before starting an
// owner session; returning the root alone is insufficient because another
// session may release or replace the binding after lookup.
func (p *Preflight) sourceBinding(sourceID string) (boundSource, Admission, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return boundSource{}, nil, errors.New("desktop_session_invalid")
	}
	for _, bound := range p.bindings {
		if bound.SourceID == sourceID && !bound.Released {
			if p.admission == nil {
				return boundSource{}, nil, errors.New("managed_scope_unverifiable")
			}
			return bound, p.admission, nil
		}
	}
	return boundSource{}, nil, errors.New("source_unknown")
}

// SetVisibility hides or restores navigation on a binding this session created.
// Hiding never releases protection.
func (p *Preflight) SetVisibility(bindingID string, request desktopcontract.SetSourceVisibility) (desktopcontract.BindingView, error) {
	var empty desktopcontract.BindingView
	if request.Validate() != nil {
		return empty, errors.New("request_invalid")
	}
	root, admission, err := p.bindingRoot(bindingID)
	if err != nil {
		return empty, err
	}
	receipt, err := admission.SetVisibility(root, bindingID, request.ExpectedRevision, request.Hidden)
	if err != nil {
		return empty, err
	}
	return receipt.bindingView(receipt.SourceID, receipt.InstanceID)
}

// Release revokes a binding this session created. A confirmation assertion alone
// is not authorization; the owner still enforces confirmation, revision and
// in-flight work.
func (p *Preflight) Release(bindingID string, request desktopcontract.ReleaseProtection, key string) (desktopcontract.BindingMutationReceipt, error) {
	var empty desktopcontract.BindingMutationReceipt
	if request.Validate() != nil {
		return empty, errors.New("request_invalid")
	}
	if key == "" {
		return empty, errors.New("operation_key_required")
	}
	root, admission, err := p.bindingRoot(bindingID)
	if err != nil {
		return empty, err
	}
	receipt, err := admission.Release(root, bindingID, request.ExpectedRevision, key)
	if err != nil {
		return empty, err
	}
	view, err := receipt.bindingView(receipt.SourceID, receipt.InstanceID)
	if err != nil {
		return empty, err
	}
	// The root is retained as a tombstone: reads stop resolving immediately, but a
	// replayed release with the original key can still reach the owner receipt.
	p.mu.Lock()
	if bound, ok := p.bindings[bindingID]; ok {
		bound.Released = true
		p.bindings[bindingID] = bound
	}
	p.mu.Unlock()
	return desktopcontract.BindingMutationReceipt{OperationID: receipt.OperationID, Action: "release", Binding: view}, nil
}

// Cancel is idempotent and affects only this session's pending selection or
// preview. It does not release managed protection or touch a vault.
func (p *Preflight) Cancel(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.selections, id)
	delete(p.previews, id)
}
func (p *Preflight) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	clear(p.selections)
	clear(p.previews)
	clear(p.bindings)
}
func (p *Preflight) prune(now time.Time) {
	for id, s := range p.selections {
		if !now.Before(s.expires) {
			delete(p.selections, id)
		}
	}
	for id, s := range p.previews {
		if !now.Before(s.selection.expires) {
			delete(p.previews, id)
		}
	}
}
func (s selection) recheck() error {
	current, err := directoryIdentity(s.path)
	if err != nil || !os.SameFile(s.identity, current) {
		return errors.New("native_selection_changed")
	}
	return nil
}
func clonePreview(view desktopcontract.BindingPreview) desktopcontract.BindingPreview {
	view.Scopes = append([]desktopcontract.ReaderScope(nil), view.Scopes...)
	return view
}
func opaqueID() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", errors.New("guard_unavailable")
	}
	return hex.EncodeToString(value[:]), nil
}

// Conservative metadata-only check. Refuse aliases in every path component.
// No claim of a durable filesystem lease: selection identity is checked again
// on consumption/confirmation; durable protection is still decided by the owner.
func directoryIdentity(path string) (os.FileInfo, error) {
	if path == "" || len(path) > 32768 || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("native_selection_invalid")
	}
	var root os.FileInfo
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("native_selection_invalid")
		}
		if root == nil {
			root = info
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return root, nil
}

func readerScopeNames(scopes []desktopcontract.ReaderScope) []string {
	names := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		names = append(names, string(scope))
	}
	return names
}
