package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// The state directory and a protection-domain root must be disjoint in BOTH
// directions. `RegisterReaderRoot` already enforces `same || rootInState ||
// stateInRoot`; guard admission only rejected state inside the root, so a vault
// registered inside the owner state directory still previewed ready.
//
// That configuration is reachable through the product: `AddSpace` validates the
// root kind but never checks it against the state path. A vault under the state
// directory shares a tree with the preview cache (`state/preview-cache`) and the
// fence snapshots (`.managed-state-v1`), so app writes would land in the
// protected domain.
func TestManagedVaultGuardRefusesRootInsideOwnerState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	base := t.TempDir()
	state := filepath.Join(base, "state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(state, "vault")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}

	first, err := New(state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.AddSpace(Space{ID: "space-1", Remote: "remote", DriveID: "drive-1", RootID: "root-1", LocalRoot: root, MaxBytes: 1 << 20}); err != nil {
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
	svc, err := New(state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	request := GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: "space-1", Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}
	if _, err := svc.PreviewGuard(request); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("preview accepted a vault root inside the owner state: %v", err)
	}
	if _, err := svc.InstallGuard(GuardInstallRequest{PreviewID: "guard-nested", Digest: "sha256:unused", BindingID: "guard-nested", InstanceID: "instance-1", SourceID: "source-1", SpaceID: "space-1", Scopes: request.Scopes, Confirmed: true, IdempotencyKey: "confirm-nested"}); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("confirm accepted a vault root inside the owner state: %v", err)
	}
	var persisted int64
	if err := svc.DB.Model(&ManagedGuard{}).Count(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted != 0 {
		t.Fatalf("a refused confirm persisted %d guard row(s)", persisted)
	}
}
