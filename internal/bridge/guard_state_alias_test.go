package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// A guard may only be installed when the owner state lives outside the
// protection domain. The fenced database, its inert marker and every
// `.managed-state-v1` snapshot are written by this application, so a state
// directory inside an adopted root would mutate the vault through a path that is
// not an admitted space target.
//
// `rejectStateRootAlias` already enforces exactly this predicate for the preview
// cache at R1. Guard preflight and confirm are entry points too, so they must not
// claim a protection domain they would themselves write into. The name keeps the
// case inside the registered D03 filter (`-run TestManagedVault`).
func TestManagedVaultGuardRefusesStateInsideProtectionDomain(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	base := t.TempDir()
	root := filepath.Join(base, "vault")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}

	// Same disposable owner shape as newGuardFixture: one adopted local root, one
	// Pinax adoption on it, then the fence migration that guard install requires.
	prepare := func(state string) *Service {
		t.Helper()
		first, err := New(state, NewOneDrive(), false)
		if err != nil {
			t.Fatal(err)
		}
		if err = first.DB.Create(&Space{ID: "space-1", Remote: "remote", LocalRoot: root, MaxBytes: 1 << 20}).Error; err != nil {
			first.Close()
			t.Fatal(err)
		}
		if err = first.DB.Create(&AdoptRecord{ID: "guard-owner", Consumer: "pinax", Kind: "local", SpaceID: "space-1", Location: root, IdempotencyKey: "guard-owner"}).Error; err != nil {
			first.Close()
			t.Fatal(err)
		}
		first.Close()
		if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := managedstate.Prepare(state, "cli", "migration-original")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = managedstate.Install(state, plan); err != nil {
			t.Fatal(err)
		}
		opened, err := New(state, NewOneDrive(), false)
		if err != nil {
			t.Fatal(err)
		}
		return opened
	}
	previewRequest := GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: "space-1", Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}

	// The state sits inside the vault. Preview and confirm must refuse before any
	// guard row is written, instead of granting a protection the app undercuts by
	// writing its own database into the protected tree.
	inside := prepare(filepath.Join(root, "state"))
	if _, err := inside.PreviewGuard(previewRequest); err == nil || err.Error() != "owner_protected" {
		inside.Close()
		t.Fatalf("preview accepted a state inside the protection domain: %v", err)
	}
	if _, err := inside.InstallGuard(GuardInstallRequest{PreviewID: "guard-inside", Digest: "sha256:unused", BindingID: "guard-inside", InstanceID: "instance-1", SourceID: "source-1", SpaceID: "space-1", Scopes: previewRequest.Scopes, Confirmed: true, IdempotencyKey: "confirm-inside"}); err == nil || err.Error() != "owner_protected" {
		inside.Close()
		t.Fatalf("confirm accepted a state inside the protection domain: %v", err)
	}
	var persisted int64
	if err := inside.DB.Model(&ManagedGuard{}).Count(&persisted).Error; err != nil {
		inside.Close()
		t.Fatal(err)
	}
	inside.Close()
	if persisted != 0 {
		t.Fatalf("a refused confirm persisted %d guard row(s)", persisted)
	}

	// The refusal is path-based, not instance-based: the same adoption with the
	// state outside the vault still previews ready and installs.
	outside := prepare(filepath.Join(base, "state-outside"))
	defer outside.Close()
	preview, err := outside.PreviewGuard(previewRequest)
	if err != nil {
		t.Fatalf("a state outside the protection domain must stay installable: %v", err)
	}
	if preview.Decision != "ready" {
		t.Fatalf("a state outside the protection domain must preview ready: %#v", preview)
	}
	if _, err = outside.InstallGuard(GuardInstallRequest{PreviewID: preview.PreviewID, Digest: preview.Digest, BindingID: preview.BindingID, ExpectedGuardRevision: preview.ExpectedGuardRevision, InstanceID: "instance-1", SourceID: "source-1", SpaceID: "space-1", Scopes: preview.Scopes, Confirmed: true, IdempotencyKey: "confirm-outside"}); err != nil {
		t.Fatalf("a state outside the protection domain must install: %v", err)
	}
}
