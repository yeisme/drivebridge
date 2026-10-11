package service_test

import (
	"github.com/yeisme/drivebridge-server/internal/service"
	"testing"
)

func TestCreateOwnKeyPreservesIdentityAndOldKey(t *testing.T) {
	f := rig(t)
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "self"}})
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID, Key string
		UserID  string `json:"user_id"`
	}
	status := bearerCall(t, f, "POST", "/key-directory", users[0].Key, map[string]any{"name": "Second device", "days": 30}, &created)
	if status != 200 || created.UserID != users[0].UserID || created.Key == "" {
		t.Fatal("self key creation failed", status)
	}
	fresh, err := f.Service.VerifyUserKey(created.Key)
	if err != nil || fresh.Owner != users[0].UserID || fresh.Admin || len(fresh.SealedKey) == 0 {
		t.Fatal("invalid issued key")
	}
	if _, err = f.Service.VerifyUserKey(users[0].Key); err != nil {
		t.Fatal("original key revoked")
	}
	if bearerCall(t, f, "POST", "/key-directory", users[0].Key, map[string]any{"name": "bad", "days": -1}, nil) != 400 {
		t.Fatal("invalid expiry accepted")
	}
}
