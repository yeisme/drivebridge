package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGuardLeasePinsEpochAcrossReleaseAndRebind(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	installed, err := f.install(preview, "confirm-first")
	if err != nil {
		t.Fatal(err)
	}
	acquire := func(epoch uint64) (func() error, error) {
		return f.svc.AcquireGuardLease(preview.BindingID, "instance-1", "source-1", f.root, epoch, []string{"reader.metadata"})
	}
	lease, err := acquire(installed.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	// 同一互斥门覆盖 owner 调用与 release；确定性检查锁，避免仅靠 sleep 推断排空。
	if f.svc.guardMu.TryLock() {
		f.svc.guardMu.Unlock()
		t.Fatal("owner session did not hold writer admission")
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.ReleaseGuard(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 1, Confirmed: true, IdempotencyKey: "release-first"})
		done <- err
	}()
	if err := lease(); err != nil {
		t.Fatal(err)
	}
	if err := lease(); err != nil { // cleanup may run twice, never unlock twice
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("release did not resume after the owner session drained")
	}
	if _, err := acquire(installed.Epoch); err == nil || err.Error() != "protection_released" {
		t.Fatalf("released epoch authorized a session: %v", err)
	}
	if _, err := f.install(f.preview(), "confirm-second"); err != nil {
		t.Fatal(err)
	}
	if _, err := acquire(installed.Epoch); err == nil || err.Error() != "protection_released" {
		t.Fatalf("rebind revived the old epoch: %v", err)
	}
}

func TestGuardLeaseRechecksRootAndProtectionDomain(t *testing.T) {
	for _, change := range []string{"replace-before", "replace-during", "new-overlap", "hidden", "wrong-source", "unreadable-registry"} {
		t.Run(change, func(t *testing.T) {
			f := newGuardFixture(t)
			f.fence()
			defer f.svc.Close()
			preview := f.preview()
			receipt, err := f.install(preview, "confirm")
			if err != nil {
				t.Fatal(err)
			}
			replace := func() {
				t.Helper()
				if err := os.Rename(f.root, f.root+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(f.root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			source := "source-1"
			want := ""
			switch change {
			case "replace-before":
				replace()
				want = "binding_stale"
			case "new-overlap":
				inner := filepath.Join(f.root, "nested")
				if err := os.Mkdir(inner, 0700); err != nil {
					t.Fatal(err)
				}
				if err := f.svc.DB.Create(&AdoptRecord{ID: "nested", Consumer: "pinax", Kind: "local", SpaceID: "other", Location: inner}).Error; err != nil {
					t.Fatal(err)
				}
				want = "managed_scope_unverifiable"
			case "hidden":
				if _, err := f.svc.SetGuardVisibility(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 1, Hidden: true, IdempotencyKey: "hide"}); err != nil {
					t.Fatal(err)
				}
			case "wrong-source":
				source, want = "other-source", "protection_released"
			case "unreadable-registry":
				if err := f.svc.DB.Migrator().DropTable(&AdoptRecord{}); err != nil {
					t.Fatal(err)
				}
				want = "guard_unavailable"
			}
			lease, err := f.svc.AcquireGuardLease(preview.BindingID, "instance-1", source, f.root, receipt.Epoch, []string{"reader.metadata"})
			if want != "" {
				if err == nil {
					_ = lease()
					t.Fatal("changed authority admitted an owner session")
				}
				if err.Error() != want {
					t.Fatalf("acquire: %v; want %s", err, want)
				}
				if !f.svc.guardMu.TryLock() {
					t.Fatal("failed acquire leaked writer admission")
				}
				f.svc.guardMu.Unlock()
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if change == "replace-during" {
				replace()
				want = "binding_stale"
			}
			err = lease()
			if want == "" && err != nil || want != "" && (err == nil || err.Error() != want) {
				t.Fatalf("readback: %v; want %q", err, want)
			}
		})
	}
}

func TestGuardLeaseRejectsUnconfirmedScopes(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	preview := f.preview()
	receipt, err := f.install(preview, "confirm-metadata-only")
	if err != nil {
		t.Fatal(err)
	}
	for _, scopes := range [][]string{nil, {}, {"reader.body"}, {"writer.editor"}, {"reader.metadata", "writer.editor"}, {"unknown"}} {
		lease, err := f.svc.AcquireGuardLease(preview.BindingID, "instance-1", "source-1", f.root, receipt.Epoch, scopes)
		if err == nil {
			_ = lease()
			t.Fatalf("unconfirmed scopes admitted: %v", scopes)
		}
		if err.Error() != "session_request_invalid" {
			t.Fatalf("unexpected scope refusal: %v", err)
		}
		if !f.svc.guardMu.TryLock() {
			t.Fatal("scope refusal leaked the guard lock")
		}
		f.svc.guardMu.Unlock()
	}
	lease, err := f.svc.AcquireGuardLease(preview.BindingID, "instance-1", "source-1", f.root, receipt.Epoch, []string{"reader.metadata"})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease(); err != nil {
		t.Fatal(err)
	}
}
