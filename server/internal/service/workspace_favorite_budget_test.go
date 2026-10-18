package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceFavoriteDefaultMaximumPagesAndBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	collections := make([]service.FileCollection, 201)
	favorites := make([]service.WorkspaceFavorite, 201)
	for i := range collections {
		collections[i] = service.FileCollection{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, Name: "collection", Kind: "manual", Scope: "private", Revision: 1}
		favorites[i] = service.WorkspaceFavorite{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, TargetKind: "collection", TargetRef: "drivebridge://" + f.Service.Instance + "/collection/" + collections[i].ID, Revision: 1}
	}
	if err := f.Service.DB.CreateInBatches(collections, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.CreateInBatches(favorites, 100).Error; err != nil {
		t.Fatal(err)
	}
	first, err := f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{})
	if err != nil || len(first.Items) != 50 || first.NextCursor == "" {
		t.Fatal("default page", len(first.Items), err)
	}
	maximum, err := f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{Limit: 200})
	if err != nil || len(maximum.Items) != 200 || maximum.NextCursor == "" {
		t.Fatal("maximum page", len(maximum.Items), err)
	}
	last, err := f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{Limit: 200, Cursor: maximum.NextCursor})
	if err != nil || len(last.Items) != 1 || last.NextCursor != "" {
		t.Fatal("maximum continuation", last, err)
	}
	extra := make([]service.WorkspaceFavorite, 800)
	for i := range extra {
		extra[i] = service.WorkspaceFavorite{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, TargetKind: "file", TargetRef: "drivebridge://" + f.Service.Instance + "/file/" + service.ID(), Revision: 1}
	}
	if err = f.Service.DB.CreateInBatches(extra, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{Limit: 1}); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("small page bypassed scan budget", err)
	}
}
