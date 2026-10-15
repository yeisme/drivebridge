package service_test

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestAdminDirectWriteAndWorkspaceWithoutMembership(t *testing.T) {
	f := rig(t)
	admin, _ := login(t, f, "admin")
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	if err := f.Service.BootstrapGovernance(admin.user.ID); err != nil {
		t.Fatal(err)
	}
	tenant := f.Service.Instance
	if err := f.Service.DB.Where("user_id = ?", admin.user.ID).Delete(&service.Membership{}).Error; err != nil {
		t.Fatal(err)
	}
	if st := govCall(t, admin, tenant, "", "GET", "/api/v1/agent-grants", nil, nil); st != 403 {
		t.Fatalf("default gate: %d", st)
	}
	f.Service.Config.AdminDirectWrite = true
	for _, endpoint := range []string{"/api/v1/agent-grants", "/api/v1/spaces", "/api/v1/connections"} {
		if st := govCall(t, admin, tenant, "", "GET", endpoint, nil, nil); st != 200 {
			t.Fatalf("admin workspace %s: %d", endpoint, st)
		}
	}
	for _, h := range []*human{admin, bob} {
		want := 200
		if h == bob {
			want = 403
		}
		if st := govCall(t, h, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "admin-written"}, nil); st != want {
			t.Fatalf("scoped write want %d got %d", want, st)
		}
	}
	if st := govCall(t, admin, service.ID(), "", "GET", "/api/v1/agent-grants", nil, nil); st != 403 {
		t.Fatalf("unknown tenant accepted: %d", st)
	}
	if err := f.Service.DB.Model(&service.PlatformRole{}).Where("user_id = ?", admin.user.ID).Update("suspended", true).Error; err != nil {
		t.Fatal(err)
	}
	if st := govCall(t, admin, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "revoked"}, nil); st != 403 {
		t.Fatalf("suspended admin write: %d", st)
	}
}

func TestRetireSpacesRejectsUnknownOperationsAndHidesExactSpaces(t *testing.T) {
	f := rig(t)
	admin, _ := login(t, f, "admin")
	alice, _ := login(t, f, "alice")
	if err := f.Service.BootstrapGovernance(admin.user.ID); err != nil {
		t.Fatal(err)
	}
	op := service.Operation{ID: service.ID(), Owner: alice.space.Owner, SpaceID: alice.space.ID, Kind: "upload", Status: "publishing", Size: 51}
	if err := f.Service.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.RetireSpaces([]string{alice.space.ID}); err == nil {
		t.Fatal("unknown operation must be reconciled")
	}
	if err := f.Service.DB.Model(&op).Update("status", "awaiting_upload").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.RetireSpaces([]string{alice.space.ID, service.ID()}); err == nil {
		t.Fatal("unknown ID must reject whole batch")
	}
	if err := f.Service.RetireSpaces([]string{alice.space.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.Service.RetireSpaces([]string{alice.space.ID}); err != nil {
		t.Fatal("retry must not duplicate retirement", err)
	}
	var spaces []service.Space
	if err := f.Service.DB.Find(&spaces).Error; err != nil {
		t.Fatal(err)
	}
	for _, sp := range spaces {
		if sp.ID == alice.space.ID {
			t.Fatal("retired space remains active")
		}
	}
	var retired service.Space
	if err := f.Service.DB.Unscoped().First(&retired, "id = ?", alice.space.ID).Error; err != nil || !retired.DeletedAt.Valid {
		t.Fatal("retirement is not recoverable", err)
	}
	if err := f.Service.DB.First(&op, "id = ?", op.ID).Error; err != nil || op.Status != "cancelled" {
		t.Fatal("pending upload not cancelled", err)
	}
	if st := govCall(t, alice, f.Service.Instance, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 403 {
		t.Fatalf("retired space still accessible: %d", st)
	}
}
