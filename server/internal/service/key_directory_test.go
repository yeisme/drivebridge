package service_test

import (
	"bytes"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestKeyDirectoryIdentityAndLabels(t *testing.T) {
	f := rig(t)
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "Directory admin", Admin: true}, {Name: "Directory member"}})
	if err != nil {
		t.Fatal(err)
	}
	var adminKey, memberKey service.UserKey
	f.Service.DB.First(&adminKey, "owner = ?", users[0].UserID)
	f.Service.DB.First(&memberKey, "owner = ?", users[1].UserID)
	call := func(token, method, path string, body any) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, f.App.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		return res.StatusCode, out
	}
	status, raw := call(users[1].Key, "GET", "/key-directory", nil)
	var out struct {
		Data struct {
			Keys []struct {
				ID      string `json:"id"`
				User    string `json:"user"`
				Current bool   `json:"current"`
			}
		}
	}
	if status != 200 || json.Unmarshal(raw, &out) != nil || len(out.Data.Keys) != 1 || out.Data.Keys[0].User != users[1].UserID || !out.Data.Keys[0].Current {
		t.Fatal("directory did not retain exact actor ownership")
	}
	for _, secret := range []string{users[0].Key, users[1].Key, memberKey.TokenHash, adminKey.TokenHash} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("credential material exposed")
		}
	}
	if status, _ = call(users[1].Key, "POST", "/key-directory/"+adminKey.ID+"/label", map[string]string{"name": "stolen"}); status == 200 {
		t.Fatal("member renamed another users key")
	}
	if status, _ = call(users[1].Key, "POST", "/key-directory/"+memberKey.ID+"/label", map[string]string{"name": "素材整理电脑"}); status != 200 {
		t.Fatal("own label failed", status)
	}
	var updated service.UserKey
	f.Service.DB.First(&updated, "id = ?", memberKey.ID)
	if updated.Owner != memberKey.Owner || updated.TokenHash != memberKey.TokenHash || updated.Name != "素材整理电脑" {
		t.Fatal("label changed credential identity")
	}
	if status, _ = call(users[1].Key, "POST", "/key-directory/"+memberKey.ID+"/label", map[string]string{"name": "  "}); status == 200 {
		t.Fatal("empty label accepted")
	}
	if status, raw = call(users[0].Key, "GET", "/key-directory", nil); status != 200 || !bytes.Contains(raw, []byte(memberKey.ID)) {
		t.Fatal("legacy admin scope lost")
	}
	agentToken := service.ID()
	agent := service.Agent{ID: service.ID(), Owner: users[1].UserID, Name: "Directory agent", TokenHash: service.Hash(agentToken), Expires: time.Now().Add(time.Hour), Scopes: "read"}
	f.Service.DB.Create(&agent)
	if status, _ = call(agentToken, "GET", "/key-directory", nil); status != 403 {
		t.Fatal("agent received login key directory", status)
	}
	if err = f.Service.BootstrapGovernance(users[0].UserID); err != nil {
		t.Fatal(err)
	}
	if status, raw = call(users[0].Key, "GET", "/key-directory", nil); status != 200 || bytes.Contains(raw, []byte(memberKey.ID)) {
		t.Fatal("governance admin inherited global credential scope")
	}
	if status, _ = call(users[0].Key, "POST", "/key-directory/"+memberKey.ID+"/label", map[string]string{"name": "tenant scope escape"}); status == 200 {
		t.Fatal("governance admin modified other credential")
	}
}
