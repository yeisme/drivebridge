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
}

func NewPreflight() *Preflight {
	return &Preflight{now: time.Now, selections: make(map[string]selection), previews: make(map[string]preview)}
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
	view := desktopcontract.BindingPreview{PreviewID: id, PreviewDigest: "sha256:" + hex.EncodeToString(digest[:]), DisplayName: "Selected local folder", Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}, Decision: "blocked", Reason: "managed_scope_unverifiable", ObservedAt: now.UTC(), ExpiresAt: selected.expires.UTC()}
	if err := view.Validate(); err != nil {
		return empty, errors.New("desktop_contract_unsupported")
	}
	p.previews[id] = preview{selection: selected, view: view}
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
// on consumption/confirmation and all confirmations remain blocked.
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
