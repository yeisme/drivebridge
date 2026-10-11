package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/personalstate"
)

func sum256(data []byte) []byte {
	digest := sha256.Sum256(data)
	return digest[:]
}

func readingStateFixture(t *testing.T, s *Service, ref, version string) {
	t.Helper()
	if err := s.DB.Create(&File{Ref: ref, SpaceID: "creative", DriveID: "drive", ItemID: "item-" + ref, Version: version, Name: ref, MIME: "text/markdown", Size: 10, ObservedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
}

func localFileRef(ref, version string) desktopcontract.ReadingRef {
	return desktopcontract.ReadingRef{Kind: "file", InstanceID: "local", SpaceID: "creative", FileID: ref, VersionID: version}
}

// D04: the root adapter keeps reading state on the local machine and survives
// a full service restart through the shared store tables.
func TestReadingStatePersistsAcrossServiceRestart(t *testing.T) {
	s, _, _, state := setup(t)
	readingStateFixture(t, s, "file-a", "v1")
	ref := localFileRef("file-a", "v1")
	scope := personalstate.Scope{Subject: "host-subject", Instance: "local"}
	favorite := true
	store, err := s.ReadingStates()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Update(t.Context(), scope, "first", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, Favorite: &favorite, Position: &desktopcontract.Position{Revision: "v1", Anchor: "p1", Offset: 3}})
	if err != nil || saved.Revision != 1 || !saved.Favorite || saved.Position == nil {
		t.Fatal(saved, err)
	}
	s.Close()
	reopened, err := New(state, fake(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	store, err = reopened.ReadingStates()
	if err != nil {
		t.Fatal(err)
	}
	reread, found, err := store.Get(t.Context(), scope, ref)
	if err != nil || !found || reread.Revision != 1 || !reread.Favorite || reread.Position == nil || reread.Position.Offset != 3 || !reread.UpdatedAt.Equal(saved.UpdatedAt) {
		t.Fatal(reread, found, err)
	}
}

// D04: CAS conflicts stay conflicts, replays return the original receipt, and
// a lost response is recoverable through Lookup with the original key.
func TestReadingStateCASReplayAndLostResponseLookup(t *testing.T) {
	s, _, _, _ := setup(t)
	readingStateFixture(t, s, "file-b", "v1")
	ref := localFileRef("file-b", "v1")
	scope := personalstate.Scope{Subject: "host-subject", Instance: "local"}
	readLater := true
	store, err := s.ReadingStates()
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Update(t.Context(), scope, "key", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater})
	if err != nil || first.Revision != 1 {
		t.Fatal(first, err)
	}
	if _, err = store.Update(t.Context(), scope, "key-2", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater}); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale CAS accepted", err)
	}
	replay, err := store.Update(t.Context(), scope, "key", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater})
	if err != nil || replay.Revision != first.Revision || replay.UpdatedAt.Unix() != first.UpdatedAt.Unix() {
		t.Fatal("replay drifted", replay, err)
	}
	unfavorite := false
	if _, err = store.Update(t.Context(), scope, "key", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, Favorite: &unfavorite}); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key accepted a changed request", err)
	}
	lookup, err := store.Lookup(t.Context(), scope, "key")
	if err != nil || !lookup.Found || lookup.State == nil || lookup.State.Revision != first.Revision {
		t.Fatal("lost response not recoverable", lookup, err)
	}
}

// D04: subjects and instances are isolated; unknown, trashed and pinax
// targets are refused without ever deleting stored progress.
func TestReadingStateIsolationAndTargetRefusals(t *testing.T) {
	s, _, _, _ := setup(t)
	readingStateFixture(t, s, "file-c", "v1")
	readingStateFixture(t, s, "file-d", "v1")
	ref := localFileRef("file-c", "v1")
	scope := personalstate.Scope{Subject: "host-subject", Instance: "local"}
	favorite := true
	store, err := s.ReadingStates()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Update(t.Context(), scope, "iso", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, Favorite: &favorite}); err != nil {
		t.Fatal(err)
	}
	other := personalstate.Scope{Subject: "other-subject", Instance: "local"}
	if _, found, _ := store.Get(t.Context(), other, ref); found {
		t.Fatal("foreign subject read host state")
	}
	if _, found, err := store.Get(t.Context(), personalstate.Scope{Subject: "host-subject", Instance: "other-instance"}, ref); err == nil || found {
		t.Fatal("foreign instance read host state", found, err)
	}
	unknown := localFileRef("missing", "v1")
	if _, found, err := store.Get(t.Context(), scope, unknown); err == nil || found {
		t.Fatal("unknown target admitted", found, err)
	}
	if err = s.DB.Model(&File{}).Where("ref = ?", "file-c").Update("trashed_at", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Get(t.Context(), scope, ref); err == nil || found {
		t.Fatal("trashed target admitted", found, err)
	}
	var rows int64
	if err = s.DB.Table("local_reading_states").Where("subject = ?", "host-subject").Count(&rows).Error; err != nil || rows != 1 {
		t.Fatal("stored state destroyed with target", rows, err)
	}
	pinaxRevision := "sha256:" + hex.EncodeToString(sum256([]byte("pinax-note")))
	pinax := desktopcontract.ReadingRef{Kind: "pinax", InstanceID: "local", SourceID: "source", ObjectID: "note", Revision: pinaxRevision, URI: "pinax://note/note?v=" + pinaxRevision}
	if _, err = store.Update(t.Context(), scope, "pinax", desktopcontract.UpdateReadingState{TargetRef: pinax, ExpectedRevision: 0, Favorite: &favorite}); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("unbound pinax reference admitted", err)
	}
	// Deleting the whole target row hides state but keeps the stored row.
	if err = s.DB.Where("ref = ?", "file-d").Delete(&File{}).Error; err != nil {
		t.Fatal(err)
	}
	refD := localFileRef("file-d", "v1")
	if _, err = store.Update(t.Context(), scope, "deleted", desktopcontract.UpdateReadingState{TargetRef: refD, ExpectedRevision: 0, Favorite: &favorite}); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("deleted target admitted", err)
	}
	var after int64
	if err = s.DB.Table("local_reading_states").Where("subject = ?", "host-subject").Count(&after).Error; err != nil || after != 1 {
		t.Fatal("refusal deleted stored state", after, err)
	}
}
