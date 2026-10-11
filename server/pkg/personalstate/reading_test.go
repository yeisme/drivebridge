package personalstate

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func readingDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	if err := MigrateReading(db); err != nil {
		t.Fatal(err)
	}
	return db
}
func fixtureRef() desktopcontract.ReadingRef {
	return desktopcontract.ReadingRef{Kind: "file", InstanceID: "instance", SpaceID: "space", FileID: "file", VersionID: "v1"}
}
func boolean(value bool) *bool                                         { return &value }
func allowed(context.Context, Scope, desktopcontract.ReadingRef) error { return nil }
func storeFor(t *testing.T, db *gorm.DB, authorize Authorize) *ReadingStore {
	t.Helper()
	store, err := NewReadingStore(db, authorize)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
func TestReadingStateCASReceiptRestartAndPosition(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "local.sqlite")
	db := readingDB(t, path)
	store := storeFor(t, db, allowed)
	scope := Scope{Subject: "human", Instance: "instance"}
	ref := fixtureRef()
	first := desktopcontract.UpdateReadingState{TargetRef: ref, Favorite: boolean(true), Position: &desktopcontract.Position{Revision: "v1", Anchor: "h1", Offset: 5}}
	one, err := store.Update(ctx, scope, "original-one", first)
	if err != nil || one.Revision != 1 || !one.Favorite {
		t.Fatal("first update failed", err)
	}
	// Concurrent changes to other fields preserve saved progress on the same revision.
	second := desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, ReadLater: boolean(true)}
	two, err := store.Update(ctx, scope, "original-two", second)
	if err != nil || two.Revision != 2 || two.Position == nil || !two.ReadLater {
		t.Fatal("patch lost state", err)
	}
	replay, err := store.Update(ctx, scope, "original-one", first)
	if err != nil || replay.Revision != 1 || replay.ReadLater {
		t.Fatal("replay returned latest state")
	}
	drift := first
	drift.Favorite = boolean(false)
	if _, err := store.Update(ctx, scope, "original-one", drift); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key drift accepted")
	}
	if _, err := store.Update(ctx, scope, "stale", first); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale update accepted")
	}
	if result, err := store.Lookup(ctx, scope, "stale"); err != nil || result.Found {
		t.Fatal("failed CAS retained receipt")
	}
	ref.VersionID = "v2"
	three, err := store.Update(ctx, scope, "original-three", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 2, Favorite: boolean(false)})
	if err != nil || three.Position != nil || three.Revision != 3 || three.Favorite || !three.ReadLater {
		t.Fatal("version shift retained stale position", err)
	}
	raw, _ := db.DB()
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := storeFor(t, readingDB(t, path), allowed)
	state, found, err := reopened.Get(ctx, scope, ref)
	if err != nil || !found || state.Revision != 3 {
		t.Fatal("restart lost state", err)
	}
	receipt, err := reopened.Lookup(ctx, scope, "original-one")
	if err != nil || !receipt.Found || receipt.State.Revision != 1 || receipt.State.Position == nil {
		t.Fatal("restart lost original receipt")
	}
}
func TestReadingStateSubjectSourceAndAdmission(t *testing.T) {
	ctx := context.Background()
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	revoked := false
	store := storeFor(t, db, func(_ context.Context, _ Scope, _ desktopcontract.ReadingRef) error {
		if revoked {
			return errors.New("private upstream path")
		}
		return nil
	})
	scope := Scope{"human", "instance"}
	ref := desktopcontract.ReadingRef{Kind: "pinax", InstanceID: "instance", SourceID: "vault-one", ObjectID: "same-id", Revision: "sha256:" + strings.Repeat("a", 64)}
	ref.URI = "pinax://note/" + ref.ObjectID + "?v=" + ref.Revision
	request := desktopcontract.UpdateReadingState{TargetRef: ref, Favorite: boolean(true)}
	if _, err := store.Update(ctx, scope, "key", request); err != nil {
		t.Fatal(err)
	}
	other := ref
	other.SourceID = "vault-two"
	if _, found, err := store.Get(ctx, scope, other); err != nil || found {
		t.Fatal("vault identity crossed")
	}
	if _, found, err := store.Get(ctx, Scope{"other", "instance"}, ref); err != nil || found {
		t.Fatal("subject identity crossed")
	}
	if result, err := store.Lookup(ctx, Scope{"other", "instance"}, "key"); err != nil || result.Found {
		t.Fatal("receipt crossed subject")
	}
	if _, _, err := store.Get(ctx, Scope{"human", "other-instance"}, ref); err == nil {
		t.Fatal("instance mismatch accepted")
	}
	revoked = true
	if _, err := store.Update(ctx, scope, "key", request); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("replay bypassed revoked source")
	}
	if _, err := store.Lookup(ctx, scope, "key"); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("lookup bypassed revoked source")
	}
	if _, _, err := store.Get(ctx, scope, ref); err == nil {
		t.Fatal("get bypassed revoked source")
	}
	if _, err := NewReadingStore(db, nil); err == nil {
		t.Fatal("missing authorizer accepted")
	}
}
func TestReadingStateConcurrentCASAndRollback(t *testing.T) {
	ctx := context.Background()
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	store := storeFor(t, db, allowed)
	scope := Scope{"human", "instance"}
	ref := fixtureRef()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, key := range []string{"one", "two"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := store.Update(ctx, scope, key, desktopcontract.UpdateReadingState{TargetRef: ref, Favorite: boolean(true)})
			results <- err
		}(key)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if err.Error() != "revision_conflict" {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("CAS has multiple winners")
	}
	// A receipt failure rolls the state update back in the same transaction.
	if err := db.Callback().Create().Before("gorm:create").Register("test_receipt_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "local_reading_receipts" {
			tx.AddError(errors.New("fixture failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(ctx, scope, "failed", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, ReadLater: boolean(true)}); err == nil {
		t.Fatal("receipt failure accepted")
	}
	state, found, err := store.Get(ctx, scope, ref)
	if err != nil || !found || state.Revision != 1 || state.ReadLater {
		t.Fatal("receipt failure left state mutation")
	}
}
func TestReadingStateAuthorizationCanReadSingleConnectionDB(t *testing.T) {
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	store := storeFor(t, db, func(ctx context.Context, _ Scope, _ desktopcontract.ReadingRef) error {
		var count int64
		return db.WithContext(ctx).Model(&readingRow{}).Count(&count).Error
	})
	if _, err := store.Update(ctx, Scope{"human", "instance"}, "original", desktopcontract.UpdateReadingState{TargetRef: fixtureRef(), Favorite: boolean(true)}); err != nil {
		t.Fatal(err)
	}
}

func TestReadingStateBudgetAndExplicitClear(t *testing.T) {
	ctx := context.Background()
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	store := storeFor(t, db, allowed)
	scope := Scope{"human", "instance"}
	ref := fixtureRef()
	if _, err := store.Update(ctx, scope, "k", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: maxRevision, Favorite: boolean(true)}); err == nil {
		t.Fatal("unsafe JS revision accepted")
	}
	state, err := store.Update(ctx, scope, "k", desktopcontract.UpdateReadingState{TargetRef: ref, Position: &desktopcontract.Position{Revision: "v1", Anchor: "h", Offset: 1}})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.Update(ctx, scope, "clear", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: state.Revision, ClearPosition: boolean(true)})
	if err != nil || state.Position != nil {
		t.Fatal("explicit clear failed")
	}
	for _, bad := range []desktopcontract.UpdateReadingState{{TargetRef: ref}, {TargetRef: ref, ClearPosition: boolean(false)}, {TargetRef: ref, Position: &desktopcontract.Position{Revision: "v2", Anchor: "h"}}} {
		if _, err := store.Update(ctx, scope, "invalid", bad); err == nil {
			t.Fatal("bad patch accepted")
		}
	}
	if err := MigrateReading(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&readingRow{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("repeated migration changed identity")
	}
}
