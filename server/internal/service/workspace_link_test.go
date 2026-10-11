package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceLinksFixedVersionRebuildAndVisibility(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	target := orgUpload(t, client, h.space.ID, "links/target.md", "# Target")
	ref := f.Service.FileRef(service.File{ID: target.FileID, VersionID: target.VersionID})
	source := orgUpload(t, client, h.space.ID, "links/source.md", "[target]("+ref+"#section)\n[relative](target.md#section)\n`[code](target.md#ignored)`\n```md\n[code](target.md#ignored)\n```\n")
	q := contract.FileLinkRebuild{VersionID: source.VersionID}
	first, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, q, "links-build")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, contract.FileLinkRebuild{VersionID: target.VersionID}, "wrong-file-version"); err == nil || err.Error() != "not_found" { t.Fatal("foreign version accepted", err) }
	links, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || !links.Indexed || links.Stale || len(links.Items) != 1 || links.Items[0].TargetRef != ref || links.Items[0].Anchor != "section" {
		t.Fatal(links, err)
	}
	back, err := f.Service.WorkspaceLinks(t.Context(), p, target.FileID, target.VersionID, true, contract.PageQuery{})
	if err != nil || len(back.Items) != 1 {
		t.Fatal(back, err)
	}
	var version service.Version
	if err = f.Service.DB.First(&version, "id = ?", source.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	version.ID = service.ID()
	if err = f.Service.DB.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", source.FileID).Update("version_id", version.ID).Error; err != nil {
		t.Fatal(err)
	}
	links, err = f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || !links.Stale || !links.Items[0].Stale {
		t.Fatal("head drift hidden", links, err)
	}
	replay, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, q, "links-build")
	if err != nil || replay != first {
		t.Fatal("rebuild replay changed receipt", err)
	}
	if _, err = f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, contract.FileLinkRebuild{VersionID: version.ID}, "links-build"); err == nil || err.Error() != "idempotency_conflict" { t.Fatal("same key retargeted source version", err) }
	if err = f.Service.DB.Where("file_id = ?", source.FileID).Delete(&service.WorkspaceFileLinkProjection{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, q, "links-repair"); err != nil {
		t.Fatal("projection not rebuildable", err)
	}
	if _, err = f.Service.WorkspaceLinks(t.Context(), service.Principal{UserID: bob.user.ID}, source.FileID, source.VersionID, false, contract.PageQuery{}); err == nil {
		t.Fatal("foreign source leaked")
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", source.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	back, err = f.Service.WorkspaceLinks(t.Context(), p, target.FileID, target.VersionID, true, contract.PageQuery{})
	if err != nil || len(back.Items) != 0 {
		t.Fatal("hidden backlink leaked", back, err)
	}
}
