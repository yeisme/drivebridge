package service_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceFavoriteHTTPPaginationAndCursorBinding(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	for i := range 3 {
		collection := service.FileCollection{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, Name: "collection", Kind: "manual", Scope: "private", Revision: 1}
		if err := f.Service.DB.Create(&collection).Error; err != nil {
			t.Fatal(err)
		}
		q := contract.FavoriteCreate{SpaceID: h.space.ID, TargetKind: "collection", TargetRef: "drivebridge://" + f.Service.Instance + "/collection/" + collection.ID}
		if _, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, "favorite-page-"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	base := "/favorites?space_id=" + h.space.ID
	var first, second contract.FavoriteList
	if status := h.call(t, "GET", base+"&limit=2", nil, &first); status != 200 || len(first.Items) != 2 || first.NextCursor == "" || first.Total != nil || first.TotalKind != "unknown" {
		t.Fatal(status, first)
	}
	continuation := base + "&limit=2&cursor=" + url.QueryEscape(first.NextCursor)
	if status := h.call(t, "GET", continuation, nil, &second); status != 200 || len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].FavoriteID == first.Items[1].FavoriteID {
		t.Fatal(status, second)
	}
	if status := h.call(t, "GET", base+"&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("limit binding", status)
	}
	if status := bob.call(t, "GET", "/favorites?space_id="+bob.space.ID+"&limit=2&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("actor binding", status)
	}
	for _, query := range []string{"", "?space_id=", "?space_id=" + h.space.ID + "&limit=0", "?space_id=" + h.space.ID + "&limit=201", "?space_id=" + h.space.ID + "&limit=2&limit=2", "?space_id=" + h.space.ID + "&unknown=1"} {
		if status := h.call(t, "GET", "/favorites"+query, nil, nil); status != 400 {
			t.Fatal("invalid query accepted", query, status)
		}
	}
	if _, err := f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.Items[0].FavoriteID, contract.FavoriteDelete{ExpectedRevision: first.Items[0].Revision}, "favorite-page-delete"); err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", continuation, nil, nil); status != 410 {
		t.Fatal("generation binding", status)
	}
	// An unregistered route returns net/http's plain-text 404, not an API
	// envelope. Inspect the transport status without the JSON API helper.
	response, err := h.http.Get(h.base + "/api/v1/spaces/" + h.space.ID + "/favorites")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("obsolete favorite route retained", response.StatusCode)
	}
}
