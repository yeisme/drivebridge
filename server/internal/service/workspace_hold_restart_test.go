package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldRestartOriginalReceipts(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}
	created, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil {
		t.Fatal(err)
	}
	released, err := f.Service.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	if err != nil {
		t.Fatal(err)
	}
	cfg := f.Service.Config
	f.App.Close()
	f.Service.Close()
	reopened, err := service.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for action, want := range map[string]contract.WorkspaceHoldReceipt{"create": created, "release": released} {
		got, err := reopened.LookupWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldLookup{Action: action, IdempotencyKey: action})
		if err != nil || got != want {
			t.Fatal("durable receipt changed", got, err)
		}
	}
	got, err := reopened.CreateWorkspaceHold(t.Context(), p, op.FileID, q, "create")
	if err != nil || got != created {
		t.Fatal("create replay changed after reopen", got, err)
	}
	got, err = reopened.ReleaseWorkspaceHold(t.Context(), p, created.HoldID, contract.WorkspaceHoldRelease{ExpectedRevision: 1}, "release")
	if err != nil || got != released {
		t.Fatal("release replay changed after reopen", got, err)
	}
}
