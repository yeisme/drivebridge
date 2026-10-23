package bridge

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

func TestManagedVaultAtomicStateFenceRejectsLegacyService(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("candidate state fence is Linux-only until platform acceptance")
	}
	state := filepath.Join(t.TempDir(), "state")
	s, err := New(state, NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Create(&Operation{ID: "original-operation", Key: "original-key", Status: "interrupted"}).Error; err != nil {
		s.Close()
		t.Fatal(err)
	}
	s.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "cli", "migration-original")
	if err != nil {
		t.Fatal(err)
	}
	installed, err := managedstate.Install(state, plan)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(installed.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := New(state, NewOneDrive(), true)
	if legacy != nil {
		legacy.Close()
		t.Fatal("legacy service started through the state fence")
	}
	if err == nil || err.Error() != "state_unavailable" {
		t.Fatalf("legacy admission error: %v", err)
	}
	after, err := os.ReadFile(installed.DatabasePath)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("legacy startup modified preserved operations")
	}
	if replay, err := managedstate.Install(state, plan); err != nil || replay != installed {
		t.Fatalf("original migration identity could not reconcile: %#v %v", replay, err)
	}
}

// Candidate storage-layout fence only. No product code installs or consumes
// this layout. Proves this checkout's legacy constructor refuses before
// migration/worker recovery; it does not attest every historical binary.
func TestManagedVaultLegacyDatabasePathFenceRejectsStartup(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	s, err := New(state, NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	legacy := filepath.Join(state, "state.sqlite")
	candidate := filepath.Join(state, "candidate-state.sqlite")
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(legacy, candidate); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(state, NewOneDrive(), true)
	if reopened != nil {
		reopened.Close()
		t.Fatal("legacy startup crossed the candidate layout fence")
	}
	if err == nil || err.Error() != "state_unavailable" {
		t.Fatalf("expected refusal before service creation: %v", err)
	}
	after, err := os.ReadFile(candidate)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("legacy startup changed the preserved database")
	}
	info, err := os.Lstat(legacy)
	if err != nil || !info.IsDir() {
		t.Fatal("legacy startup removed the candidate fence")
	}
}

// These are admission counterexamples, not tests of an installed managed
// guard. A binding must remain unavailable while these writers are admissible.
func TestManagedVaultLegacySeparateStateBypassesAdoptRegistry(t *testing.T) {
	s, local, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.Mkdir(context.Background(), "vault", ".pinax"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("adopt metadata boundary: %v", err)
	}
	// Content paths are intentionally not protected by the historical fence.
	if err := s.Mkdir(context.Background(), "vault", "notes"); err != nil {
		t.Fatal(err)
	}
	other, err := New(filepath.Join(t.TempDir(), "other-state"), NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err = other.AddSpace(Space{ID: "alias", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if err = other.Mkdir(context.Background(), "alias", ".pinax/cross-state-proof"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, ".pinax", "cross-state-proof")); err != nil || !info.IsDir() {
		t.Fatal("missing separate-state counterexample")
	}
}

func TestManagedVaultLegacyNewTableDoesNotGateStartup(t *testing.T) {
	_, local, root := genericFixture(t)
	state := filepath.Join(t.TempDir(), "legacy-state")
	s, err := New(state, NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	// A synthetic future registration table is deliberately unknown to New.
	// This proves only this checkout's legacy startup, not every old release.
	type FutureManagedRegistration struct {
		ID               string `gorm:"primaryKey"`
		ProtectionActive bool
	}
	if err = s.DB.AutoMigrate(&FutureManagedRegistration{}); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err = s.DB.Create(&FutureManagedRegistration{ID: "fixture", ProtectionActive: true}).Error; err != nil {
		s.Close()
		t.Fatal(err)
	}
	s.Close()
	reopened, err := New(state, NewOneDrive(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err = reopened.AddSpace(Space{ID: "legacy", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if err = reopened.Mkdir(context.Background(), "legacy", "after-future-registration"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "after-future-registration")); err != nil {
		t.Fatal(err)
	}
}

func TestManagedVaultAdoptRegistryUnavailableFailsClosed(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	// Disposable database only: simulate unreadable/missing protection facts.
	if err := s.DB.Migrator().DropTable(&AdoptRecord{}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".pinax/blocked", "ordinary/blocked"} {
		err := s.Mkdir(context.Background(), "vault", rel)
		if err == nil || err.Error() != "guard_unavailable" {
			t.Fatalf("registry failure must refuse before provider: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatal("registry failure mutated provider")
		}
	}
	source := filepath.Join(root, "fixture-upload.txt")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Upload(context.Background(), "vault", "", source, "fixture.txt", "original-key"); err == nil || err.Error() != "guard_unavailable" || safeError(err) != "guard_unavailable" {
		t.Fatalf("upload registry failure: %v", err)
	}
	var count int64
	if err := s.DB.Model(&Operation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("registry failure queued an operation")
	}
	var absent *Service
	if err := absent.rejectOwnerProtected("vault", ".pinax"); err == nil || err.Error() != "guard_unavailable" {
		t.Fatal("nil registry must refuse")
	}
}
