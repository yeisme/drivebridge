package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldRecoveryRechecksCurrentSource(t *testing.T) {
	for _, change := range []string{"owner", "file", "version", "space"} {
		t.Run(change, func(t *testing.T) {
			f := rig(t)
			f.Service.Config.HoldManagement = true
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
			q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
			created, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
			if err != nil { t.Fatal(err) }
			if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err != nil { t.Fatal(err) }
			column := map[string]string{"owner": "owner", "file": "file_id", "version": "version_id", "space": "space_id"}[change]
			if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update(column, service.ID()).Error; err != nil { t.Fatal(err) }
			if _, err = f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create"); err == nil { t.Fatal("create replay trusted obsolete source") }
			if _, err = f.Service.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release"); err == nil { t.Fatal("release replay trusted obsolete source") }
			if _, err = f.Service.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: "create", IdempotencyKey: "create"}); err == nil { t.Fatal("lookup trusted obsolete source") }
			if _, err = f.Service.WorkspaceHolds(t.Context(), p, op.FileID, "", contract.PageQuery{}); err == nil { t.Fatal("list trusted obsolete source") }
		})
	}
}
