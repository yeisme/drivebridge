package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceHoldAtomicReceiptFailures(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	fault := func() {
		if err := f.Service.DB.Callback().Create().Before("gorm:create").Register("hold_receipt_fault", func(tx *gorm.DB) {
			if tx.Statement.Table == "workspace_requests" {
				tx.AddError(errors.New("synthetic receipt failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	remove := func() {
		if err := f.Service.DB.Callback().Create().Remove("hold_receipt_fault"); err != nil {
			t.Fatal(err)
		}
	}
	fault()
	_, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	remove()
	if err == nil || err.Error() != "database_unavailable" {
		t.Fatal(err)
	}
	var count int64
	if err = f.Service.DB.Model(&service.RetentionHold{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("partial create", count, err)
	}
	created, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil {
		t.Fatal(err)
	}
	fault()
	_, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	remove()
	if err == nil || err.Error() != "database_unavailable" {
		t.Fatal(err)
	}
	var hold service.RetentionHold
	if err = f.Service.DB.First(&hold, "id = ?", created.HoldID).Error; err != nil || hold.State != "active" || hold.Revision != 1 {
		t.Fatal("partial release", hold, err)
	}
	if _, err = f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "release", IdempotencyKey: "release"}); err == nil || err.Error() != "not_found" {
		t.Fatal("rollback receipt", err)
	}
}

func TestWorkspaceHoldIsolationAndImmutableDigest(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	first, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		id       string
		revision uint64
	}{{first.HoldID, 2}, {second.HoldID, 1}} {
		if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, input.id, contract.WorkspaceHoldRelease{ExpectedRevision: input.revision}, "release"); err == nil || err.Error() != "idempotency_conflict" {
			t.Fatal("digest omitted target/revision", err)
		}
	}
	q.Purpose = "transfer_window"
	if _, err = f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("create digest", err)
	}
	bob, _ := login(t, f, "bob")
	for _, bad := range []service.Principal{{UserID: bob.user.ID}, {UserID: p.UserID, TenantID: "other"}, {UserID: p.UserID, ScopeSpace: "other"}, {UserID: p.UserID, Agent: &service.Agent{ID: service.ID()}}, {UserID: p.UserID, SessionHash: "revoked"}, {UserID: p.UserID, ExternalRead: true}} {
		if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), bad, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err == nil {
			t.Fatal("unauthorized replay")
		}
		if _, err = f.Service.LookupWorkspaceHold(t.Context(), bad, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "create"}); err == nil {
			t.Fatal("unauthorized receipt")
		}
		if _, err = f.Service.WorkspaceHolds(t.Context(), bad, op.FileID, "", contract.PageQuery{}); err == nil {
			t.Fatal("unauthorized list")
		}
	}
	if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update("status", "cancelled").Error; err != nil {
		t.Fatal(err)
	}
	got, err := f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "create"})
	if err != nil || got != first {
		t.Fatal("source lifecycle changed original receipt", got, err)
	}
}
