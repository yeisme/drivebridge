package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestTeamSelfCustodianAndQuota(t *testing.T) {
	f := rig(t)
	f.Service.Config.RequireExplicitAdminRead = true
	root, _ := login(t, f, "root")
	other, _ := login(t, f, "other")
	if err := f.Service.BootstrapGovernance(root.user.ID); err != nil {
		t.Fatal(err)
	}
	call := func(h *human, a string, q map[string]any, out any) int {
		return govCall(t, h, f.Service.Instance, "", "POST", "/api/v1/governance/"+a, q, out)
	}
	var team service.Team
	if st := call(root, "teams", map[string]any{"name": "yeisme", "user_id": root.user.ID}, &team); st != 200 {
		t.Fatal(st)
	}
	var sp service.Space
	if st := call(root, "spaces", map[string]any{"team_id": team.ID, "user_id": root.user.ID, "name": "test", "max_bytes": int64(1 << 30)}, &sp); st != 200 {
		t.Fatal("self custodian", st)
	}
	if sp.MaxBytes != 1<<30 {
		t.Fatal(sp.MaxBytes)
	}
	var permission service.SpacePermission
	if e := f.Service.DB.First(&permission, "space_id = ? AND user_id = ?", sp.ID, root.user.ID).Error; e != nil || permission.Role != "custodian" {
		t.Fatal("missing explicit permission")
	}
	if st := call(other, "space-quota", map[string]any{"team_id": team.ID, "space_id": sp.ID, "max_bytes": 200}, nil); st != 403 {
		t.Fatal("unauthorized quota", st)
	}
	f.Service.DB.Model(&service.Space{}).Where("id = ?", sp.ID).Updates(map[string]any{"used": 100, "reserved": 20})
	if st := call(root, "space-quota", map[string]any{"team_id": team.ID, "space_id": sp.ID, "max_bytes": 119}, nil); st < 400 {
		t.Fatal("underflow accepted")
	}
	if st := call(root, "space-quota", map[string]any{"team_id": team.ID, "space_id": sp.ID, "max_bytes": 120}, nil); st != 200 {
		t.Fatal("exact occupied", st)
	}
	if st := call(root, "spaces", map[string]any{"team_id": team.ID, "user_id": root.user.ID, "name": "bad", "max_bytes": 0}, nil); st < 400 {
		t.Fatal("zero accepted")
	}
	var legacy service.Space
	if st := call(root, "spaces", map[string]any{"team_id": team.ID, "user_id": root.user.ID, "name": "legacy"}, &legacy); st != 200 {
		t.Fatal(st)
	}
	if legacy.MaxBytes != f.Service.Config.MaxBytes {
		t.Fatal("default changed")
	}
	var otherTeam service.Team
	if st := call(root, "teams", map[string]any{"name": "other team", "user_id": other.user.ID}, &otherTeam); st != 200 {
		t.Fatal(st)
	}
	var otherSpace service.Space
	if st := call(root, "spaces", map[string]any{"team_id": otherTeam.ID, "user_id": other.user.ID, "name": "other space"}, &otherSpace); st != 200 {
		t.Fatal(st)
	}
	if st := govCall(t, root, f.Service.Instance, otherSpace.ID, "GET", "/api/v1/browser?space_id="+otherSpace.ID, nil, nil); st == 200 {
		t.Fatal("administrator bypassed explicit read grants")
	}

}
