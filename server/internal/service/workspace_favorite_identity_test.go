package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceFavoriteDirectoryRootAndTagIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	base := "drivebridge://" + f.Service.Instance + "/connection/" + h.space.ConnectionID + "/root/"
	ids := map[string]bool{}
	for _, rootID := range []string{service.ID(), service.ID()} {
		root := service.CloudRoot{ID: rootID, Owner: p.UserID, ConnectionID: h.space.ConnectionID, Provider: "s3", RootRef: rootID, Status: "observed", Generation: 1}
		if err := f.Service.DB.Create(&root).Error; err != nil {
			t.Fatal(err)
		}
		entry := service.ExternalEntry{ID: service.ID(), RootID: rootID, ProviderItemID: "same-item", Owner: p.UserID, ConnectionID: h.space.ConnectionID, Path: "same-path", Kind: "directory"}
		if err := f.Service.DB.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
		out, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, contract.FavoriteCreate{TargetKind: "directory", TargetRef: base + rootID + "/directory/same-item"}, "favorite-root-"+rootID)
		if err != nil || ids[out.FavoriteID] {
			t.Fatal("root identity collapsed", out, err)
		}
		ids[out.FavoriteID] = true
	}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "tags.md", "# Tags")
	for _, namespace := range []string{"manual", "rule", "suggested"} {
		label := "reference/中文 notes"
		if err := f.Service.DB.Create(&service.FileLabelAssignment{ID: service.ID(), Owner: p.UserID, FileID: file.FileID, Namespace: namespace, Label: label, AuthorKind: "human"}).Error; err != nil {
			t.Fatal(err)
		}
		ref := (&url.URL{Scheme: "drivebridge", Host: f.Service.Instance, Path: "/tag/" + namespace + "/" + label}).String()
		out, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, contract.FavoriteCreate{TargetKind: "tag", TargetRef: ref}, "favorite-tag-"+namespace)
		if err != nil || ids[out.FavoriteID] {
			t.Fatal("tag namespace/Unicode identity collapsed", out, err)
		}
		ids[out.FavoriteID] = true
	}
	list, err := f.Service.WorkspaceFavorites(t.Context(), p, h.space.ID, contract.PageQuery{})
	if err != nil || len(list.Items) != 5 {
		t.Fatal(list, err)
	}
}
