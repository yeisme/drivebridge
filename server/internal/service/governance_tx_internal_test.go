package service

// F4 rework regression: every authorization lookup inside an open
// transaction must run on the caller's transaction handle. With
// MaxOpenConns(1) a pooled query inside an open transaction blocks forever,
// so each probe runs under a watchdog: pre-fix it deadlocks, post-fix the
// authorization completes on the transaction itself.

import (
	"gorm.io/gorm"

	"os"
	"testing"
	"time"
)

// probeInTransaction runs fn inside one open transaction and fails the test
// if it does not complete before the watchdog fires.
func probeInTransaction(t *testing.T, s *Service, name string, fn func(tx *gorm.DB) error) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- s.DB.Transaction(func(tx *gorm.DB) error { return fn(tx) })
	}()
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("%s failed: %v", name, e)
		}
	case <-time.After(8 * time.Second):
		// The leaked transaction still holds the single connection; the
		// fixture cannot be closed on this path, so it is abandoned.
		t.Fatalf("%s deadlocked: pooled query inside the open transaction", name)
	}
}

func TestFPReworkAuthorizationUsesTransactionHandle(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true})
	if e != nil {
		t.Fatal(e)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			s.Close()
		}
	})
	root, _ := s.ensureUser("fixture", "root", "Root")
	owner, _ := s.ensureUser("fixture", "owner", "Owner")
	if e = s.BootstrapGovernance(root.ID); e != nil {
		t.Fatal(e)
	}
	var sp Space
	s.DB.First(&sp, "owner = ?", owner.ID)
	expires := time.Now().Add(time.Hour)

	// A file row so execution-segment path checks can match it.
	file := File{ID: ID(), Owner: owner.ID, SpaceID: sp.ID, Path: "gov/f.txt", VersionID: ID(), Size: 3, Updated: time.Now().UTC()}
	if e = s.DB.Create(&file).Error; e != nil {
		t.Fatal(e)
	}

	// Approved range grant: the grants loop plus its canApprove call run
	// inside the transaction (governance_model.go execution/grants segment).
	rangeGrant := AccessRequest{ID: ID(), TenantID: s.Instance, Requester: root.ID, Approver: owner.ID, SpaceID: sp.ID, Kind: "range", Action: "write", Status: "approved", GrantUntil: expires}
	if e = s.DB.Create(&rangeGrant).Error; e != nil {
		t.Fatal(e)
	}
	// Approved emergency grant: the platformAdmin check runs on the handle.
	emergency := AccessRequest{ID: ID(), TenantID: s.Instance, Requester: root.ID, Approver: owner.ID, SpaceID: sp.ID, Kind: "emergency", Action: "write", Status: "approved", GrantUntil: expires}
	if e = s.DB.Create(&emergency).Error; e != nil {
		t.Fatal(e)
	}
	// Executing request with a running item: the execution segment and its
	// in-loop canApprove run on the handle.
	executing := AccessRequest{ID: ID(), TenantID: s.Instance, Requester: root.ID, Approver: owner.ID, SpaceID: sp.ID, Kind: "range", Action: "overwrite", Status: "executing", GrantUntil: expires, Items: []RequestedAction{{FileID: file.ID, VersionID: file.VersionID, Action: "overwrite", Status: "running"}}}
	if e = s.DB.Create(&executing).Error; e != nil {
		t.Fatal(e)
	}
	// A live share for the ShareID principal probe in allowOn.
	secret := ID()
	share := Share{ID: ID(), Owner: owner.ID, ActorID: owner.ID, Name: "tx", SpaceID: sp.ID, Path: "gov", Mode: "fixed", Audience: "link", Revision: 1, Created: time.Now().UTC(), Expires: expires, TokenHash: Hash(secret)}
	if e = s.DB.Create(&share).Error; e != nil {
		t.Fatal(e)
	}

	adminP := Principal{UserID: root.ID, TenantID: s.Instance, DataOwner: owner.ID}
	// Viewer-class write: the platform admin is no owner/editor/custodian,
	// so the grant evaluation segment must decide — on the transaction.
	probeInTransaction(t, s, "range grant write", func(tx *gorm.DB) error {
		return s.governanceAllow(tx, adminP, sp.ID, "gov/g.txt", "write")
	})
	probeInTransaction(t, s, "emergency grant write", func(tx *gorm.DB) error {
		return s.governanceAllow(tx, adminP, sp.ID, "gov/e.txt", "write")
	})
	execP := Principal{UserID: root.ID, TenantID: s.Instance, DataOwner: owner.ID, ExecutionID: executing.ID}
	probeInTransaction(t, s, "executing request write", func(tx *gorm.DB) error {
		return s.governanceAllow(tx, execP, sp.ID, file.Path, "overwrite")
	})
	// ShareID principals: the share lookup inside allowOn runs on the
	// caller's handle too.
	shareP := Principal{UserID: owner.ID, TenantID: s.Instance, DataOwner: owner.ID, ShareID: share.ID}
	probeInTransaction(t, s, "share principal read", func(tx *gorm.DB) error {
		return s.allowOn(tx, shareP, sp.ID, file.Path, "read")
	})
	closed = true
}
