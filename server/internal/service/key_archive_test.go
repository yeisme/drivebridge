package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestArchiveKeysRevokesAndPreservesLastAdmin(t *testing.T) {
	f := rig(t)
	issued, err := f.Service.ImportUsers([]service.UserImport{{Name: "admin", Admin: true, Days: 7}, {Name: "member", Days: 7}})
	if err != nil {
		t.Fatal(err)
	}
	call := func(key, endpoint string) int {
		req, _ := http.NewRequest("POST", f.App.URL+"/api/v1"+endpoint, bytes.NewBufferString("{}"))
		req.Header.Set("Authorization", "Bearer "+key)
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	var admin, member service.UserKey
	f.Service.DB.First(&admin, "owner = ?", issued[0].UserID)
	f.Service.DB.First(&member, "owner = ?", issued[1].UserID)
	if got := call(issued[0].Key, "/users/"+admin.ID+"/archive"); got == 200 {
		t.Fatal("last administrator removed")
	}
	if got := call(issued[1].Key, "/users/"+admin.ID+"/archive"); got != 403 {
		t.Fatal("nonadmin archived user", got)
	}
	if got := call(issued[0].Key, "/users/"+member.ID+"/archive"); got != 200 {
		t.Fatal("archive", got)
	}
	f.Service.DB.First(&member, "id = ?", member.ID)
	if !member.Revoked || !member.Archived {
		t.Fatal("archive state")
	}
	if got := call(issued[1].Key, "/users/"+admin.ID+"/archive"); got != 401 {
		t.Fatal("archived credential still valid", got)
	}
	raw, _ := json.Marshal(member)
	if bytes.Contains(raw, []byte(member.TokenHash)) {
		t.Fatal("hash exposed")
	}
}
