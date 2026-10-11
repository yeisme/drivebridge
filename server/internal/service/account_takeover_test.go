package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestAdminNameConfirmationAndAtomicTakeover(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	owner, _ := login(t, f, "owner")
	other, _ := login(t, f, "other")
	if err := f.Service.BootstrapGovernance(root.user.ID); err != nil {
		t.Fatal(err)
	}
	tenant := service.Tenant{ID: service.ID(), Name: "Handoff tenant"}
	f.Service.DB.Create(&tenant)
	f.Service.DB.Create(&service.Membership{ID: service.ID(), TenantID: tenant.ID, UserID: owner.user.ID, DataOwner: service.ID(), Role: "admin", Status: "active"})
	team := service.Team{ID: service.ID(), TenantID: tenant.ID, Name: "Handoff team", Revision: 1}
	f.Service.DB.Create(&team)
	f.Service.DB.Create(&service.TeamMember{TeamID: team.ID, UserID: owner.user.ID, Role: "admin"})
	space := service.Space{ID: service.ID(), Owner: service.ID(), TenantID: tenant.ID, TeamID: team.ID, Name: "Handoff drive"}
	f.Service.DB.Create(&space)
	f.Service.DB.Create(&service.SpacePermission{SpaceID: space.ID, UserID: owner.user.ID, Role: "custodian"})
	var impact struct {
		User            service.User
		Allowed         bool
		TakeoverAllowed bool   `json:"takeover_allowed"`
		Digest          string `json:"takeover_digest"`
		Handoffs        []struct{ Kind string }
	}
	base := "/api/v1/governance/users/" + owner.user.ID
	if status := govCall(t, root, f.Service.Instance, "", "GET", base+"/impact?action=archive", nil, &impact); status != 200 || impact.Allowed || !impact.TakeoverAllowed || len(impact.Handoffs) != 3 {
		t.Fatal("takeover plan incomplete", status)
	}
	request := map[string]any{"action": "archive", "expected_revision": impact.User.Revision, "confirm": true, "confirmation_name": "wrong", "takeover": true, "takeover_digest": impact.Digest}
	if status := govCall(t, root, f.Service.Instance, "", "POST", base+"/action", request, nil); status != 400 {
		t.Fatal("wrong name accepted", status)
	}
	request["confirmation_name"] = owner.user.Name
	if status := govCall(t, other, f.Service.Instance, "", "POST", base+"/action", request, nil); status != 403 {
		t.Fatal("non-platform takeover accepted", status)
	}
	request["takeover_digest"] = "stale"
	if status := govCall(t, root, f.Service.Instance, "", "POST", base+"/action", request, nil); status != 409 {
		t.Fatal("stale plan accepted", status)
	}
	var count int64
	f.Service.DB.Model(&service.TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, root.user.ID).Count(&count)
	if count != 0 {
		t.Fatal("rejected operation assigned role")
	}
	request["takeover_digest"] = impact.Digest
	// Fixture OIDC sessions have no recent authentication proof. Exact target
	// name is the explicit alternative, not a new authentication mechanism.
	if status := govCall(t, root, f.Service.Instance, "", "POST", base+"/action", request, nil); status != 200 {
		t.Fatal("confirmed takeover rejected", status)
	}
	var user service.User
	f.Service.DB.First(&user, "id = ?", owner.user.ID)
	if !user.Archived || !user.Disabled {
		t.Fatal("target not archived")
	}
	var membership service.Membership
	f.Service.DB.First(&membership, "tenant_id = ? AND user_id = ?", tenant.ID, root.user.ID)
	var teamMember service.TeamMember
	f.Service.DB.First(&teamMember, "team_id = ? AND user_id = ?", team.ID, root.user.ID)
	var permission service.SpacePermission
	f.Service.DB.First(&permission, "space_id = ? AND user_id = ?", space.ID, root.user.ID)
	if membership.Role != "admin" || membership.Status != "active" || teamMember.Role != "admin" || permission.Role != "custodian" {
		t.Fatal("responsibility handoff incomplete")
	}
	var events int64
	f.Service.DB.Model(&service.GovernanceEvent{}).Where("action = ?", "account.responsibility_taken").Count(&events)
	if events != 3 {
		t.Fatal("handoffs not audited", events)
	}
}
