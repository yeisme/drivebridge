package service_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceLinksDefaultMaximumPagesAndScanBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	target := orgUpload(t, client, h.space.ID, "link-budget/target.md", "# Target")
	var body strings.Builder
	for i := range 201 { fmt.Fprintf(&body, "[link](target.md#%04d)\n", i) }
	source := orgUpload(t, client, h.space.ID, "link-budget/source.md", body.String())
	if _, err := f.Service.RebuildWorkspaceLinks(t.Context(), p, source.FileID, contract.FileLinkRebuild{VersionID: source.VersionID}, "links-budget-pages"); err != nil { t.Fatal(err) }
	first, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{})
	if err != nil || len(first.Items) != 50 || first.NextCursor == "" { t.Fatal("default page", len(first.Items), err) }
	maximum, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{Limit: 200})
	if err != nil || len(maximum.Items) != 200 || maximum.NextCursor == "" { t.Fatal("maximum page", len(maximum.Items), err) }
	last, err := f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{Limit: 200, Cursor: maximum.NextCursor})
	if err != nil || len(last.Items) != 1 || last.NextCursor != "" { t.Fatal("maximum continuation", last, err) }
	rows := make([]service.WorkspaceFileLinkProjection, 800)
	for i := range rows {
		rows[i] = service.WorkspaceFileLinkProjection{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, FileID: source.FileID, VersionID: source.VersionID, TargetRef: f.Service.FileRef(service.File{ID: target.FileID, VersionID: target.VersionID}), Anchor: fmt.Sprintf("extra-%04d", i)}
	}
	if err = f.Service.DB.CreateInBatches(rows, 100).Error; err != nil { t.Fatal(err) }
	if _, err = f.Service.WorkspaceLinks(t.Context(), p, source.FileID, source.VersionID, false, contract.PageQuery{Limit: 1}); err == nil || err.Error() != "query_budget_exceeded" { t.Fatal("small page bypassed scan budget", err) }
}
