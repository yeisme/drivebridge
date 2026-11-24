package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"github.com/yeisme/drivebridge/internal/bridge"
)

type admissionFixture struct {
	root      string
	state     string
	preflight *Preflight
}

// newAdmissionService builds a real owner state, fences it through the owner
// protocol and registers one space for the selectable directory. Nothing here
// touches a user vault or a real deployment state. It returns the admission and
// the registered root so host-level tests can reuse the same domain.
func newAdmissionService(t *testing.T) (*BridgeAdmission, string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	base := t.TempDir()
	state := filepath.Join(base, "state")
	root := filepath.Join(base, "vault")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	svc, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.DB.Create(&bridge.Space{ID: "vault", Remote: "remote", LocalRoot: root, MaxBytes: 1 << 20}).Error; err != nil {
		svc.Close()
		t.Fatal(err)
	}
	if err = svc.DB.Create(&bridge.AdoptRecord{ID: "owner-vault", Consumer: "pinax", Kind: "local", SpaceID: "vault", Location: root, IdempotencyKey: "owner-vault"}).Error; err != nil {
		svc.Close()
		t.Fatal(err)
	}
	svc.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "cli", "migration-desktop-admission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		t.Fatal(err)
	}
	reopened, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	admission, err := NewBridgeAdmission(reopened)
	if err != nil {
		t.Fatal(err)
	}
	return admission, root
}

func newAdmissionFixture(t *testing.T) *admissionFixture {
	t.Helper()
	admission, root := newAdmissionService(t)
	return &admissionFixture{root: root, preflight: NewPreflightWithAdmission(admission)}
}

func (f *admissionFixture) preview(t *testing.T) desktopcontract.BindingPreview {
	t.Helper()
	selection, err := f.preflight.Select(f.root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.preflight.Preview(selection)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

// The host must reach a real `active` binding only through the owner guard, and
// the confirmation must consume the preview so it cannot be replayed into a
// second binding.
func TestDesktopAdmissionConfirmInstallsGuard(t *testing.T) {
	f := newAdmissionFixture(t)
	defer f.preflight.Close()
	view := f.preview(t)
	if view.Decision != "ready" || view.Reason != "none" || view.ExpectedGuardRevision != 0 {
		t.Fatalf("admission preview: %#v", view)
	}
	request := desktopcontract.ConfirmBinding{PreviewID: view.PreviewID, PreviewDigest: view.PreviewDigest, ExpectedGuardRevision: view.ExpectedGuardRevision, Confirmed: true}
	receipt, err := f.preflight.Confirm(request, "desktop-confirm-key")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Action != "confirm" || receipt.Binding.State != "active" || !receipt.Binding.ProtectionActive {
		t.Fatalf("receipt: %#v", receipt)
	}
	if receipt.OperationID == "" || receipt.Binding.GuardRevision != 1 {
		t.Fatalf("receipt identity: %#v", receipt)
	}
	// The same key replays the original receipt after the preview is consumed.
	replayed, err := f.preflight.Confirm(request, "desktop-confirm-key")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.OperationID != receipt.OperationID || replayed.Binding.GuardRevision != receipt.Binding.GuardRevision {
		t.Fatalf("lost confirmation was not replayed: %#v", replayed)
	}
	// A different key cannot turn the consumed preview into another install.
	if _, err = f.preflight.Confirm(request, "desktop-confirm-other"); err == nil || err.Error() != "binding_preview_expired" {
		t.Fatalf("consumed preview accepted another key: %v", err)
	}
	// A new preview observes the installed revision, so a stale one can never
	// silently overwrite it.
	next := f.preview(t)
	if next.ExpectedGuardRevision != 1 {
		t.Fatalf("second preview revision: %#v", next)
	}
	if _, err = f.preflight.Confirm(desktopcontract.ConfirmBinding{PreviewID: view.PreviewID, PreviewDigest: view.PreviewDigest, ExpectedGuardRevision: 0, Confirmed: true}, "stale-key"); err == nil || err.Error() != "binding_stale" {
		t.Fatalf("stale confirmation: %v", err)
	}
}

// A host without the owner seam can only refuse — the same explicit denial as
// before, with no path that turns a confirmation into a receipt.
func TestDesktopAdmissionAbsentStaysRefused(t *testing.T) {
	root := t.TempDir()
	preflight := NewPreflight()
	defer preflight.Close()
	selection, err := preflight.Select(root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := preflight.Preview(selection)
	if err != nil {
		t.Fatal(err)
	}
	if view.Decision != "blocked" || view.Reason != "managed_scope_unverifiable" {
		t.Fatalf("deny-only preview: %#v", view)
	}
	if _, err = preflight.Confirm(desktopcontract.ConfirmBinding{PreviewID: view.PreviewID, PreviewDigest: view.PreviewDigest, ExpectedGuardRevision: 0, Confirmed: true}, "key"); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("deny-only confirm: %v", err)
	}
}

// A directory no owner space registered cannot join a protection domain; the
// host must not infer a space from a renderer path.
func TestDesktopAdmissionUnregisteredRootRefused(t *testing.T) {
	f := newAdmissionFixture(t)
	defer f.preflight.Close()
	other := filepath.Join(filepath.Dir(f.root), "unregistered")
	if err := os.MkdirAll(other, 0700); err != nil {
		t.Fatal(err)
	}
	selection, err := f.preflight.Select(other)
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.preflight.Preview(selection)
	if err != nil {
		t.Fatalf("an unregistered root must stay a blocked preview, not a host error: %v", err)
	}
	if view.Decision != "blocked" || view.Reason != "managed_scope_unverifiable" {
		t.Fatalf("unregistered root: %#v", view)
	}
}

// Hiding keeps protection; release requires the explicit confirmation flag and
// advances the revision; a released binding leaves the session so it cannot be
// mutated again through the same ticket.
func TestDesktopAdmissionVisibilityAndRelease(t *testing.T) {
	f := newAdmissionFixture(t)
	defer f.preflight.Close()
	view := f.preview(t)
	confirmed, err := f.preflight.Confirm(desktopcontract.ConfirmBinding{PreviewID: view.PreviewID, PreviewDigest: view.PreviewDigest, ExpectedGuardRevision: 0, Confirmed: true}, "confirm-key")
	if err != nil {
		t.Fatal(err)
	}
	id := confirmed.Binding.BindingID
	hidden, err := f.preflight.SetVisibility(id, desktopcontract.SetSourceVisibility{ExpectedRevision: 1, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if hidden.State != "hidden" || !hidden.ProtectionActive || hidden.Revision != 2 {
		t.Fatalf("hide: %#v", hidden)
	}
	shown, err := f.preflight.SetVisibility(id, desktopcontract.SetSourceVisibility{ExpectedRevision: 2, Hidden: false})
	if err != nil {
		t.Fatal(err)
	}
	if shown.State != "active" || !shown.ProtectionActive || shown.Revision != 3 {
		t.Fatalf("show: %#v", shown)
	}
	// A caller-supplied assertion without `confirmed` is not authorization.
	if _, err = f.preflight.Release(id, desktopcontract.ReleaseProtection{ExpectedRevision: 3}, "release-unconfirmed"); err == nil || err.Error() != "request_invalid" {
		t.Fatalf("unconfirmed release: %v", err)
	}
	released, err := f.preflight.Release(id, desktopcontract.ReleaseProtection{ExpectedRevision: 3, Confirmed: true}, "release-key")
	if err != nil {
		t.Fatal(err)
	}
	if released.Binding.State != "released" || released.Binding.ProtectionActive || released.Binding.Revision != 4 {
		t.Fatalf("release: %#v", released)
	}
	// The owner answers with its own released-protection code instead of the host
	// inventing a state, and the released source no longer resolves reads while
	// its tombstone still allows a replayed release.
	if _, err = f.preflight.SetVisibility(id, desktopcontract.SetSourceVisibility{ExpectedRevision: 4, Hidden: true}); err == nil || err.Error() != "protection_released" {
		t.Fatalf("mutating a released binding through the same ticket: %v", err)
	}
	if _, err = f.preflight.SourceRoot(released.Binding.SourceID); err == nil || err.Error() != "source_unknown" {
		t.Fatalf("a released source still resolved reads: %v", err)
	}
}

// The two declared mutation routes accept exactly one opaque identity segment and
// refuse extra segments, empty identities and unrelated paths.
func TestBindingMutationRoutePaths(t *testing.T) {
	if id, ok := bindingVisibilityPath("/v1/desktop/bindings/guard-abc"); !ok || id != "guard-abc" {
		t.Fatalf("visibility path: %q %v", id, ok)
	}
	for _, target := range []string{"/v1/desktop/bindings/", "/v1/desktop/bindings/guard-abc/extra", "/v1/desktop/other"} {
		if _, ok := bindingVisibilityPath(target); ok {
			t.Fatalf("visibility path accepted %q", target)
		}
	}
	if id, ok := bindingReleasePath("/v1/desktop/bindings/guard-abc/release"); !ok || id != "guard-abc" {
		t.Fatalf("release path: %q %v", id, ok)
	}
	for _, target := range []string{"/v1/desktop/bindings//release", "/v1/desktop/bindings/a/b/release", "/v1/desktop/bindings/guard-abc"} {
		if _, ok := bindingReleasePath(target); ok {
			t.Fatalf("release path accepted %q", target)
		}
	}
}
