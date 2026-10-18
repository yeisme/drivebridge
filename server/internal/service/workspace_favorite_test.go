package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceFavoriteCASRecoveryIsolation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "favorites/note.md", "# Note")
	q := contract.FavoriteCreate{TargetKind: "file", TargetRef: "drivebridge://" + f.Service.Instance + "/file/" + file.FileID}
	first, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "favorite-create")
	if err != nil || first.ResultRevision != 1 {
		t.Fatal(first, err)
	}
	if _, err = f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "stale-create"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("stale create", err)
	}
	removed, err := f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.FavoriteID, contract.FavoriteDelete{ExpectedRevision: 1}, "favorite-delete")
	if err != nil || removed.ResultRevision != 2 || !removed.Deleted {
		t.Fatal(removed, err)
	}
	replay, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "favorite-create")
	if err != nil || replay != first {
		t.Fatal("original receipt lost", replay, err)
	}
	lookup, err := f.Service.LookupWorkspaceFavorite(t.Context(), p, h.space.ID, contract.FavoriteLookup{Action: "delete", IdempotencyKey: "favorite-delete"})
	if err != nil || lookup != removed {
		t.Fatal("delete receipt lost", err)
	}
	rows, err := f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{})
	if err != nil || len(rows.Items) != 1 || !rows.Items[0].Deleted || rows.Items[0].Revision != 2 {
		t.Fatal(rows, err)
	}
	var stored service.File
	if err = f.Service.DB.First(&stored, "id = ?", file.FileID).Error; err != nil || stored.Trashed {
		t.Fatal("favorite deleted target", err)
	}
	q.ExpectedRevision = 2
	added, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "favorite-readd")
	if err != nil || added.FavoriteID != first.FavoriteID || added.ResultRevision != 3 {
		t.Fatal("binding CAS reset", added, err)
	}
	deleteReplay, err := f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.FavoriteID, contract.FavoriteDelete{ExpectedRevision: 1}, "favorite-delete")
	if err != nil || deleteReplay != removed {
		t.Fatal("delete replay followed readded binding", deleteReplay, err)
	}
	if _, err = f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "favorite-create"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed CAS request", err)
	}
	if _, err = f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.FavoriteID, contract.FavoriteDelete{ExpectedRevision: 3}, "favorite-delete"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("delete same key accepted different CAS", err)
	}
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.WorkspaceFavorites(t.Context(), bad, h.space.ID, contract.PageQuery{}); err == nil {
			t.Fatal("foreign favorite visible")
		}
		if _, err = f.Service.LookupWorkspaceFavorite(t.Context(), bad, h.space.ID, contract.FavoriteLookup{Action: "create", IdempotencyKey: "favorite-create"}); err == nil {
			t.Fatal("foreign receipt visible")
		}
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", file.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{})
	if err != nil || len(rows.Items) != 0 {
		t.Fatal("hidden favorite leaked", rows, err)
	}
	if _, err = f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.FavoriteID, contract.FavoriteDelete{ExpectedRevision: 3}, "remove-hidden"); err != nil {
		t.Fatal("cannot remove inaccessible binding", err)
	}
}
