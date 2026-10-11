package service_test

import (
	"bytes"
	"context"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"net/http"
	"strings"
	"testing"
)

func TestMeProfileUpdate(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	var out struct {
		User service.User `json:"user"`
	}
	if status := alice.call(t, "PATCH", "/me", map[string]any{"display_name": "资料用户"}, &out); status != 200 {
		t.Fatal(status)
	}
	if out.User.Name != alice.user.Name || out.User.ID != alice.user.ID || out.User.DisplayName != "资料用户" {
		t.Fatal("identity compatibility changed")
	}
	for _, payload := range []map[string]any{{"display_name": ""}, {"display_name": "  "}, {"display_name": strings.Repeat("名", 81)}, {"display_name": "a\n" + "b"}, {"avatar_ref": "https://example.com/x"}, {"avatar_ref": "asset:brand/avatar"}, {"avatar_ref": "file:missing:missing"}, {}} {
		if status := alice.call(t, "PATCH", "/me", payload, nil); status != 400 {
			t.Fatalf("invalid profile status %d", status)
		}
	}
	csrf := alice.csrf
	alice.csrf = "wrong"
	if status := alice.call(t, "PATCH", "/me", map[string]string{"display_name": "blocked"}, nil); status != 403 {
		t.Fatal(status)
	}
	alice.csrf = csrf
	_, client := agent(t, alice, "", "read,write")
	if err := client.JSON(context.Background(), "PATCH", "/api/v1/me", "", map[string]string{"display_name": "agent-change"}, nil); err == nil {
		t.Fatal("agent changed human profile")
	}
	// A valid fixture image becomes a controlled immutable avatar reference.
	payload := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 31, 21, 196, 137}
	op, err := client.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "avatar.png"}, "profile-avatar", bytes.NewReader(payload), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := "file:" + op.FileID + ":" + op.VersionID
	if status := alice.call(t, "PATCH", "/me", map[string]string{"avatar_ref": ref}, &out); status != 200 {
		t.Fatal(status)
	}
	if out.User.AvatarRef != ref {
		t.Fatal("avatar not saved")
	}
	if status := bob.call(t, "PATCH", "/me", map[string]string{"avatar_ref": ref}, nil); status != 400 {
		t.Fatal("foreign avatar accepted", status)
	}
	var candidates struct {
		Items []map[string]string `json:"items"`
	}
	alice.call(t, "GET", "/me/avatar-options", nil, &candidates)
	if len(candidates.Items) != 1 || candidates.Items[0]["value"] != ref {
		t.Fatal("avatar options missing")
	}
	response, err := alice.http.Get(alice.base + "/api/v1/me/avatar")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal("avatar content failed", response.StatusCode)
	}
	f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed", true)
	response, err = alice.http.Get(alice.base + "/api/v1/me/avatar")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("trashed avatar still served")
	}
	f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Update("trashed", false)
	second, _ := login(t, f, "alice")
	second.call(t, "GET", "/me", nil, &out)
	if out.User.DisplayName != "资料用户" || out.User.AvatarRef != ref {
		t.Fatal("profile did not survive login")
	}
	out.User = service.User{}
	if status := alice.call(t, "PATCH", "/me", map[string]any{"avatar_ref": nil}, &out); status != 200 || out.User.AvatarRef != "" {
		t.Fatal("avatar clear failed")
	}
}
