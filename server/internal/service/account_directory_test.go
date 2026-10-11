package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestAccountDirectoryLifecycleAndMetadata(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	if err := f.Service.BootstrapGovernance(root.user.ID); err != nil {
		t.Fatal(err)
	}
	f.Service.DB.Model(&service.Session{}).Where("owner = ?", root.user.ID).Update("authenticated_at", time.Now().UTC())
	tenant := f.Service.Instance
	secret := service.ID() + service.ID()
	key := service.UserKey{ID: service.ID(), Owner: alice.user.ID, Name: "Phone credential", TokenHash: service.Hash(secret), Created: time.Now()}
	f.Service.DB.Create(&key)
	file := service.File{ID: service.ID(), Owner: alice.user.ID, SpaceID: alice.space.ID, Path: "retained.txt"}
	f.Service.DB.Create(&file)
	call := func(method, path string, body, out any) int {
		return govCall(t, root, tenant, "", method, "/api/v1/governance/users"+path, body, out)
	}
	var listed struct {
		Items []struct {
			ID       string
			KeyCount *int64 `json:"key_count"`
		}
		Total int64
	}
	if st := call("GET", "?q=Phone", nil, &listed); st != 200 || len(listed.Items) != 1 || listed.Items[0].ID != alice.user.ID || listed.Items[0].KeyCount == nil || *listed.Items[0].KeyCount != 1 {
		t.Fatal("key metadata search failed", st)
	}
	var detail json.RawMessage
	if st := call("GET", "/"+alice.user.ID, nil, &detail); st != 200 {
		t.Fatal(st)
	}
	if bytes.Contains(detail, []byte(secret)) || bytes.Contains(detail, []byte(key.TokenHash)) {
		t.Fatal("secret exposed in account metadata")
	}
	if st := govCall(t, bob, tenant, "", "GET", "/api/v1/governance/users/"+alice.user.ID, nil, nil); st != 404 {
		t.Fatal("unrelated account exposed", st)
	}
	change := func(action string, revision uint64) int {
		return call("POST", "/"+alice.user.ID+"/action", map[string]any{"action": action, "expected_revision": revision, "confirm": true}, nil)
	}
	var current service.User
	f.Service.DB.First(&current, "id = ?", alice.user.ID)
	if st := change("freeze", current.Revision); st != 200 {
		t.Fatal("freeze failed", st)
	}
	if st := govCall(t, alice, tenant, "", "GET", "/api/v1/governance", nil, nil); st == 200 {
		t.Fatal("frozen session survived")
	}
	if st := change("unfreeze", current.Revision); st != 409 {
		t.Fatal("stale revision accepted", st)
	}
	f.Service.DB.First(&current, "id = ?", alice.user.ID)
	if st := change("unfreeze", current.Revision); st != 200 {
		t.Fatal("unfreeze failed", st)
	}
	if st := govCall(t, alice, tenant, "", "GET", "/api/v1/governance", nil, nil); st == 200 {
		t.Fatal("old session resurrected")
	}
	f.Service.DB.First(&current, "id = ?", alice.user.ID)
	if st := change("archive", current.Revision); st != 200 {
		t.Fatal("archive failed", st)
	}
	f.Service.DB.First(&current, "id = ?", alice.user.ID)
	f.Service.DB.First(&key, "id = ?", key.ID)
	if !current.Archived || !current.Disabled || !key.Revoked {
		t.Fatal("archive did not deactivate credentials")
	}
	var count int64
	f.Service.DB.Model(&service.File{}).Where("id = ?", file.ID).Count(&count)
	if count != 1 {
		t.Fatal("archive deleted files")
	}
	if st := call("GET", "?status=archived", nil, &listed); st != 200 || len(listed.Items) != 1 {
		t.Fatal("archive filter failed", st)
	}
	if st := govCall(t, root, tenant, "", "POST", "/api/v1/governance/account-status", map[string]any{"user_id": alice.user.ID, "disabled": false}, nil); st != 409 {
		t.Fatal("legacy unfreeze bypassed archive", st)
	}
	if st := change("restore", current.Revision); st != 200 {
		t.Fatal("restore failed", st)
	}
	f.Service.DB.First(&current, "id = ?", alice.user.ID)
	if current.Archived || !current.Disabled {
		t.Fatal("restore should retain frozen state")
	}
	if st := call("POST", "/"+root.user.ID+"/action", map[string]any{"action": "freeze", "expected_revision": 1, "confirm": true}, nil); st != 403 {
		t.Fatal("self freeze accepted", st)
	}
	// Platform authority plus a browser session is not a substitute for proof.
	f.Service.DB.Model(&service.Session{}).Where("owner = ?", root.user.ID).Update("authenticated_at", time.Now().Add(-time.Hour))
	if st := change("unfreeze", current.Revision); st != 403 {
		t.Fatal("stale verification accepted", st)
	}
	if st := bearerCall(t, f, http.MethodGet, "/governance/users", secret, nil, nil); st == 200 {
		t.Fatal("archived key authenticated")
	}
}

func TestAccountFreezeProtectsRolesThroughLegacyRoutes(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	if err := f.Service.BootstrapGovernance(root.user.ID); err != nil {
		t.Fatal(err)
	}
	f.Service.DB.Model(&service.Session{}).Where("owner = ?", root.user.ID).Update("authenticated_at", time.Now().UTC())
	tenant := f.Service.Instance
	team := service.Team{ID: service.ID(), TenantID: tenant, Name: "Guarded"}
	f.Service.DB.Create(&team)
	f.Service.DB.Create(&service.TeamMember{TeamID: team.ID, UserID: alice.user.ID, Role: "admin"})
	call := func(action, user string) int {
		return govCall(t, root, tenant, "", "POST", "/api/v1/governance/"+action, map[string]any{"user_id": user, "disabled": true}, nil)
	}
	if st := call("account-status", alice.user.ID); st != 409 {
		t.Fatal("last team admin frozen", st)
	}
	f.Service.DB.Create(&service.TeamMember{TeamID: team.ID, UserID: bob.user.ID, Role: "admin"})
	space := service.Space{ID: service.ID(), Owner: service.ID(), TenantID: tenant, TeamID: team.ID, Name: "Shared"}
	f.Service.DB.Create(&space)
	f.Service.DB.Create(&service.SpacePermission{SpaceID: space.ID, UserID: alice.user.ID, Role: "custodian"})
	if st := call("account-status", alice.user.ID); st != 409 {
		t.Fatal("last custodian frozen", st)
	}
	f.Service.DB.Create(&service.SpacePermission{SpaceID: space.ID, UserID: bob.user.ID, Role: "custodian"})
	if st := call("account-status", alice.user.ID); st != 200 {
		t.Fatal("safe freeze rejected", st)
	}
	if st := call("account-status", bob.user.ID); st != 409 {
		t.Fatal("frozen user counted as successor", st)
	}
	if st := govCall(t, root, tenant, "", "POST", "/api/v1/governance/team-member", map[string]any{"team_id": team.ID, "user_id": bob.user.ID, "role": "remove"}, nil); st != 409 {
		t.Fatal("old team route bypassed active successor check", st)
	}
	var detail struct {
		KeysVisible bool `json:"keys_visible"`
		Keys        []service.UserKey
	}
	if st := govCall(t, bob, tenant, "", "GET", "/api/v1/governance/users/"+alice.user.ID, nil, &detail); st != 200 || detail.KeysVisible || len(detail.Keys) != 0 {
		t.Fatal("shared team exposed key metadata", st)
	}
	var impact struct {
		Allowed bool
		Reason  string
	}
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/governance/users/"+bob.user.ID+"/impact?action=archive", nil, &impact); st != 200 || impact.Allowed || impact.Reason == "" {
		t.Fatal("impact did not expose blocking role", st)
	}
}
