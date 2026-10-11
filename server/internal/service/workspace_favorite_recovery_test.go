package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceFavoriteConcurrentCASAndRestart(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "favorite-restart/note.md", "# Empty links")
	q := contract.FavoriteCreate{TargetKind: "file", TargetRef: "drivebridge://" + f.Service.Instance + "/file/" + file.FileID}
	type result struct {
		key     string
		receipt contract.FavoriteReceipt
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, key := range []string{"favorite-concurrent-a", "favorite-concurrent-b"} {
		go func() {
			<-start
			receipt, err := f.Service.CreateWorkspaceFavorite(t.Context(), p, h.space.ID, q, key)
			results <- result{key, receipt, err}
		}()
	}
	close(start)
	var winner result
	wins, conflicts := 0, 0
	for range 2 {
		r := <-results
		if r.err == nil {
			wins++
			winner = r
		} else if r.err.Error() == "revision_conflict" {
			conflicts++
		} else {
			t.Fatal(r.err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("CAS admitted multiple writers", wins, conflicts)
	}
	links, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, file.FileID, contract.FileLinkRebuild{VersionID: file.VersionID}, "links-restart")
	if err != nil {
		t.Fatal(err)
	}
	cfg := f.Service.Config
	f.App.Close()
	f.Service.Close()
	reopened, err := service.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.LookupWorkspaceFavorite(t.Context(), p, h.space.ID, contract.FavoriteLookup{Action: "create", IdempotencyKey: winner.key})
	if err != nil || recovered != winner.receipt {
		t.Fatal("favorite receipt lost on reopen", recovered, err)
	}
	recoveredLinks, err := reopened.LookupWorkspaceLinks(t.Context(), p, file.FileID, file.VersionID, "links-restart")
	if err != nil || recoveredLinks != links {
		t.Fatal("link receipt lost on reopen", recoveredLinks, err)
	}
	view, err := reopened.WorkspaceLinks(t.Context(), p, file.FileID, file.VersionID, false, contract.PageQuery{})
	if err != nil || !view.Indexed || len(view.Items) != 0 {
		t.Fatal("empty index lost on reopen", view, err)
	}
}
