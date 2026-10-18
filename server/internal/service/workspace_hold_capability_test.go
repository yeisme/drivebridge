package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldCapabilitiesAndSourceRevoke(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	share := service.Share{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, Mode: "fixed", Revision: 1}
	if err := f.Service.DB.Create(&share).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Create(&service.ShareItem{ShareID: share.ID, FileID: op.FileID, VersionID: op.VersionID}).Error; err != nil {
		t.Fatal(err)
	}
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "fixed_delivery", SourceKind: "share", SourceRef: share.ID}
	created, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "first")
	if err != nil {
		t.Fatal(err)
	}
	page, err := f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{})
	if err != nil || !page.ManagementEnabled || len(page.Items) != 1 || len(page.Items[0].AllowedActions) != 1 || page.Items[0].AllowedActions[0] != "release" {
		t.Fatal(page, err)
	}
	f.Service.Config.HoldManagement = false
	page, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{})
	if err != nil || page.ManagementEnabled || page.Items[0].AllowedActions == nil || len(page.Items[0].AllowedActions) != 0 {
		t.Fatal(page, err)
	}
	f.Service.Config.HoldManagement = true
	if err = f.Service.DB.Model(&service.Share{}).Where("id = ?", share.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	got, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "first")
	if err != nil || got != created {
		t.Fatal("original receipt after source revocation", got, err)
	}
	if _, err = f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "new"); err == nil {
		t.Fatal("revoked source created new protection")
	}
	page, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{})
	if err != nil || page.Items[0].State != "inactive_source_or_expiry" {
		t.Fatal(page, err)
	}
	if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err != nil {
		t.Fatal("owner cannot release revoked source", err)
	}
	page, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{})
	if err != nil || page.Items[0].State != "released" || len(page.Items[0].AllowedActions) != 0 {
		t.Fatal(page, err)
	}
}
