package service_test

import (
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceLinksHTTPPaginationAndCursorBinding(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	client := mustSDK(t, h)
	p := service.Principal{UserID: h.user.ID}
	target := orgUpload(t, client, h.space.ID, "link-page/target.md", "# Target")
	source := orgUpload(t, client, h.space.ID, "link-page/source.md", "[a](target.md#a) [b](target.md#b) [c](target.md#c)")
	if _, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, contract.FileLinkRebuild{VersionID: source.VersionID}, "links-page"); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"/files/" + source.FileID + "/links?version_id=" + source.VersionID, "/files/" + target.FileID + "/backlinks?version_id=" + target.VersionID} {
		var first, second contract.FileLinkList
		if status := h.call(t, "GET", base+"&limit=2", nil, &first); status != 200 || len(first.Items) != 2 || first.NextCursor == "" || first.Total != nil || first.TotalKind != "unknown" || first.ObservedAt.IsZero() {
			t.Fatal(status, first)
		}
		if status := h.call(t, "GET", base+"&limit=2&cursor="+url.QueryEscape(first.NextCursor), nil, &second); status != 200 || len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].Anchor == first.Items[1].Anchor {
			t.Fatal(status, second)
		}
		if status := h.call(t, "GET", base+"&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
			t.Fatal("link limit binding", status)
		}
	}
	var first contract.FileLinkList
	base := "/files/" + source.FileID + "/links?version_id=" + source.VersionID
	if status := h.call(t, "GET", base+"&limit=1", nil, &first); status != 200 {
		t.Fatal(status)
	}
	if status := h.call(t, "GET", "/files/"+source.FileID+"/backlinks?version_id="+source.VersionID+"&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("cursor direction binding", status)
	}
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", target.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "GET", base+"&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
		t.Fatal("permission generation binding", status)
	}
	for _, limit := range []string{"0", "201", "x", "1&limit=1"} {
		if status := h.call(t, "GET", base+"&limit="+limit, nil, nil); status != 400 {
			t.Fatal("invalid page limit", status)
		}
	}
}
