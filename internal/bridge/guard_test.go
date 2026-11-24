package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// guardFixture builds a real state directory, a real space root and a real
// Service. fenced migrates the disposable state through the owner protocol so
// the guard sees the layout it requires.
type guardFixture struct {
	state  string
	root   string
	space  string
	svc    *Service
	t      *testing.T
	fenced bool
}

func newGuardFixture(t *testing.T) *guardFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	base := t.TempDir()
	f := &guardFixture{t: t, state: filepath.Join(base, "state"), root: filepath.Join(base, "vault"), space: "space-1"}
	if err := os.MkdirAll(f.root, 0700); err != nil {
		t.Fatal(err)
	}
	svc, err := New(f.state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.DB.Create(&Space{ID: f.space, Remote: "remote", LocalRoot: f.root, MaxBytes: 1 << 20}).Error; err != nil {
		svc.Close()
		t.Fatal(err)
	}
	if err = svc.DB.Create(&AdoptRecord{ID: "guard-owner", Consumer: "pinax", Kind: "local", SpaceID: f.space, Location: f.root, IdempotencyKey: "guard-owner"}).Error; err != nil {
		svc.Close()
		t.Fatal(err)
	}
	svc.Close()
	f.svc = nil
	return f
}

// open starts the owner on the current layout. Fixture construction leaves the
// state closed so the fence migration can take the original process lock.
func (f *guardFixture) open() {
	f.t.Helper()
	svc, err := New(f.state, NewOneDrive(), false)
	if err != nil {
		f.t.Fatal(err)
	}
	f.svc = svc
}

// fence runs the owner state migration on the disposable state. It is the only
// supported precondition for guard installation; a legacy state cannot prove an
// older writer is rejected.
func (f *guardFixture) fence() {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.state, "process.lock"), nil, 0600); err != nil {
		f.t.Fatal(err)
	}
	plan, err := managedstate.Prepare(f.state, "cli", "migration-original")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err = managedstate.Install(f.state, plan); err != nil {
		f.t.Fatal(err)
	}
	f.open()
	f.fenced = true
}

func (f *guardFixture) preview() GuardPreview {
	f.t.Helper()
	preview, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}})
	if err != nil {
		f.t.Fatalf("preview: %v", err)
	}
	return preview
}

func (f *guardFixture) install(preview GuardPreview, key string) (GuardReceipt, error) {
	return f.svc.InstallGuard(GuardInstallRequest{PreviewID: preview.PreviewID, Digest: preview.Digest, BindingID: preview.BindingID, ExpectedGuardRevision: preview.ExpectedGuardRevision, InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: preview.Scopes, Confirmed: true, IdempotencyKey: key})
}

// A legacy layout cannot prove that an older published writer is refused before
// it writes, so preflight and installation both stay closed. This is the L1
// precondition in managed-vault-guard-protocol.md, not a policy preference.
func TestGuardRequiresFencedState(t *testing.T) {
	f := newGuardFixture(t)
	f.open()
	defer f.svc.Close()
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("legacy preview: %v", err)
	}
}

func TestGuardPreviewReadyAndCASInstall(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	if preview.Decision != "ready" || preview.Reason != "none" || preview.ExpectedGuardRevision != 0 {
		t.Fatalf("preview: %#v", preview)
	}
	if preview.PendingMutations != 0 || preview.OverlappingRoots != 0 {
		t.Fatalf("counts: %#v", preview)
	}
	// A tampered digest must never install: the CAS anchor covers the exact
	// domain facts the preview observed.
	tampered := preview
	tampered.Digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := f.install(tampered, "key-tampered"); err == nil || err.Error() != "binding_stale" {
		t.Fatalf("tampered install: %v", err)
	}
	if facts, found, err := f.svc.Guard(preview.BindingID); err != nil || found {
		t.Fatalf("tampered install wrote a guard: %#v %v %v", facts, found, err)
	}
	receipt, err := f.install(preview, "key-install")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.State != "active" || receipt.Binding.Revision != 1 || !receipt.Binding.ProtectionActive {
		t.Fatalf("binding: %#v", receipt.Binding)
	}
	if receipt.OperationID == "" || receipt.Replayed {
		t.Fatalf("receipt: %#v", receipt)
	}
}

func TestGuardInstallReplayKeepsOriginalOperation(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	first, err := f.install(preview, "key-once")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.install(preview, "key-once")
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.OperationID != first.OperationID {
		t.Fatalf("replay created a second operation: %#v vs %#v", replay, first)
	}
	if replay.Binding.Revision != first.Binding.Revision {
		t.Fatalf("replay bumped the revision: %d vs %d", replay.Binding.Revision, first.Binding.Revision)
	}
}

func TestGuardInstallRejectsStaleRevision(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	if _, err := f.install(preview, "key-first"); err != nil {
		t.Fatal(err)
	}
	// The same preview is now stale: a second writer must not overwrite the
	// revision it never observed.
	if _, err := f.install(preview, "key-second"); err == nil || err.Error() != "binding_stale" {
		t.Fatalf("stale install: %v", err)
	}
	fresh := f.preview()
	if fresh.ExpectedGuardRevision != 1 {
		t.Fatalf("expected revision after install: %#v", fresh)
	}
	if _, err := f.install(fresh, "key-second"); err != nil {
		t.Fatalf("re-previewed install: %v", err)
	}
	if facts, _, _ := f.svc.Guard(preview.BindingID); facts.Revision != 2 {
		t.Fatalf("revision after second install: %#v", facts)
	}
}

func TestGuardPendingOperationBlocksInstallAndRelease(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	if _, err := f.install(preview, "key-install"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DB.Create(&Operation{ID: "op-pending", Key: "op-pending-key", SpaceID: f.space, Kind: "upload", Status: "queued"}).Error; err != nil {
		t.Fatal(err)
	}
	blocked := f.preview()
	if blocked.Decision != "blocked" || blocked.Reason != "managed_scope_busy" || blocked.PendingMutations != 1 {
		t.Fatalf("busy preview: %#v", blocked)
	}
	if _, err := f.install(blocked, "key-busy"); err == nil || err.Error() != "managed_scope_busy" {
		t.Fatalf("busy install: %v", err)
	}
	if _, err := f.svc.ReleaseGuard(GuardRequest{BindingID: blocked.BindingID, ExpectedRevision: blocked.ExpectedGuardRevision, Confirmed: true, IdempotencyKey: "key-release"}); err == nil || err.Error() != "managed_scope_busy" {
		t.Fatalf("busy release: %v", err)
	}
}

func TestGuardHideKeepsProtectionAndReleaseRevokes(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	if _, err := f.install(preview, "key-install"); err != nil {
		t.Fatal(err)
	}
	hidden, err := f.svc.SetGuardVisibility(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 1, Hidden: true, IdempotencyKey: "key-hide"})
	if err != nil {
		t.Fatal(err)
	}
	if hidden.Binding.State != "hidden" || !hidden.Binding.ProtectionActive {
		t.Fatalf("hide released protection: %#v", hidden.Binding)
	}
	shown, err := f.svc.SetGuardVisibility(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 2, Hidden: false, IdempotencyKey: "key-show"})
	if err != nil {
		t.Fatal(err)
	}
	if shown.Binding.State != "active" || !shown.Binding.ProtectionActive {
		t.Fatalf("show: %#v", shown.Binding)
	}
	// Release requires an explicit confirmation; a caller assertion alone is
	// not authorization.
	if _, err := f.svc.ReleaseGuard(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 3, IdempotencyKey: "key-release-unconfirmed"}); err == nil || err.Error() != "invalid_protection_release" {
		t.Fatalf("unconfirmed release: %v", err)
	}
	released, err := f.svc.ReleaseGuard(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 3, Confirmed: true, IdempotencyKey: "key-release"})
	if err != nil {
		t.Fatal(err)
	}
	if released.Binding.State != "released" || released.Binding.ProtectionActive {
		t.Fatalf("release: %#v", released.Binding)
	}
	facts, found, err := f.svc.Guard(preview.BindingID)
	if err != nil || !found || facts.Epoch != 2 {
		t.Fatalf("epoch not advanced to invalidate sessions: %#v %v %v", facts, found, err)
	}
	// Releasing a binding never permits owner metadata writes: the adopt guard
	// is independent of the guard row.
	if err := f.svc.rejectOwnerProtected(f.space, ".pinax/notes.md"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("released guard allowed owner metadata write: %v", err)
	}
}

func TestGuardDomainRejectsOverlappingScope(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	inner := filepath.Join(f.root, "nested")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DB.Create(&Space{ID: "space-2", Remote: "remote2", LocalRoot: inner, MaxBytes: 1 << 20}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DB.Create(&Connection{ID: "conn-2", Label: "nested", BaseURL: "http://127.0.0.1:1", InstanceID: "instance-1", SpaceID: "space-2", CredentialSource: "env"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("overlapping scope accepted: %v", err)
	}
}

func TestGuardRootReplacementInvalidatesPreview(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	spare := filepath.Join(filepath.Dir(f.root), "vault-other")
	if err := os.MkdirAll(spare, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(spare, f.root); err != nil {
		t.Fatal(err)
	}
	if _, err := f.install(preview, "key-install"); err == nil || err.Error() != "binding_stale" {
		t.Fatalf("replaced root accepted: %v", err)
	}
}

// A fenced state keeps the original path as an inert marker and the live
// database under the managed snapshot directory. The current owner opens that
// layout; a writer that does not understand it cannot open a database at the
// marker path at all (the published-binary proof remains D03.5).
func TestFencedStateOpensManagedDatabase(t *testing.T) {
	f := newGuardFixture(t)
	f.open()
	if err := f.svc.DB.Create(&Operation{ID: "original-operation", Key: "original-key", SpaceID: f.space, Kind: "upload", Status: "interrupted"}).Error; err != nil {
		f.svc.Close()
		t.Fatal(err)
	}
	f.svc.Close()
	f.fence()
	defer f.svc.Close()
	marker, err := os.ReadFile(filepath.Join(f.state, "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := managedstate.Inspect(f.state, "cli")
	if err != nil || layout.Layout != "fenced" {
		t.Fatalf("layout: %#v %v", layout, err)
	}
	after, err := os.ReadFile(filepath.Join(f.state, "state.sqlite"))
	if err != nil || string(after) != string(marker) {
		t.Fatal("owner startup rewrote the state marker")
	}
	var operation Operation
	if err := f.svc.DB.First(&operation, "id = ?", "original-operation").Error; err != nil || operation.Key != "original-key" {
		t.Fatalf("preserved operation identity: %#v %v", operation, err)
	}
	rows, err := os.ReadDir(filepath.Join(f.state, ".managed-state-v1"))
	if err != nil || len(rows) == 0 {
		t.Fatalf("snapshot directory: %v %v", rows, err)
	}
}
