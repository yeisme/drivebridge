package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// TestUsersManagementSDKAgainstHTTPSurface drives the SDK UsersCreate/
// UsersList/UsersRevoke methods against the real HTTP surface landed by
// drivebridge-key-session-login-v1: admin gate, {"keys":[...]} list shape,
// path-parameter revoke, plaintext keys exactly once in the issuance
// response. This is the e2e receipt for drivebridge-users-agent-surface-v1
// task 1.3.
func TestUsersManagementSDKAgainstHTTPSurface(t *testing.T) {
	f := rig(t)
	ctx := context.Background()

	// Bootstrap one admin key with operator authority; everything after
	// this line goes through the SDK over HTTP.
	admin, e := f.Service.ImportUsers([]service.UserImport{{Name: "sdk-admin", Admin: true}})
	if e != nil || len(admin) != 1 {
		t.Fatalf("admin bootstrap: %v n=%d", e, len(admin))
	}
	adminC, e := sdk.New(f.App.URL, admin[0].Key)
	if e != nil {
		t.Fatal(e)
	}

	supplied := "dbu_" + strings.Repeat("cd", 48)
	issued, e := adminC.UsersCreate(ctx, []sdk.UserImport{{Name: "sdk-made"}, {Name: "sdk-imported", Key: supplied}})
	if e != nil || len(issued) != 2 {
		t.Fatalf("sdk create: %v n=%d", e, len(issued))
	}
	if !strings.HasPrefix(issued[0].Key, "dbu_") || len(issued[0].Key) != 100 {
		t.Fatalf("generated key shape: %q", issued[0].Key)
	}
	if issued[1].Key != supplied || issued[0].Name != "sdk-made" || issued[1].Name != "sdk-imported" {
		t.Fatalf("issuance mismatch: %+v", issued)
	}
	if issued[0].UserID == "" || issued[0].Admin {
		t.Fatalf("issuance identity: %+v", issued[0])
	}

	// The imported key is a working credential that is not admin: it must
	// be forbidden (not unauthorized) on the users surface.
	importedC, e := sdk.New(f.App.URL, issued[1].Key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := importedC.UsersList(ctx); e == nil || e.Error() != "forbidden" {
		t.Fatalf("imported non-admin list: %v", e)
	}

	// List over the SDK reports lifecycle rows only: find both provisioned
	// keys by name and confirm owner/id/admin shape, never key material.
	keys, e := adminC.UsersList(ctx)
	if e != nil {
		t.Fatalf("sdk list: %v", e)
	}
	var sdkMade, adminRow *sdk.UserKey
	for i := range keys {
		switch keys[i].Name {
		case "sdk-made":
			sdkMade = &keys[i]
		case "sdk-admin":
			adminRow = &keys[i]
		}
	}
	if sdkMade == nil || adminRow == nil {
		t.Fatalf("list missing rows: %+v", keys)
	}
	if sdkMade.ID == "" || sdkMade.User == "" || sdkMade.Admin || sdkMade.Revoked {
		t.Fatalf("sdk-made row shape: %+v", sdkMade)
	}
	if !adminRow.Admin {
		t.Fatalf("admin row lost admin flag: %+v", adminRow)
	}
	if sdkMade.User == adminRow.User {
		t.Fatal("rows must belong to distinct users")
	}

	// Revoke over the SDK (path-parameter form); the revoked credential
	// must be unauthorized on its next request.
	if e := adminC.UsersRevoke(ctx, sdkMade.ID); e != nil {
		t.Fatalf("sdk revoke: %v", e)
	}
	revokedC, e := sdk.New(f.App.URL, issued[0].Key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := revokedC.UsersList(ctx); e == nil || e.Error() != "unauthorized" {
		t.Fatalf("revoked key must be unauthorized: %v", e)
	}
	rows, e := adminC.UsersList(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, k := range rows {
		if k.ID == sdkMade.ID && !k.Revoked {
			t.Fatalf("revoke not reflected in list: %+v", k)
		}
	}
}
