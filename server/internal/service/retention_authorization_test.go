package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestRetentionCurrentAuthorization(t *testing.T) {
	for _, scenario := range []string{"external", "share", "access_grant", "expired_session", "disabled", "archived", "foreign_actor", "scope", "tenant", "governance_reader"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "permission.txt", "permission", []byte("protected"))
			if e := f.Service.Trash(p, op.FileID, false); e != nil {
				t.Fatal(e)
			}
			if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; e != nil {
				t.Fatal(e)
			}
			preview, e := f.Service.PreviewRetention(p, "trash")
			if e != nil {
				t.Fatal(e)
			}
			switch scenario {
			case "external":
				p.ExternalRead = true
			case "share":
				p.ShareID = "synthetic-share"
			case "access_grant":
				p.AccessGrantHash = "synthetic-grant"
			case "expired_session":
				p.SessionHash = "missing-session"
			case "disabled":
				if e := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; e != nil {
					t.Fatal(e)
				}
			case "archived":
				if e := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("archived", true).Error; e != nil {
					t.Fatal(e)
				}
			case "foreign_actor":
				other, _ := login(t, f, "bob")
				p.UserID = other.user.ID
				p.DataOwner = h.user.ID
			case "scope":
				p.ScopeSpace = "other-space"
			case "tenant":
				p.TenantID = "other-tenant"
			case "governance_reader":
				root, _ := login(t, f, "root")
				if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
					t.Fatal(e)
				}
				var space service.Space
				if e := f.Service.DB.First(&space, "id = ?", h.space.ID).Error; e != nil {
					t.Fatal(e)
				}
				// Platform admin can observe but has no implicit delete permission.
				p = service.Principal{UserID: root.user.ID, DataOwner: space.Owner, TenantID: f.Service.Instance, ScopeSpace: space.ID}
				if e := f.Service.DB.Model(&service.RetentionPlan{}).Where("id = ?", preview["plan_id"]).Update("owner", space.Owner).Error; e != nil {
					t.Fatal(e)
				}
			}
			out, err := f.Service.ExecuteRetention(context.Background(), p, preview["plan_id"].(string))
			if scenario == "scope" || scenario == "tenant" || scenario == "governance_reader" {
				if err != nil || out["status"] != "failed" || fpNum(out["items_deleted"]) != 0 {
					t.Fatalf("authorization verdict missing: %v %v", out, err)
				}
				var item service.RetentionItem
				if e := f.Service.DB.Where("plan_id = ?", preview["plan_id"]).First(&item).Error; e != nil {
					t.Fatal(e)
				}
				want := "not_found"
				if scenario == "governance_reader" {
					want = "approval_required"
				}
				if item.Detail != want {
					t.Fatalf("wrong denial: %s", item.Detail)
				}
			} else {
				want := "forbidden"
				if scenario == "expired_session" {
					want = "unauthorized"
				}
				if err == nil || err.Error() != want || out != nil {
					t.Fatalf("human admission not refused: %v %v", out, err)
				}
			}
			var count int64
			if e := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Count(&count).Error; e != nil || count != 1 {
				t.Fatalf("unauthorized file loss: %d %v", count, e)
			}
			if e := f.Service.DB.Model(&service.Version{}).Where("id = ?", op.VersionID).Count(&count).Error; e != nil || count != 1 {
				t.Fatalf("unauthorized version loss: %d %v", count, e)
			}
		})
	}
}
