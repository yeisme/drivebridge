package service_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestKeyDisableEnableAndLogin(t *testing.T) {
	f := rig(t)
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "Owner"}})
	if err != nil {
		t.Fatal(err)
	}
	var primary service.UserKey
	f.Service.DB.First(&primary, "owner = ?", users[0].UserID)
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
	status, raw := call(users[0].Key, "POST", "/key-directory/"+primary.ID+"/disable", map[string]any{"confirm": true})
	if status == 200 {
		t.Fatal("last live key disabled")
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Error.Code != "last_active_key" {
		t.Fatalf("want last_active_key, got %s %s", envelope.Error.Code, raw)
	}
	status, raw = call(users[0].Key, "POST", "/key-directory", map[string]any{"name": "laptop", "days": 90})
	if status != 200 {
		t.Fatal("create spare", status, string(raw))
	}
	var created struct {
		Data struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &created) != nil || created.Data.ID == "" || created.Data.Key == "" {
		t.Fatal("spare envelope", string(raw))
	}
	status, raw = call(users[0].Key, "POST", "/key-directory/"+created.Data.ID+"/disable", map[string]any{"confirm": true})
	if status != 200 {
		t.Fatal("disable spare", status, string(raw))
	}
	if status, _ = call(created.Data.Key, "GET", "/me", nil); status == 200 {
		t.Fatal("disabled key still authenticates")
	}
	status, raw = call(users[0].Key, "GET", "/key-directory", nil)
	var listed struct {
		Data struct {
			Keys []struct {
				ID         string `json:"id"`
				Disabled   bool   `json:"disabled"`
				CanEnable  bool   `json:"can_enable"`
				CanDisable bool   `json:"can_disable"`
				CanCopy    bool   `json:"can_copy"`
			} `json:"keys"`
		} `json:"data"`
	}
	if status != 200 || json.Unmarshal(raw, &listed) != nil {
		t.Fatal("list", string(raw))
	}
	found := false
	for _, row := range listed.Data.Keys {
		if row.ID == created.Data.ID {
			found = true
			if !row.Disabled || !row.CanEnable || row.CanDisable || row.CanCopy {
				t.Fatalf("disabled flags %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("disabled key missing from directory")
	}
	if status, _ = call(users[0].Key, "POST", "/key-directory/"+created.Data.ID+"/enable", nil); status != 200 {
		t.Fatal("enable spare", status)
	}
	if status, _ = call(created.Data.Key, "GET", "/me", nil); status != 200 {
		t.Fatal("enabled key cannot authenticate")
	}
	if status, _ = call(users[0].Key, "POST", "/key-directory/"+created.Data.ID+"/reveal", nil); status != 200 {
		t.Fatal("enabled key not revealable")
	}
}
