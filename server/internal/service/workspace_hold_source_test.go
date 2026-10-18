package service_test

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldShareBinding(t *testing.T) {
	for _, scenario := range []string{"fixed", "live", "wrong_file", "wrong_version", "wrong_space", "foreign_owner", "revoked", "expired", "unknown_mode"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.HoldManagement = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "share/held.txt", "upload", []byte("held"))
			share := service.Share{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, Path: "share", Mode: "fixed", Revision: 1}
			item := service.ShareItem{ShareID: share.ID, FileID: op.FileID, VersionID: op.VersionID, Path: "share/held.txt"}
			switch scenario {
			case "live":
				share.Mode = "live"
			case "wrong_file":
				item.FileID = service.ID()
			case "wrong_version":
				item.VersionID = service.ID()
			case "wrong_space":
				share.SpaceID = service.ID()
			case "foreign_owner":
				share.Owner = service.ID()
			case "revoked":
				share.Revoked = true
			case "expired":
				share.Expires = time.Now().Add(-time.Hour)
			case "unknown_mode":
				share.Mode = "future"
			}
			if err := f.Service.DB.Create(&share).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.Service.DB.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "fixed_delivery", SourceKind: "share", SourceRef: share.ID}
			_, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "share-hold")
			if scenario == "fixed" || scenario == "live" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unbound source admitted")
			}
		})
	}
}

func TestWorkspaceHoldLegacyCurrentAgentAndTarget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	p := service.Principal{UserID: h.user.ID, Agent: &a}
	op := fpUploadBytes(t, c, h.space.ID, "held.txt", "upload", []byte("held"))
	q := service.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "fixed_delivery", SourceRef: op.ID}
	created, err := f.Service.CreateRetentionHold(p, q, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	other := fpUploadBytes(t, c, h.space.ID, "other.txt", "other", []byte("other"))
	q.SourceRef = other.ID
	if _, err = f.Service.CreateRetentionHold(p, q, "wrong"); err == nil {
		t.Fatal("legacy wrong-target bypass")
	}
	q.SourceRef = op.ID
	if err = f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateRetentionHold(p, q, "legacy"); err == nil {
		t.Fatal("revoked replay")
	}
	if _, err = f.Service.GetRetentionHold(p, created.HoldID); err == nil {
		t.Fatal("revoked read")
	}
	if _, err = f.Service.ReleaseRetentionHold(p, created.HoldID); err == nil {
		t.Fatal("agent release")
	}
}

func TestWorkspaceHoldCurrentSourceManage(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	root, _ := login(t, f, "root")
	if err := f.Service.BootstrapGovernance(root.user.ID); err != nil {
		t.Fatal(err)
	}
	var space service.Space
	if err := f.Service.DB.First(&space, "id = ?", h.space.ID).Error; err != nil {
		t.Fatal(err)
	}
	p := service.Principal{UserID: root.user.ID, DataOwner: space.Owner, TenantID: f.Service.Instance, ScopeSpace: space.ID}
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	if _, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "reader"); err == nil {
		t.Fatal("read authority became source manage")
	}
}
