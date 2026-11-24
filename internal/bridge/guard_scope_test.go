package bridge

import (
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

// Guard installation decides which reader scopes a bound session may hold, so it
// must accept only the contract's closed reader set. A weaker `len > 0` check
// lets an undefined scope be persisted on the guard row and then admitted by
// `guardScopesCover`, which turns an unknown scope into granted authority.
//
// The contract already owns the predicate (known scopes only, no duplicates, at
// most four, always including `reader.metadata`); the guard must reuse it instead
// of keeping a second, weaker rule.
func TestManagedVaultGuardRejectsUndefinedReaderScopes(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()

	preview := func(scopes []desktopcontract.ReaderScope) error {
		_, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: scopes})
		return err
	}
	// The confirm path keeps its existing closed error (`invalid_binding_confirmation`)
	// because scope and binding identity share one validation step; the preview path
	// already owns the precise `invalid_reader_scopes`.
	confirm := func(scopes []desktopcontract.ReaderScope, key string) error {
		_, err := f.svc.InstallGuard(GuardInstallRequest{PreviewID: "guard-scope", Digest: "sha256:unused", BindingID: "guard-scope", InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: scopes, Confirmed: true, IdempotencyKey: key})
		return err
	}

	cases := []struct {
		name   string
		scopes []desktopcontract.ReaderScope
	}{
		{"undefined", []desktopcontract.ReaderScope{"reader.bogus"}},
		{"writer", []desktopcontract.ReaderScope{"writer.editor"}},
		{"duplicate", []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata, desktopcontract.ScopeMetadata}},
		{"metadata-floor", []desktopcontract.ReaderScope{desktopcontract.ScopeBody}},
	}
	for _, tc := range cases {
		if err := preview(tc.scopes); err == nil || err.Error() != "invalid_reader_scopes" {
			t.Fatalf("%s: preview must refuse with invalid_reader_scopes: %v", tc.name, err)
		}
		if err := confirm(tc.scopes, "confirm-"+tc.name); err == nil || err.Error() != "invalid_binding_confirmation" {
			t.Fatalf("%s: confirm must refuse with invalid_binding_confirmation: %v", tc.name, err)
		}
	}
	var persisted int64
	if err := f.svc.DB.Model(&ManagedGuard{}).Count(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted != 0 {
		t.Fatalf("refused scopes persisted %d guard row(s)", persisted)
	}

	// A contract-valid reader set still previews ready and installs, and the
	// granted lease is limited to exactly those scopes.
	valid := []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata, desktopcontract.ScopeBody}
	ready, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: valid})
	if err != nil {
		t.Fatalf("a contract-valid reader set must stay installable: %v", err)
	}
	if ready.Decision != "ready" {
		t.Fatalf("a contract-valid reader set must preview ready: %#v", ready)
	}
	receipt, err := f.install(ready, "confirm-valid")
	if err != nil {
		t.Fatalf("a contract-valid reader set must install: %v", err)
	}
	lease, err := f.svc.AcquireGuardLease(ready.BindingID, "instance-1", "source-1", f.root, receipt.Epoch, []string{"reader.body"})
	if err != nil {
		t.Fatalf("confirmed body scope must be leaseable: %v", err)
	}
	if err := lease(); err != nil {
		t.Fatal(err)
	}
	// An undefined scope can never be grantable, even after a guard exists.
	if _, err := f.svc.AcquireGuardLease(ready.BindingID, "instance-1", "source-1", f.root, receipt.Epoch, []string{"reader.bogus"}); err == nil || err.Error() != "session_request_invalid" {
		t.Fatalf("an undefined scope must never be grantable: %v", err)
	}
}
