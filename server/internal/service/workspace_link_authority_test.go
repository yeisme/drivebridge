package service_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceLinksAuthorityAndBudgets(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	visible := orgUpload(t, client, h.space.ID, "link-auth/visible.md", "# Visible")
	foreign := orgUpload(t, mustSDK(t, bob), bob.space.ID, "private-secret.md", "# Hidden")
	foreignRef := f.Service.FileRef(service.File{ID: foreign.FileID, VersionID: foreign.VersionID})
	source := orgUpload(t, client, h.space.ID, "link-auth/source.md", "[visible](visible.md) [hidden]("+foreignRef+") [external](https://example.invalid/private)")
	q := contract.FileLinkRebuild{VersionID: source.VersionID}
	if _, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, q, "links-auth"); err != nil {
		t.Fatal(err)
	}
	links, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || len(links.Items) != 1 || links.Items[0].TargetPath != "link-auth/visible.md" {
		t.Fatal("unauthorized target projection", links, err)
	}
	legacy, _ := agent(t, h, "", "read,write")
	for _, bad := range []service.Principal{{UserID: p.UserID, Agent: &legacy}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err := f.Service.WorkspaceLinks(t.Context(), bad, source.FileID, source.VersionID, false, contract.PageQuery{}); err == nil {
			t.Fatal("unauthorized links")
		}
		if _, err := f.Service.RebuildWorkspaceLinks(t.Context(), bad, source.FileID, q, "links-auth"); err == nil {
			t.Fatal("unauthorized replay")
		}
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", visible.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	links, err = f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || len(links.Items) != 0 {
		t.Fatal("revoked target path/ref leaked", links, err)
	}
	f.Service.DB.Callback().Query().Before("gorm:query").Register("links_query_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_file_link_projections" {
			tx.AddError(errors.New("synthetic projection failure"))
		}
	})
	_, failure := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	f.Service.DB.Callback().Query().Remove("links_query_failure")
	if failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal("failure disguised as empty", failure)
	}
	large := orgUpload(t, client, h.space.ID, "link-auth/large.md", strings.Repeat("[link](visible.md) ", 1001))
	if _, err = f.Service.RebuildWorkspaceLinks(t.Context(), p, large.FileID, contract.FileLinkRebuild{VersionID: large.VersionID}, "links-over-budget"); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("link budget ignored", err)
	}
	var count int64
	if err = f.Service.DB.Model(&service.WorkspaceFileLinkIndex{}).Where("file_id = ?", large.FileID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("budget wrote partial index", count, err)
	}
}
