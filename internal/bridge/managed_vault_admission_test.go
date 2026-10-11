package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
