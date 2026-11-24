package bridge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"gorm.io/gorm"
)

// Managed-vault guard states. Only `released` clears ProtectionActive on the
// binding projection; it revokes DriveBridge binding/session authority and does
// NOT permit writing owner metadata. Owner-protected paths stay rejected for
// adopted spaces whether or not a guard row exists.
const (
	GuardActive    = "active"
	GuardHidden    = "hidden"
	GuardReleasing = "releasing"
	GuardReleased  = "released"
)

// GuardMinWriterVersion is a declared fact: the state layout a writer must
// understand. The mechanism that actually rejects an older published writer is
// the state fence (managedstate), not this string.
const GuardMinWriterVersion = "drivebridge.managed-state.v1"

// Install and release refuse unknown/nonterminal operation states. The bridge
// worker writes completed; succeeded is retained for existing owner receipts.
var guardTerminalStatuses = []string{"completed", "succeeded", "failed", "cancelled"}

// ManagedGuard is the CAS anchor for one protected binding. It is owner-private
// persistence: absolute roots, device/inode values and scope internals never
// cross the renderer contract.
type ManagedGuard struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	InstanceID       string     `json:"instance_id"`
	SourceID         string     `json:"source_id"`
	SpaceID          string     `json:"space_id" gorm:"index"`
	Revision         uint64     `json:"revision"`
	Epoch            uint64     `json:"epoch"`
	State            string     `json:"state"`
	MinWriterVersion string     `json:"min_writer_version"`
	Scopes           string     `json:"-"`
	RootDigest       string     `json:"-"`
	ContentDigest    string     `json:"-"`
	InstalledAt      time.Time  `json:"installed_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ReleasedAt       *time.Time `json:"released_at,omitempty"`
}

// ManagedGuardMember freezes the protection-domain enumeration that install
// verified. Rechecks compare against these rows instead of re-deriving a
// narrower domain from a caller request.
type ManagedGuardMember struct {
	GuardID string `json:"-" gorm:"primaryKey"`
	Kind    string `json:"kind" gorm:"primaryKey"`
	Ref     string `json:"ref" gorm:"primaryKey"`
	Digest  string `json:"-"`
}

// ManagedGuardReceipt binds one confirm/release to its original operation so a
// replay returns the same receipt instead of creating a second operation.
type ManagedGuardReceipt struct {
	Key         string    `json:"-" gorm:"primaryKey"`
	GuardID     string    `json:"-" gorm:"index"`
	Action      string    `json:"action"`
	OperationID string    `json:"operation_id"`
	Revision    uint64    `json:"revision"`
	CreatedAt   time.Time `json:"created_at"`
}

// GuardPreviewRequest is the application-service input for a protection-domain
// preflight. SpaceID is an internal reference, never a renderer path.
type GuardPreviewRequest struct {
	InstanceID string
	SourceID   string
	SpaceID    string
	Scopes     []desktopcontract.ReaderScope
}

// GuardPreview is a redacted preflight projection. Counts describe this domain
// only; zero does not mean "no other writer exists".
type GuardPreview struct {
	PreviewID             string
	Digest                string
	BindingID             string
	ExpectedGuardRevision uint64
	Decision              string
	Reason                string
	OverlappingRoots      uint32
	PendingMutations      uint32
	Scopes                []desktopcontract.ReaderScope
}

// GuardInstallRequest confirms a preview and installs the guard under CAS.
type GuardInstallRequest struct {
	PreviewID             string
	Digest                string
	BindingID             string
	ExpectedGuardRevision uint64
	InstanceID            string
	SourceID              string
	SpaceID               string
	Scopes                []desktopcontract.ReaderScope
	Confirmed             bool
	IdempotencyKey        string
}

// GuardReceipt is the closed result of a confirm/release mutation.
type GuardReceipt struct {
	OperationID string
	Action      string
	Binding     desktopcontract.BindingView
	Replayed    bool
	Epoch       uint64
}

// GuardRequest is a CAS-guarded visibility or release mutation.
type GuardRequest struct {
	BindingID        string
	ExpectedRevision uint64
	Confirmed        bool
	Hidden           bool
	IdempotencyKey   string
}

// rejectStateRootOverlap refuses owner-state writes that share a tree with one
// protection-domain root, in either direction. The fenced database, its inert
// marker, the `.managed-state-v1` snapshots and the preview cache are all
// written by this application, so nesting state and a protected root would make
// app writes land in the owner's tree through a path that is not a
// space-relative target. `RegisterReaderRoot` already applies this same
// symmetric rule.
func (s *Service) rejectStateRootOverlap(root string) error {
	state, err := canonicalMutationPath(s.state)
	if err != nil {
		return err
	}
	if same, stateInRoot, rootInState := sameOrNested(state, root); same || stateInRoot || rootInState {
		return errors.New("owner_protected")
	}
	return nil
}

// rejectStateRootAlias refuses owner-state writes when the DriveBridge state
// directory sits inside an adopted local root. The preview cache then lives in
// the vault, so writing it would mutate the owner's tree through a path that is
// not a space-relative target.
func (s *Service) rejectStateRootAlias() error {
	if s == nil || s.DB == nil || s.state == "" {
		return errors.New("guard_unavailable")
	}
	var records []AdoptRecord
	if err := s.DB.Where("kind = ?", "local").Find(&records).Error; err != nil {
		return errors.New("guard_unavailable")
	}
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		root, err := canonicalMutationPath(record.Location)
		if err != nil {
			return err
		}
		if err := s.rejectStateRootOverlap(root); err != nil {
			return err
		}
	}
	return nil
}

// GuardFacts is the redacted read projection used by host/CLI surfaces.
type GuardFacts struct {
	BindingID        string
	InstanceID       string
	SourceID         string
	State            string
	Revision         uint64
	Epoch            uint64
	ProtectionActive bool
	MinWriterVersion string
	Scopes           []desktopcontract.ReaderScope
	UpdatedAt        time.Time
}

// stateDatabase resolves the owner database path for the current layout. A
// fresh state has no database yet and keeps the legacy path. A state created by
// a current writer is fenced: the original path holds an inert marker and the
// live database lives under the managed snapshot directory, so a writer that
// does not understand the layout cannot open it at all. This is the L1 condition
// that guard installation depends on; it does not itself grant a binding.
func stateDatabase(state string) (string, error) {
	legacy := filepath.Join(state, "state.sqlite")
	_, err := os.Lstat(legacy)
	if errors.Is(err, os.ErrNotExist) {
		return legacy, nil
	}
	if err != nil {
		return "", errors.New("state_unavailable")
	}
	layout, err := managedstate.Inspect(state, "cli")
	if err != nil {
		// Unreadable, aliased or non-canonical layouts are refused instead of
		// being repaired or silently re-created as an empty database.
		return "", errors.New("state_unavailable")
	}
	switch layout.Layout {
	case "legacy", "fenced":
		return layout.DatabasePath, nil
	default:
		return "", errors.New("state_layout_invalid")
	}
}

func newGuardID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return "guard-" + hex.EncodeToString(b[:])
}

// digestGuardFacts hashes only stable, comparable facts. It deliberately
// excludes timestamps and display names so a preview and a later confirm agree
// on identity while still failing after any real domain change.
func digestGuardFacts(state string, revision uint64, rootDigest, contentDigest string, members []ManagedGuardMember, scopes []desktopcontract.ReaderScope) string {
	parts := []string{state, "rev=" + guardUint(revision), "root=" + rootDigest, "content=" + contentDigest}
	rows := make([]string, 0, len(members))
	for _, member := range members {
		rows = append(rows, member.Kind+"|"+member.Ref+"|"+member.Digest)
	}
	sort.Strings(rows)
	parts = append(parts, rows...)
	scopeIDs := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scopeIDs = append(scopeIDs, string(scope))
	}
	sort.Strings(scopeIDs)
	parts = append(parts, "scopes="+strings.Join(scopeIDs, ","))
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func guardUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[pos:])
}

// guardDomain enumerates the protection domain that a binding would cover.
// Anything it cannot prove becomes an error, never a zero count.
func (s *Service) guardDomain(req GuardPreviewRequest) (rootDigest, contentDigest string, members []ManagedGuardMember, pending uint32, err error) {
	if s == nil || s.DB == nil || s.state == "" {
		return "", "", nil, 0, errors.New("guard_unavailable")
	}
	// The owner state must live outside the protection domain. The fenced
	// database, its inert marker and every `.managed-state-v1` snapshot are
	// written by this application, so a state directory inside an adopted root
	// would mutate the vault through a path that is not an admitted space
	// target. The same predicate already gates the preview cache at R1; a guard
	// preflight or confirm must not claim a domain it would itself write into.
	if err := s.rejectStateRootAlias(); err != nil {
		return "", "", nil, 0, err
	}
	if strings.TrimSpace(req.SpaceID) == "" {
		return "", "", nil, 0, errors.New("managed_scope_unverifiable")
	}
	p, err := s.Space(req.SpaceID)
	if err != nil {
		return "", "", nil, 0, errors.New("managed_scope_unverifiable")
	}
	canonical, err := canonicalMutationPath(filepath.Clean(p.LocalRoot))
	if err != nil {
		return "", "", nil, 0, err
	}
	rootDigest, err = guardRootIdentity(canonical)
	if err != nil {
		return "", "", nil, 0, err
	}
	contentDigest = "none"
	if p.Kind() == "generic" {
		remote, aliasErr := AliasContentRoot(p.Remote)
		if aliasErr != nil {
			return "", "", nil, 0, errors.New("managed_scope_unverifiable")
		}
		resolved, canonicalErr := canonicalMutationPath(filepath.Join(remote, p.RootID))
		if canonicalErr != nil {
			return "", "", nil, 0, canonicalErr
		}
		// The content root is a second physical root of the same domain and is not
		// an AdoptRecord, so the record-based predicate above cannot see a state
		// inside the alias target. Both claimed roots are checked.
		if err = s.rejectStateRootOverlap(resolved); err != nil {
			return "", "", nil, 0, err
		}
		if contentDigest, err = guardRootIdentity(resolved); err != nil {
			return "", "", nil, 0, err
		}
	}
	// The bound root itself is claimed whether or not an adoption record points at
	// it, so the state must be outside it too.
	if err = s.rejectStateRootOverlap(canonical); err != nil {
		return "", "", nil, 0, err
	}
	// Registered local adoptions and writable connections that can reach this
	// root are part of the domain, including a different space's alias.
	var adopts []AdoptRecord
	if err = s.DB.Where("kind = ?", "local").Find(&adopts).Error; err != nil {
		return "", "", nil, 0, errors.New("guard_unavailable")
	}
	ownerAdopted := false
	for _, record := range adopts {
		owner, canonicalErr := canonicalMutationPath(record.Location)
		if canonicalErr != nil {
			return "", "", nil, 0, canonicalErr
		}
		same, inside, ancestor := sameOrNested(canonical, owner)
		if !same && !inside && !ancestor {
			continue
		}
		digest, identityErr := guardRootIdentity(owner)
		if identityErr != nil {
			return "", "", nil, 0, identityErr
		}
		if same {
			if record.SpaceID == p.ID && record.Consumer == "pinax" {
				ownerAdopted = true
			}
			continue // the bound root itself is represented by rootDigest
		}
		members = append(members, ManagedGuardMember{Kind: "adopt", Ref: record.ID, Digest: digest})
	}
	// A registered file root is not an owner adoption. Guard installation must
	// retain the metadata shield after release, rather than creating a binding
	// whose protection vanishes with the guard row.
	if !ownerAdopted {
		return "", "", nil, 0, errors.New("managed_scope_unverifiable")
	}
	if len(members) > 0 {
		return "", "", nil, 0, errors.New("managed_scope_unverifiable")
	}
	var connections []Connection
	if err = s.DB.Find(&connections).Error; err != nil {
		return "", "", nil, 0, errors.New("guard_unavailable")
	}
	for _, connection := range connections {
		if connection.SpaceID == "" || connection.SpaceID == p.ID {
			continue
		}
		other, spaceErr := s.Space(connection.SpaceID)
		if spaceErr != nil {
			return "", "", nil, 0, errors.New("managed_scope_unverifiable")
		}
		otherCanonical, canonicalErr := canonicalMutationPath(filepath.Clean(other.LocalRoot))
		if canonicalErr != nil {
			return "", "", nil, 0, canonicalErr
		}
		same, inside, ancestor := sameOrNested(canonical, otherCanonical)
		if !same && !inside && !ancestor {
			continue
		}
		digest, identityErr := guardRootIdentity(otherCanonical)
		if identityErr != nil {
			return "", "", nil, 0, identityErr
		}
		members = append(members, ManagedGuardMember{Kind: "connection", Ref: connection.ID, Digest: digest})
	}
	if len(members) > 0 {
		return "", "", nil, 0, errors.New("managed_scope_unverifiable")
	}
	var inFlight int64
	if err = s.DB.Model(&Operation{}).Where("space_id = ? AND status NOT IN ?", p.ID, guardTerminalStatuses).Limit(1).Count(&inFlight).Error; err != nil {
		return "", "", nil, 0, errors.New("guard_unavailable")
	}
	return rootDigest, contentDigest, members, uint32(inFlight), nil
}

// rejectGuardMutation protects the complete installed domain, beyond the legacy
// adoption metadata filter. It is shared by provider and local write gates so
// another space's alias or an ancestor action cannot bypass a bound vault.
func (s *Service) rejectGuardMutation(spaceID string, local bool, rels ...string) error {
	if s == nil || s.DB == nil {
		return errors.New("guard_unavailable")
	}
	var guards []ManagedGuard
	if err := s.DB.Where("state <> ?", GuardReleased).Find(&guards).Error; err != nil {
		return errors.New("guard_unavailable")
	}
	if len(guards) == 0 {
		return nil
	}
	p, err := s.Space(spaceID)
	if err != nil {
		return errors.New("guard_unavailable")
	}
	base := p.LocalRoot
	if !local && p.Kind() == "generic" {
		remote, err := AliasContentRoot(p.Remote)
		if err != nil {
			return errors.New("guard_unavailable")
		}
		base = filepath.Join(remote, p.RootID)
	}
	if len(rels) == 0 {
		rels = []string{""}
	}
	for _, guard := range guards {
		if guard.SpaceID == spaceID {
			return errors.New("owner_protected")
		}
		if !local && p.Kind() != "generic" {
			continue // the provider writes a remote object, not a local alias
		}
		owner, err := s.Space(guard.SpaceID)
		if err != nil {
			return errors.New("guard_unavailable")
		}
		root, err := canonicalMutationPath(owner.LocalRoot)
		if err != nil {
			return err
		}
		identity, err := guardRootIdentity(root)
		if err != nil || identity != guard.RootDigest {
			return errors.New("guard_unavailable")
		}
		roots := []string{root}
		if owner.Kind() == "generic" {
			remote, err := AliasContentRoot(owner.Remote)
			if err != nil {
				return errors.New("guard_unavailable")
			}
			content, err := canonicalMutationPath(filepath.Join(remote, owner.RootID))
			if err != nil {
				return err
			}
			identity, err := guardRootIdentity(content)
			if err != nil || identity != guard.ContentDigest {
				return errors.New("guard_unavailable")
			}
			roots = append(roots, content)
		}
		for _, rel := range rels {
			name := filepath.Join(base, rel)
			target, err := canonicalMutationPath(name)
			if err != nil {
				return err
			}
			for _, protected := range roots {
				// Both names matter: an in-vault symlink pointing out must not
				// permit removing or renaming its protected directory entry.
				for _, candidate := range []string{name, target} {
					same, inside, ancestor := sameOrNested(candidate, protected)
					if same || inside || ancestor {
						return errors.New("owner_protected")
					}
				}
			}
		}
	}
	return nil
}

// PreviewGuard computes a metadata-only preflight. It never persists, never
// creates a directory and never grants a session.
func (s *Service) PreviewGuard(req GuardPreviewRequest) (GuardPreview, error) {
	var empty GuardPreview
	if req.InstanceID == "" || req.SourceID == "" {
		return empty, errors.New("managed_scope_unverifiable")
	}
	if !validGuardScopes(req.Scopes) {
		return empty, errors.New("invalid_reader_scopes")
	}
	rootDigest, contentDigest, members, pending, err := s.guardDomain(req)
	if err != nil {
		return empty, err
	}
	state, err := managedstate.Inspect(s.state, "cli")
	if err != nil || state.Layout != "fenced" {
		// A legacy state cannot prove that an older published writer is
		// rejected before it writes, so the domain stays unverifiable.
		return empty, errors.New("managed_scope_unverifiable")
	}
	bindingID := newGuardID()
	if bindingID == "" {
		return empty, errors.New("guard_unavailable")
	}
	existing, found, err := s.guardBySource(req.InstanceID, req.SourceID)
	if err != nil {
		return empty, err
	}
	var revision uint64
	if found {
		bindingID = existing.ID
		revision = existing.Revision
	}
	digest := digestGuardFacts("preview", revision, rootDigest, contentDigest, members, req.Scopes)
	preview := GuardPreview{PreviewID: bindingID, Digest: digest, BindingID: bindingID, ExpectedGuardRevision: revision, Scopes: req.Scopes, PendingMutations: pending}
	if pending > 0 {
		preview.Decision, preview.Reason = "blocked", "managed_scope_busy"
		return preview, nil
	}
	preview.Decision, preview.Reason = "ready", "none"
	return preview, nil
}

// validGuardScopes defers to the contract's closed reader set. A guard row stores
// the scopes a bound session may later request, and `guardScopesCover` admits any
// scope it finds there, so a weaker local rule (non-empty is not enough) would
// persist an undefined scope and then grant it.
func validGuardScopes(scopes []desktopcontract.ReaderScope) bool {
	return desktopcontract.ValidReaderScopes(scopes)
}

func (s *Service) guardBySource(instanceID, sourceID string) (ManagedGuard, bool, error) {
	var row ManagedGuard
	err := s.DB.Where("instance_id = ? AND source_id = ?", instanceID, sourceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ManagedGuard{}, false, nil
	}
	if err != nil {
		return ManagedGuard{}, false, errors.New("guard_unavailable")
	}
	return row, true, nil
}

// Guard returns the redacted facts for one binding, if it exists.
func (s *Service) Guard(bindingID string) (GuardFacts, bool, error) {
	var row ManagedGuard
	err := s.DB.Where("id = ?", bindingID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GuardFacts{}, false, nil
	}
	if err != nil {
		return GuardFacts{}, false, errors.New("guard_unavailable")
	}
	return guardFactsOf(row), true, nil
}

// AcquireGuardLease revalidates a binding immediately before an owner session
// starts and holds the guard lock until that bounded call finishes. Release and
// visibility mutations use the same lock, so a release cannot pass its
// in-flight check while a reader or writer is still using the binding. The
// physical root is checked again to close the rename/replace window between
// preflight and the owner call.
// ponytail: one instance-wide lock serializes bounded owner calls with release;
// use per-binding leases only if desktop throughput requires concurrent calls.
func guardScopesCover(installed string, requested []string) bool {
	if len(requested) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, scope := range strings.Split(installed, ",") {
		if scope != "" {
			have[scope] = true
		}
	}
	for _, scope := range requested {
		if !have[scope] {
			return false
		}
	}
	return true
}

func (s *Service) AcquireGuardLease(bindingID, instanceID, sourceID, root string, epoch uint64, scopes []string) (func() error, error) {
	if s == nil || s.DB == nil || strings.TrimSpace(bindingID) == "" || strings.TrimSpace(root) == "" {
		return nil, errors.New("guard_unavailable")
	}
	s.guardMu.Lock()
	check := func() error {
		row, err := s.guardRow(bindingID)
		if err != nil {
			return err
		}
		// epoch 属于确认时的授权，release 后重新绑定也不能复活旧会话。
		if epoch == 0 || row.Epoch != epoch || row.InstanceID != instanceID || row.SourceID != sourceID || (row.State != GuardActive && row.State != GuardHidden) {
			return errors.New("protection_released")
		}
		if !guardScopesCover(row.Scopes, scopes) {
			return errors.New("session_request_invalid")
		}
		canonical, err := canonicalMutationPath(root)
		if err != nil {
			return err
		}
		digest, err := guardRootIdentity(canonical)
		if err != nil || digest != row.RootDigest {
			return errors.New("binding_stale")
		}
		currentRoot, currentContent, _, _, err := s.guardDomain(GuardPreviewRequest{SpaceID: row.SpaceID})
		if err != nil {
			return err
		}
		if currentRoot != row.RootDigest || currentContent != row.ContentDigest {
			return errors.New("binding_stale")
		}
		layout, err := managedstate.Inspect(s.state, "cli")
		if err != nil || layout.Layout != "fenced" {
			return errors.New("guard_unavailable")
		}
		return nil
	}
	if err := check(); err != nil {
		s.guardMu.Unlock()
		return nil, err
	}
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			defer s.guardMu.Unlock()
			result = check()
		})
		return result
	}, nil
}

func guardFactsOf(row ManagedGuard) GuardFacts {
	scopes := make([]desktopcontract.ReaderScope, 0, 4)
	for _, scope := range strings.Split(row.Scopes, ",") {
		if scope != "" {
			scopes = append(scopes, desktopcontract.ReaderScope(scope))
		}
	}
	return GuardFacts{BindingID: row.ID, InstanceID: row.InstanceID, SourceID: row.SourceID, State: row.State, Revision: row.Revision, Epoch: row.Epoch, ProtectionActive: row.State != GuardReleased, MinWriterVersion: row.MinWriterVersion, Scopes: scopes, UpdatedAt: row.UpdatedAt}
}

func guardBindingView(row ManagedGuard) (desktopcontract.BindingView, error) {
	scopes := make([]string, 0, 4)
	for _, scope := range strings.Split(row.Scopes, ",") {
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	_ = scopes
	view := desktopcontract.BindingView{BindingID: row.ID, SourceID: row.SourceID, InstanceID: row.InstanceID, State: row.State, Revision: row.Revision, GuardRevision: row.Revision, ProtectionActive: row.State != GuardReleased, UpdatedAt: row.UpdatedAt}
	if err := view.Validate(); err != nil {
		return desktopcontract.BindingView{}, errors.New("guard_state_invalid")
	}
	return view, nil
}

func (s *Service) guardReceipt(key, guardID string) (ManagedGuardReceipt, bool, error) {
	var row ManagedGuardReceipt
	err := s.DB.Where("key = ? AND guard_id = ?", key, guardID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ManagedGuardReceipt{}, false, nil
	}
	if err != nil {
		return ManagedGuardReceipt{}, false, errors.New("guard_unavailable")
	}
	return row, true, nil
}

// InstallGuard installs or refreshes a guard inside one write transaction. The
// guard revision is the CAS anchor: a stale expected revision never rewrites the
// row, never guesses that the previous writer succeeded, and keeps the original
// operation identity for the caller's reconcile.
func (s *Service) InstallGuard(req GuardInstallRequest) (GuardReceipt, error) {
	var empty GuardReceipt
	if !req.Confirmed {
		return empty, errors.New("binding_confirmation_required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return empty, errors.New("operation_key_required")
	}
	if !validGuardScopes(req.Scopes) || strings.TrimSpace(req.BindingID) == "" {
		return empty, errors.New("invalid_binding_confirmation")
	}
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	if replay, found, err := s.guardReceipt(req.IdempotencyKey, req.BindingID); err != nil {
		return empty, err
	} else if found {
		row, rowErr := s.guardRow(replay.GuardID)
		if rowErr != nil {
			return empty, rowErr
		}
		view, viewErr := guardBindingView(row)
		if viewErr != nil {
			return empty, viewErr
		}
		return GuardReceipt{OperationID: replay.OperationID, Action: replay.Action, Binding: view, Replayed: true, Epoch: row.Epoch}, nil
	}
	rootDigest, contentDigest, members, pending, err := s.guardDomain(GuardPreviewRequest{InstanceID: req.InstanceID, SourceID: req.SourceID, SpaceID: req.SpaceID, Scopes: req.Scopes})
	if err != nil {
		return empty, err
	}
	if pending != 0 {
		return empty, errors.New("managed_scope_busy")
	}
	state, err := managedstate.Inspect(s.state, "cli")
	if err != nil || state.Layout != "fenced" {
		return empty, errors.New("managed_scope_unverifiable")
	}
	digest := digestGuardFacts("preview", req.ExpectedGuardRevision, rootDigest, contentDigest, members, req.Scopes)
	if digest != req.Digest {
		return empty, errors.New("binding_stale")
	}
	now := time.Now().UTC()
	row, found, err := s.guardBySource(req.InstanceID, req.SourceID)
	if err != nil {
		return empty, err
	}
	operationID := req.IdempotencyKey + "@" + req.BindingID + "@" + guardUint(req.ExpectedGuardRevision+1)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if found {
			// CAS: compare-and-swap on the persisted revision. A closed
			// transaction on a stale revision rolls back the whole install.
			result := tx.Model(&ManagedGuard{}).Where("id = ? AND revision = ?", row.ID, req.ExpectedGuardRevision).Updates(map[string]any{"revision": req.ExpectedGuardRevision + 1, "state": GuardActive, "scopes": joinScopeIDs(req.Scopes), "min_writer_version": GuardMinWriterVersion, "root_digest": rootDigest, "content_digest": contentDigest, "updated_at": now})
			if result.Error != nil {
				return errors.New("guard_unavailable")
			}
			if result.RowsAffected != 1 {
				return errors.New("binding_stale")
			}
			if err := tx.Where("guard_id = ?", row.ID).Delete(&ManagedGuardMember{}).Error; err != nil {
				return errors.New("guard_unavailable")
			}
		} else {
			if req.ExpectedGuardRevision != 0 {
				return errors.New("binding_stale")
			}
			created := ManagedGuard{ID: req.BindingID, InstanceID: req.InstanceID, SourceID: req.SourceID, SpaceID: req.SpaceID, Revision: 1, Epoch: 1, State: GuardActive, MinWriterVersion: GuardMinWriterVersion, Scopes: joinScopeIDs(req.Scopes), RootDigest: rootDigest, ContentDigest: contentDigest, InstalledAt: now, UpdatedAt: now}
			if err := tx.Create(&created).Error; err != nil {
				return errors.New("guard_unavailable")
			}
		}
		for _, member := range members {
			row := ManagedGuardMember{GuardID: req.BindingID, Kind: member.Kind, Ref: member.Ref, Digest: member.Digest}
			if err := tx.Create(&row).Error; err != nil {
				return errors.New("guard_unavailable")
			}
		}
		receipt := ManagedGuardReceipt{Key: req.IdempotencyKey, GuardID: req.BindingID, Action: "confirm", OperationID: operationID, Revision: req.ExpectedGuardRevision + 1, CreatedAt: now}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return empty, err
	}
	installed, err := s.guardRow(req.BindingID)
	if err != nil {
		return empty, err
	}
	view, err := guardBindingView(installed)
	if err != nil {
		return empty, err
	}
	return GuardReceipt{OperationID: operationID, Action: "confirm", Binding: view, Epoch: installed.Epoch}, nil
}

func (s *Service) guardRow(bindingID string) (ManagedGuard, error) {
	var row ManagedGuard
	if err := s.DB.Where("id = ?", bindingID).First(&row).Error; err != nil {
		return ManagedGuard{}, errors.New("guard_unavailable")
	}
	return row, nil
}

func joinScopeIDs(scopes []desktopcontract.ReaderScope) string {
	ids := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		ids = append(ids, string(scope))
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// SetGuardVisibility hides or restores navigation only. Hiding never releases
// protection: the state stays active/hidden and both report ProtectionActive.
func (s *Service) SetGuardVisibility(req GuardRequest) (GuardReceipt, error) {
	var empty GuardReceipt
	if req.ExpectedRevision == 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return empty, errors.New("invalid_source_visibility")
	}
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	row, err := s.guardRow(req.BindingID)
	if err != nil {
		return empty, err
	}
	if row.State == GuardReleased || row.State == GuardReleasing {
		return empty, errors.New("protection_released")
	}
	action := "show"
	next := GuardActive
	if req.Hidden {
		action, next = "hide", GuardHidden
	}
	if replay, found, err := s.guardReceipt(req.IdempotencyKey, row.ID); err != nil {
		return empty, err
	} else if found {
		view, viewErr := guardBindingView(row)
		if viewErr != nil {
			return empty, viewErr
		}
		return GuardReceipt{OperationID: replay.OperationID, Action: action, Binding: view, Replayed: true}, nil
	}
	now := time.Now().UTC()
	operationID := req.IdempotencyKey + "@" + row.ID + "@visibility"
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ManagedGuard{}).Where("id = ? AND revision = ?", row.ID, req.ExpectedRevision).Updates(map[string]any{"state": next, "revision": req.ExpectedRevision + 1, "updated_at": now})
		if result.Error != nil {
			return errors.New("guard_unavailable")
		}
		if result.RowsAffected != 1 {
			return errors.New("binding_stale")
		}
		receipt := ManagedGuardReceipt{Key: req.IdempotencyKey, GuardID: row.ID, Action: action, OperationID: operationID, Revision: req.ExpectedRevision + 1, CreatedAt: now}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return empty, err
	}
	updated, err := s.guardRow(row.ID)
	if err != nil {
		return empty, err
	}
	view, err := guardBindingView(updated)
	if err != nil {
		return empty, err
	}
	return GuardReceipt{OperationID: operationID, Action: action, Binding: view}, nil
}

// ReleaseGuard revokes binding/session authority. It requires explicit
// confirmation, a matching revision and no in-flight work in the domain, and it
// bumps Epoch so every issued session token stops validating. It never restores
// an old database and never deletes the state fence or the adopter registry.
func (s *Service) ReleaseGuard(req GuardRequest) (GuardReceipt, error) {
	var empty GuardReceipt
	if !req.Confirmed || req.ExpectedRevision == 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return empty, errors.New("invalid_protection_release")
	}
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	row, err := s.guardRow(req.BindingID)
	if err != nil {
		return empty, err
	}
	if replay, found, err := s.guardReceipt(req.IdempotencyKey, row.ID); err != nil {
		return empty, err
	} else if found {
		view, viewErr := guardBindingView(row)
		if viewErr != nil {
			return empty, viewErr
		}
		return GuardReceipt{OperationID: replay.OperationID, Action: "release", Binding: view, Replayed: true}, nil
	}
	if row.State == GuardReleased {
		view, viewErr := guardBindingView(row)
		if viewErr != nil {
			return empty, viewErr
		}
		return GuardReceipt{Action: "release", Binding: view, Replayed: true}, errors.New("protection_released")
	}
	var inFlight int64
	if err = s.DB.Model(&Operation{}).Where("space_id = ? AND status NOT IN ?", row.SpaceID, guardTerminalStatuses).Limit(1).Count(&inFlight).Error; err != nil {
		return empty, errors.New("guard_unavailable")
	}
	if inFlight > 0 {
		return empty, errors.New("managed_scope_busy")
	}
	now := time.Now().UTC()
	operationID := req.IdempotencyKey + "@" + row.ID + "@release"
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ManagedGuard{}).Where("id = ? AND revision = ?", row.ID, req.ExpectedRevision).Updates(map[string]any{"state": GuardReleased, "revision": req.ExpectedRevision + 1, "epoch": row.Epoch + 1, "released_at": now, "updated_at": now})
		if result.Error != nil {
			return errors.New("guard_unavailable")
		}
		if result.RowsAffected != 1 {
			return errors.New("binding_stale")
		}
		receipt := ManagedGuardReceipt{Key: req.IdempotencyKey, GuardID: row.ID, Action: "release", OperationID: operationID, Revision: req.ExpectedRevision + 1, CreatedAt: now}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		return empty, err
	}
	released, err := s.guardRow(row.ID)
	if err != nil {
		return empty, err
	}
	view, err := guardBindingView(released)
	if err != nil {
		return empty, err
	}
	return GuardReceipt{OperationID: operationID, Action: "release", Binding: view}, nil
}
