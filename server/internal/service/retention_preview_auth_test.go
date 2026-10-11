package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestRetentionPreviewCurrentReadAuthorization(t *testing.T) {
	for _, scenario := range []string{"scope", "tenant", "prefix", "revoked", "disabled", "external", "foreign_actor", "history_prefix"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			for _, name := range []string{"allowed/one.txt", "hidden/two.txt"} {
				op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, name, name, []byte(name))
				if err := f.Service.Trash(p, op.FileID, false); err != nil {
					t.Fatal(err)
				}
				if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "scope":
				p.ScopeSpace = "other"
			case "tenant":
				p.TenantID = "other"
			case "history_prefix":
				a, _ := agent(t, h, "allowed", "read")
				p.Agent = &a
				var file service.File
				if e := f.Service.DB.Where("path = ? AND owner = ?", "hidden/two.txt", h.user.ID).First(&file).Error; e != nil {
					t.Fatal(e)
				}
				var old service.Version
				if e := f.Service.DB.First(&old, "id = ?", file.VersionID).Error; e != nil {
					t.Fatal(e)
				}
				old.ID = service.ID()
				old.Created = time.Now().Add(-400 * 24 * time.Hour)
				if e := f.Service.DB.Create(&old).Error; e != nil {
					t.Fatal(e)
				}
			case "prefix":
				a, _ := agent(t, h, "allowed", "read")
				p.Agent = &a
			case "revoked":
				a, _ := agent(t, h, "", "read")
				p.Agent = &a
				if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true).Error; err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if err := f.Service.DB.Model(&service.User{}).Where("id = ?", h.user.ID).Update("disabled", true).Error; err != nil {
					t.Fatal(err)
				}
			case "external":
				p.ExternalRead = true
			case "foreign_actor":
				b, _ := login(t, f, "bob")
				p.UserID = b.user.ID
				p.DataOwner = h.user.ID
			}
			category := "trash"
			if scenario == "history_prefix" {
				category = "historical_versions"
			}
			out, err := f.Service.PreviewRetention(p, category)
			if err == nil || out != nil {
				t.Errorf("preview exposed inaccessible targets: %v %v", out, err)
			}
			for _, model := range []any{&service.RetentionPlan{}, &service.RetentionItem{}} {
				var count int64
				if e := f.Service.DB.Model(model).Count(&count).Error; e != nil || count != 0 {
					t.Errorf("partial plan persisted: %T %d %v", model, count, e)
				}
			}
		})
	}
}

func TestRetentionPreviewAgentReadPositive(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "allowed/one.txt", "read-positive", []byte("fixture"))
	if err := f.Service.Trash(p, op.FileID, false); err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed_at", time.Now().Add(-400*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	a, c := agent(t, h, "allowed", "read")
	p.Agent = &a
	out, err := f.Service.PreviewRetention(p, "trash")
	if err != nil || fpNum(out["items_total"]) != 1 {
		t.Fatalf("valid Agent rejected: %v %v", out, err)
	}
	viaSDK, err := c.PreviewRetention(context.Background(), "trash")
	if err != nil || fpNum(viaSDK["items_total"]) != 1 {
		t.Fatalf("SDK preview: %v %v", viaSDK, err)
	}
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	// No historical candidates exist, but a revoked Agent must not create
	// even an empty durable preview through a cached application principal.
	if result, err := f.Service.PreviewRetention(p, "historical_versions"); err == nil || result != nil {
		t.Fatalf("revoked empty preview: %v %v", result, err)
	}
}
