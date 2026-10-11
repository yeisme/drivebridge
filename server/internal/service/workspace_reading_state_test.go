package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/personalstate"
)

func readingFileRef(h *human, instance, file, version string) desktopcontract.ReadingRef {
	return desktopcontract.ReadingRef{Kind: "file", InstanceID: instance, SpaceID: h.space.ID, FileID: file, VersionID: version}
}

// D04: the server keeps a thin adapter over the same shared application the
// desktop facade runs. States persist across reopen, receipts replay the
// original result, and foreign subjects never see another user's state.
func TestReadingStateServerAdapterPersistsAcrossReopen(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "reading/note.md", "# Note")
	instance := f.Service.Instance
	ref := readingFileRef(h, instance, file.FileID, file.VersionID)
	scope := personalstate.Scope{Subject: h.user.ID, Instance: instance}
	favorite := true
	position := &desktopcontract.Position{Revision: file.VersionID, Anchor: "para-2", Offset: 12}
	store, err := f.Service.WorkspaceReadingStates(p)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Update(t.Context(), scope, "reading-first", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, Favorite: &favorite, Position: position})
	if err != nil || saved.Revision != 1 || !saved.Favorite || saved.Position == nil || saved.Position.Offset != 12 {
		t.Fatal(saved, err)
	}
	original := f.Service.Config
	f.Service.Close()
	reopened, err := service.Open(original)
	if err != nil {
		t.Fatal(err)
	}
	f.Service = reopened
	defer func() { f.Service = reopened }()
	store, err = reopened.WorkspaceReadingStates(p)
	if err != nil {
		t.Fatal(err)
	}
	reread, found, err := store.Get(t.Context(), scope, ref)
	if err != nil || !found || reread.Revision != saved.Revision || reread.Favorite != saved.Favorite || reread.Position == nil || reread.Position.Offset != saved.Position.Offset || !reread.UpdatedAt.Equal(saved.UpdatedAt) {
		t.Fatal(reread, found, err)
	}
	// A second subject on the same deployment sees nothing of the first.
	foreign, err := reopened.WorkspaceReadingStates(service.Principal{UserID: bob.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := foreign.Get(t.Context(), personalstate.Scope{Subject: bob.user.ID, Instance: instance}, ref); err == nil || found {
		t.Fatal("foreign subject read alice state", found, err)
	}
	if _, err := foreign.Update(t.Context(), personalstate.Scope{Subject: h.user.ID, Instance: instance}, "steal", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0}); err == nil {
		t.Fatal("foreign principal wrote alice scope")
	}
}

// D04: CAS conflicts stay conflicts, replayed keys return the original
// receipt, and a lost response is recoverable through Lookup.
func TestReadingStateServerAdapterCASAndLostResponse(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "reading/cas.md", "body")
	instance := f.Service.Instance
	ref := readingFileRef(h, instance, file.FileID, file.VersionID)
	scope := personalstate.Scope{Subject: h.user.ID, Instance: instance}
	readLater := true
	store, err := f.Service.WorkspaceReadingStates(p)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Update(t.Context(), scope, "cas", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater})
	if err != nil || first.Revision != 1 {
		t.Fatal(first, err)
	}
	if _, err = store.Update(t.Context(), scope, "cas-again", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater}); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale CAS accepted", err)
	}
	replayed, err := store.Update(t.Context(), scope, "cas", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, ReadLater: &readLater})
	if err != nil || replayed != first {
		t.Fatal("replay drifted", replayed, err)
	}
	unfavorite := false
	if _, err = store.Update(t.Context(), scope, "cas", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 1, Favorite: &unfavorite}); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key accepted a changed request", err)
	}
	lookup, err := store.Lookup(t.Context(), scope, "cas")
	if err != nil || !lookup.Found || lookup.State == nil || *lookup.State != first {
		t.Fatal("lost response not recoverable", lookup, err)
	}
}

// D04: deleting the target hides its state but never deletes stored progress;
// the row survives and becomes readable again once the file is restored.
// Pinax references are machine-local and refused on the server.
func TestReadingStateServerAdapterHidesDeletedTargetAndRefusesPinax(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "reading/gone.md", "gone")
	instance := f.Service.Instance
	ref := readingFileRef(h, instance, file.FileID, file.VersionID)
	scope := personalstate.Scope{Subject: h.user.ID, Instance: instance}
	favorite := true
	store, err := f.Service.WorkspaceReadingStates(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Update(t.Context(), scope, "gone", desktopcontract.UpdateReadingState{TargetRef: ref, ExpectedRevision: 0, Favorite: &favorite}); err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Delete(&service.File{}, "id = ?", file.FileID).Error; err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Get(t.Context(), scope, ref); err == nil || err.Error() != "reading_target_unavailable" || found {
		t.Fatal("deleted target still readable", found, err)
	}
	var rows int64
	if err = f.Service.DB.Table("local_reading_states").Where("subject = ?", h.user.ID).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatal("state row destroyed with target", rows, err)
	}
	pinaxRevision := "sha256:" + service.Hash("note")
	pinax := desktopcontract.ReadingRef{Kind: "pinax", InstanceID: instance, SourceID: "source", ObjectID: "note", Revision: pinaxRevision, URI: "pinax://note/note?v=" + pinaxRevision}
	if _, err = store.Update(t.Context(), scope, "pinax", desktopcontract.UpdateReadingState{TargetRef: pinax, ExpectedRevision: 0, ReadLater: &favorite}); err == nil || err.Error() != "reading_target_unavailable" {
		t.Fatal("server accepted a machine-local pinax reference", err)
	}
}
