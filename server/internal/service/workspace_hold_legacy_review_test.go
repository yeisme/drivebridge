package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceHoldLegacyReleaseCannotBypassManagedCAS(t *testing.T) {
	f := rig(t)
	f.Service.Config.HoldManagement = true
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	created, err := f.Service.CreateWorkspaceHold(t.Context(), p, op.FileID, contract.WorkspaceHoldCreate{VersionID: op.VersionID, Purpose: "legal_hold", SourceKind: "operation", SourceRef: op.ID}, "managed")
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		f.Service.Config.HoldManagement = enabled
		for _, input := range []any{nil, map[string]any{"expected_revision": 0}, map[string]any{"expected_revision": 1}} {
			if status := h.call(t, "POST", "/retention/holds/"+created.HoldID+"/release", input, nil); status != 422 {
				t.Fatal("legacy bypass", enabled, status)
			}
		}
	}
	var row service.RetentionHold
	if err = f.Service.DB.First(&row, "id = ?", created.HoldID).Error; err != nil || row.State != "active" || row.Revision != 1 {
		t.Fatal("managed hold mutated", row, err)
	}
	f.Service.Config.HoldManagement = false
	legacy, err := f.Service.CreateRetentionHold(p, service.RetentionHoldRequest{FileID: op.FileID, VersionID: op.VersionID, Purpose: "legal_hold", SourceRef: op.ID}, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "POST", "/retention/holds/"+legacy.HoldID+"/release", nil, nil); status != 200 {
		t.Fatal("legacy compatibility", status)
	}
}

func TestWorkspaceHoldLegacyOmittedVersionReplay(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	q := service.RetentionHoldRequest{FileID: op.FileID, Purpose: "legal_hold", SourceRef: op.ID}
	first, err := f.Service.CreateRetentionHold(p, q, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	version := service.Version{ID: service.ID(), Owner: p.UserID, FileID: op.FileID}
	if err = f.Service.DB.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("version_id", version.ID).Error; err != nil {
		t.Fatal(err)
	}
	got, err := f.Service.CreateRetentionHold(p, q, "legacy")
	if err != nil || got != first {
		t.Fatal("rebound omitted version", got, err)
	}
	q.VersionID = version.ID
	if _, err = f.Service.CreateRetentionHold(p, q, "legacy"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("changed explicit version", err)
	}
}

func TestWorkspaceHoldLegacyReadSourceAuthority(t *testing.T) {
	for _, scenario := range []string{"agent", "owner", "file_id", "version_id", "space_id"} {
		t.Run(scenario, func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			a, client := agent(t, h, "", "read,write")
			p := service.Principal{UserID: h.user.ID, Agent: &a}
			op := fpUploadBytes(t, client, h.space.ID, "held.txt", "upload", []byte("held"))
			hold, err := f.Service.CreateRetentionHold(p, service.RetentionHoldRequest{FileID: op.FileID, Purpose: "legal_hold", SourceRef: op.ID}, "legacy")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "agent" {
				other, _ := agent(t, h, "", "read,write")
				p.Agent = &other
			} else if err = f.Service.DB.Model(&service.Operation{}).Where("id = ?", op.ID).Update(scenario, service.ID()).Error; err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.GetRetentionHold(p, hold.HoldID); err == nil {
				t.Fatal("legacy source bypass")
			}
			if _, err = f.Service.CallTool(t.Context(), p, "retention_hold_get", map[string]any{"hold_id": hold.HoldID}); err == nil {
				t.Fatal("MCP source bypass")
			}
		})
	}
}
