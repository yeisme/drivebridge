package personalstate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

func pageLimit(n uint16) *uint16 { return &n }
func populateReading(t *testing.T, store *ReadingStore, scope Scope, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		ref := fixtureRef()
		ref.FileID = fmt.Sprintf("file-%d", i)
		if _, err := store.Update(context.Background(), scope, fmt.Sprintf("key-%d", i), desktopcontract.UpdateReadingState{TargetRef: ref, Favorite: boolean(true)}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestReadingStateListFilteredStablePages(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"human", "instance"}
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	revoked := false
	store := storeFor(t, db, func(_ context.Context, _ Scope, ref desktopcontract.ReadingRef) error {
		if revoked && ref.FileID == "file-1" {
			return ErrTargetHidden
		}
		return nil
	})
	populateReading(t, store, scope, 7)
	revoked = true
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 5; page++ {
		result, err := store.List(ctx, scope, "access-one", desktopcontract.PageQuery{Limit: pageLimit(2), Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Items) > 2 {
			t.Fatal("page bound")
		}
		for _, item := range result.Items {
			if item.TargetRef.FileID == "file-1" || seen[item.TargetRef.FileID] {
				t.Fatal("hidden or duplicate target")
			}
			seen[item.TargetRef.FileID] = true
		}
		if result.NextCursor == "" {
			break
		}
		if len(result.Items) == 0 {
			t.Fatal("hidden-only continuation")
		}
		cursor = result.NextCursor
	}
	if len(seen) != 6 {
		t.Fatal("lost visible target")
	}
	empty, err := store.List(ctx, Scope{"other", "instance"}, "access-one", desktopcontract.PageQuery{})
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatal("subject leaked")
	}
}
func TestReadingStateListCursorIsolationAndExpiry(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"human", "instance"}
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	store := storeFor(t, db, allowed)
	populateReading(t, store, scope, 3)
	now := time.Now()
	store.now = func() time.Time { return now }
	first, err := store.List(ctx, scope, "access-one", desktopcontract.PageQuery{Limit: pageLimit(1)})
	if err != nil || first.NextCursor == "" {
		t.Fatal("missing cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(decoded), "human") || strings.Contains(string(decoded), "instance") || json.Valid(decoded) {
		t.Fatal("cursor exposes private identity")
	}
	for _, tc := range []struct {
		scope  Scope
		epoch  string
		limit  uint16
		cursor string
	}{
		{Scope{"other", "instance"}, "access-one", 1, first.NextCursor},
		{Scope{"human", "other"}, "access-one", 1, first.NextCursor},
		{scope, "access-two", 1, first.NextCursor},
		{scope, "access-one", 2, first.NextCursor},
		{scope, "access-one", 1, "invalid"},
	} {
		if _, err := store.List(ctx, tc.scope, tc.epoch, desktopcontract.PageQuery{Limit: pageLimit(tc.limit), Cursor: tc.cursor}); err == nil || err.Error() != "cursor_expired" {
			t.Fatal("cursor crossed binding")
		}
	}
	decoded[len(decoded)-1] ^= 1
	if _, err := store.List(ctx, scope, "access-one", desktopcontract.PageQuery{Limit: pageLimit(1), Cursor: base64.RawURLEncoding.EncodeToString(decoded)}); err == nil {
		t.Fatal("tampered cursor accepted")
	}
	reopened := storeFor(t, db, allowed)
	if _, err := reopened.List(ctx, scope, "access-one", desktopcontract.PageQuery{Limit: pageLimit(1), Cursor: first.NextCursor}); err == nil {
		t.Fatal("session restart accepted old cursor")
	}
	now = now.Add(readingCursorLifetime)
	if _, err := store.List(ctx, scope, "access-one", desktopcontract.PageQuery{Limit: pageLimit(1), Cursor: first.NextCursor}); err == nil {
		t.Fatal("expired cursor accepted")
	}
}
func TestReadingStateListGenerationAndReplay(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"human", "instance"}
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	store := storeFor(t, db, allowed)
	populateReading(t, store, scope, 3)
	first, err := store.List(ctx, scope, "epoch", desktopcontract.PageQuery{Limit: pageLimit(1)})
	if err != nil {
		t.Fatal(err)
	}
	ref := fixtureRef()
	ref.FileID = "file-0"
	if _, err := store.Update(ctx, scope, "key-0", desktopcontract.UpdateReadingState{TargetRef: ref, Favorite: boolean(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, scope, "epoch", desktopcontract.PageQuery{Limit: pageLimit(1), Cursor: first.NextCursor}); err != nil {
		t.Fatal("receipt replay advanced generation")
	}
	// Another store on the same DB must invalidate the original store's cursor.
	other := storeFor(t, db, allowed)
	if _, err := other.Update(ctx, scope, "new-key", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, ReadLater: boolean(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, scope, "epoch", desktopcontract.PageQuery{Limit: pageLimit(1), Cursor: first.NextCursor}); err == nil || err.Error() != "cursor_expired" {
		t.Fatal("committed update did not expire cursor")
	}
}
func TestReadingStateListBudgetAndMidReadChange(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"human", "instance"}
	db := readingDB(t, filepath.Join(t.TempDir(), "local.sqlite"))
	store := storeFor(t, db, allowed)
	// Fixture seeding uses GORM, not application bypasses in production.
	rows := make([]readingRow, readingScanBudget+1)
	for i := range rows {
		ref := fixtureRef()
		ref.FileID = fmt.Sprintf("seed-%d", i)
		target := targetKey(ref)
		state := desktopcontract.ReadingState{TargetRef: ref, Revision: 1, UpdatedAt: time.Now().UTC()}
		encoded, _ := json.Marshal(state)
		rows[i] = readingRow{ID: hashParts(scope.Subject, scope.Instance, target), Subject: scope.Subject, Instance: scope.Instance, Target: target, Revision: 1, State: string(encoded)}
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	unavailable := storeFor(t, db, func(context.Context, Scope, desktopcontract.ReadingRef) error {
		return errors.New("private diagnostic")
	})
	if _, err := unavailable.List(ctx, scope, "epoch", desktopcontract.PageQuery{}); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("owner failure hidden as empty results")
	}
	hidden := storeFor(t, db, func(context.Context, Scope, desktopcontract.ReadingRef) error {
		return ErrTargetHidden
	})
	if _, err := hidden.List(ctx, scope, "epoch", desktopcontract.PageQuery{}); err == nil || err.Error() != "reading_scan_budget_exceeded" {
		t.Fatal("bounded scan pretended completion")
	}
	var calls int
	changing := storeFor(t, db, func(ctx context.Context, scope Scope, ref desktopcontract.ReadingRef) error {
		calls++
		if calls == 1 {
			_, err := store.Update(ctx, scope, "mid-list", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, Favorite: boolean(true)})
			return err
		}
		return nil
	})
	if _, err := changing.List(ctx, scope, "epoch", desktopcontract.PageQuery{Limit: pageLimit(1)}); err == nil || err.Error() != "cursor_expired" {
		t.Fatal("mid-list change accepted")
	}
	for _, n := range []uint16{0, 201} {
		if _, err := store.List(ctx, scope, "epoch", desktopcontract.PageQuery{Limit: pageLimit(n)}); err == nil {
			t.Fatal("invalid limit")
		}
	}
}
