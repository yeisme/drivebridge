package service_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestKeySelfDeleteRequiresSpareKey(t *testing.T) {
	f := rig(t)
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "Solo"}})
	if err != nil {
		t.Fatal(err)
	}
	var key service.UserKey
	f.Service.DB.First(&key, "owner = ?", users[0].UserID)
	call := func(method, path string, body any) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, f.App.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+users[0].Key)
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		return res.StatusCode, out
	}
	status, raw := call("POST", "/key-directory/"+key.ID+"/delete", map[string]any{"confirm": true})
	if status == 200 {
		t.Fatal("last key deleted")
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Error.Code != "last_active_key" {
		t.Fatalf("want last_active_key, got %s %s", envelope.Error.Code, raw)
	}
	status, _ = call("POST", "/key-directory", map[string]any{"name": "laptop", "days": 90})
	if status != 200 {
		t.Fatal("create spare key", status)
	}
	status, raw = call("GET", "/key-directory", nil)
	var listed struct {
		Data struct {
			Keys []struct {
				ID        string `json:"id"`
				CanRevoke bool   `json:"can_revoke"`
				CanDelete bool   `json:"can_delete"`
				Name      string `json:"name"`
			} `json:"keys"`
		} `json:"data"`
	}
	if status != 200 || json.Unmarshal(raw, &listed) != nil || len(listed.Data.Keys) != 2 {
		t.Fatalf("list: %s", raw)
	}
	spare := ""
	for _, row := range listed.Data.Keys {
		if row.Name == "laptop" {
			if !row.CanDelete || !row.CanRevoke {
				t.Fatal("spare key not deletable")
			}
			spare = row.ID
		}
	}
	if spare == "" {
		t.Fatal("spare missing")
	}
	if status, _ = call("POST", "/key-directory/"+spare+"/delete", map[string]any{}); status == 200 {
		t.Fatal("delete without confirm")
	}
	if status, _ = call("POST", "/key-directory/"+spare+"/delete", map[string]any{"confirm": true}); status != 200 {
		t.Fatal("delete spare", status)
	}
	if status, _ = call("POST", "/key-directory/"+spare+"/reveal", nil); status == 200 {
		t.Fatal("deleted key still revealable")
	}
}
