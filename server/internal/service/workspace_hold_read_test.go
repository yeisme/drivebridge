package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceHoldReadAndSourceFailures(t *testing.T) {
	for _, table := range []string{"retention_holds", "operations", "workspace_requests", "spaces", "versions"} {
		t.Run(table, func(t *testing.T) {
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
			hits := 0
			if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("hold_read_fault", func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					hits++
					tx.AddError(errors.New("synthetic query failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			_, lookupErr := f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "first"})
			_, releaseErr := f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
			if err = f.Service.DB.Callback().Query().Remove("hold_read_fault"); err != nil {
				t.Fatal(err)
			}
			if hits == 0 || lookupErr == nil || lookupErr.Error() != "database_unavailable" || releaseErr == nil || releaseErr.Error() != "database_unavailable" {
				t.Fatal(hits, lookupErr, releaseErr)
			}
			var row service.RetentionHold
			if err = f.Service.DB.First(&row, "id = ?", first.HoldID).Error; err != nil || row.Revision != 1 || row.State != "active" {
				t.Fatal(row, err)
			}
		})
	}
}

func TestWorkspaceHoldCurrentTargetMove(t *testing.T) {
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
	foreign := service.Space{ID: service.ID(), Owner: p.UserID, TenantID: "other"}
	if err = f.Service.DB.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("space_id", foreign.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{}); err == nil {
		t.Fatal("moved target list")
	}
	if _, err = f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "first"}); err == nil {
		t.Fatal("moved target receipt")
	}
	if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, first.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err == nil {
		t.Fatal("moved target release")
	}
}
