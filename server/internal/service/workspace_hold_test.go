package service_test

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldRecoveryCAS(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "fixed_delivery", SourceKind: "operation", SourceRef: op.ID, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	first, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil || first.ResultRevision != 1 {
		t.Fatal(first, err)
	}
	released, err := f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	if err != nil || released.ResultRevision != 2 {
		t.Fatal(released, err)
	}
	if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "other-release"); err == nil || err.Error() != "revision_conflict" {
		t.Fatal("double release", err)
	}
	replay, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil || replay != first {
		t.Fatal("create receipt changed", replay, err)
	}
	replay, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	if err != nil || replay != released {
		t.Fatal("release receipt changed", replay, err)
	}
	f.Service.Config.HoldManagement = false
	for action, want := range map[string]contract.WorkspaceHoldReceipt{"create": first, "release": released} {
		got, err := f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: action, IdempotencyKey: action})
		if err != nil || got != want {
			t.Fatal("recovery while disabled", got, err)
		}
	}
	if _, err = f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "disabled"); err == nil || err.Error() != "unsupported_action" {
		t.Fatal("default off", err)
	}
}

func TestWorkspaceHoldSourceTargetAuthority(t *testing.T) {
	for _, scenario := range []string{"wrong_file", "wrong_version", "wrong_kind", "tenant", "scope", "agent", "disabled", "session", "cancelled_source"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.HoldManagement = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
			q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
			switch scenario {
			case "wrong_file":
				other := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "other.txt", "other", []byte("other"))
				q.SourceRef = other.ID
			case "wrong_version":
				version := service.Version{ID: service.ID(), Owner: p.UserID, FileID: op.FileID}
				if err := f.Service.DB.Create(&version).Error; err != nil {
					t.Fatal(err)
				}
				q.VersionID = version.ID
			case "wrong_kind":
				q.SourceKind = "share"
			case "tenant":
				p.TenantID = "other"
			case "scope":
				p.ScopeSpace = "other"
			case "agent":
				p.Agent = &service.Agent{ID: service.ID()}
			case "disabled":
				if err := f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
					t.Fatal(err)
				}
			case "session":
				p.SessionHash = "revoked"
			case "cancelled_source":
				if err := f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update("status", "cancelled").Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "denied"); err == nil {
				t.Fatal("unsafe hold admitted")
			}
			var count int64
			if err := f.Service.DB.Model(&service.RetentionHold{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("denial wrote hold", count, err)
			}
		})
	}
}

func TestWorkspaceHoldListCurrentAuthorityCursor(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	first, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "second"); err != nil {
		t.Fatal(err)
	}
	page, err := f.Service.WorkspaceHolds(t.Context(), p, op.FileID, op.VersionID, contract.PageQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" || page.Total != nil {
		t.Fatal(page, err)
	}
	last, err := f.Service.WorkspaceHolds(t.Context(), p, op.FileID, op.VersionID, contract.PageQuery{Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(last.Items) != 1 || last.Items[0].HoldID == page.Items[0].HoldID {
		t.Fatal(last, err)
	}
	if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, op.VersionID, contract.PageQuery{Limit: 1, Cursor: page.NextCursor}); err == nil || err.Error() != "cursor_expired" {
		t.Fatal("stale cursor", err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{}); err == nil {
		t.Fatal("disabled actor list")
	}
	if _, err = f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "first"}); err == nil {
		t.Fatal("disabled actor recovery")
	}
}
