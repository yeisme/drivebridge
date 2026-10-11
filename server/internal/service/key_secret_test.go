package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestKeySecretOwnershipRepeatAndRotation(t *testing.T) {
	f := rig(t)
	issued, err := f.Service.ImportUsers([]service.UserImport{{Name: "Owner"}, {Name: "Admin", Admin: true}})
	if err != nil {
		t.Fatal(err)
	}
	key, err := f.Service.VerifyUserKey(issued[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(key.SealedKey) == 0 || bytes.Contains(key.SealedKey, []byte(issued[0].Key)) {
		t.Fatal("key must be encrypted at rest")
	}
	encoded, _ := json.Marshal(key)
	if bytes.Contains(encoded, []byte(issued[0].Key)) || bytes.Contains(encoded, []byte(key.TokenHash)) || bytes.Contains(encoded, []byte("SealedKey")) {
		t.Fatal("credential material exposed by metadata")
	}
	base := "/key-directory/" + key.ID
	for i := 0; i < 2; i++ {
		var revealed struct{ Key string }
		if status := bearerCall(t, f, "POST", base+"/reveal", issued[0].Key, nil, &revealed); status != 200 || revealed.Key != issued[0].Key {
			t.Fatal("repeat reveal failed", status)
		}
	}
	for _, action := range []string{"reveal", "rotate"} {
		if status := bearerCall(t, f, "POST", base+"/"+action, issued[1].Key, map[string]bool{"confirm": true}, nil); status != 404 {
			t.Fatal("admin crossed credential ownership", status)
		}
	}
	// Identical authority rules apply after enabling tenant governance.
	if err := f.Service.BootstrapGovernance(issued[1].UserID); err != nil {
		t.Fatal(err)
	}
	if status := bearerCall(t, f, "POST", base+"/reveal", issued[1].Key, nil, nil); status == 200 {
		t.Fatal("tenant admin revealed another key")
	}
	if status := bearerCall(t, f, "POST", base+"/rotate", issued[0].Key, map[string]bool{"confirm": false}, nil); status == 200 {
		t.Fatal("unconfirmed rotation accepted")
	}
	var rotated struct{ ID, Key string }
	if status := bearerCall(t, f, "POST", base+"/rotate", issued[0].Key, map[string]bool{"confirm": true}, &rotated); status != 200 || rotated.ID == key.ID || rotated.Key == issued[0].Key || rotated.Key == "" {
		t.Fatal("rotation failed", status)
	}
	if status := bearerCall(t, f, "GET", "/me", issued[0].Key, nil, nil); status != 401 {
		t.Fatal("old key remained active", status)
	}
	if status := bearerCall(t, f, "POST", base+"/reveal", rotated.Key, nil, nil); status == 200 {
		t.Fatal("revoked material returned")
	}
	var revealed struct{ Key string }
	if status := bearerCall(t, f, "POST", "/key-directory/"+rotated.ID+"/reveal", rotated.Key, nil, &revealed); status != 200 || revealed.Key != rotated.Key {
		t.Fatal("new key not repeatable", status)
	}
	newKey, err := f.Service.VerifyUserKey(rotated.Key)
	if err != nil || newKey.Owner != key.Owner || newKey.Admin != key.Admin || newKey.Expires != key.Expires {
		t.Fatal("rotation changed identity or lifetime")
	}
}

func TestKeySecretLegacySessionAndConcurrentRotation(t *testing.T) {
	f := rig(t)
	issued, err := f.Service.ImportUsers([]service.UserImport{{Name: "Legacy"}})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := f.Service.VerifyUserKey(issued[0].Key)
	f.Service.DB.Model(&key).Update("sealed_key", nil)
	if status := bearerCall(t, f, "POST", "/key-directory/"+key.ID+"/reveal", issued[0].Key, nil, nil); status == 200 {
		t.Fatal("hash-only key invented plaintext")
	}
	login := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		body, _ := json.Marshal(map[string]string{"key": issued[0].Key})
		r, _ := http.NewRequest("POST", f.App.URL+"/auth/key", bytes.NewReader(body))
		r.Header.Set("Origin", f.App.URL)
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatal("login failed", res.StatusCode)
		}
		return client
	}
	current, other := login(), login()
	var copy struct{ Key string }
	if status := bearerCall(t, f, "POST", "/key-directory/"+key.ID+"/reveal", issued[0].Key, nil, &copy); status != 200 || copy.Key != issued[0].Key {
		t.Fatal("verified legacy login did not retain key")
	}
	res, err := current.Get(f.App.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	var me struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	json.NewDecoder(res.Body).Decode(&me)
	res.Body.Close()
	r, _ := http.NewRequest("POST", f.App.URL+"/api/v1/key-directory/"+key.ID+"/rotate", strings.NewReader(`{"confirm":true}`))
	r.Header.Set("Origin", f.App.URL)
	r.Header.Set("X-CSRF-Token", me.Data.CSRF)
	res, err = current.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	var rotated struct{ Data struct{ ID, Key string } }
	json.NewDecoder(res.Body).Decode(&rotated)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("session rotation failed", res.StatusCode)
	}
	for i, client := range []*http.Client{current, other} {
		res, err := client.Get(f.App.URL + "/api/v1/me")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 200
		if i == 1 {
			want = 401
		}
		if res.StatusCode != want {
			t.Fatal("rotation session isolation failed", i, res.StatusCode)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.Service.RotateUserKey(rotated.Data.ID, 0); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("concurrent rotation minted multiple successors", success)
	}
}

func TestKeySecretInactiveAndTampered(t *testing.T) {
	f := rig(t)
	issued, _ := f.Service.ImportUsers([]service.UserImport{{Name: "Owner"}})
	key, _ := f.Service.VerifyUserKey(issued[0].Key)
	// An independent OIDC session can inspect metadata but may never retrieve
	// inactive material. A second active key represents that same owner here.
	second, _ := f.Service.ImportUsers([]service.UserImport{{Name: "Second"}})
	other, _ := f.Service.VerifyUserKey(second[0].Key)
	f.Service.DB.Model(&other).Update("owner", key.Owner)
	for _, updates := range []map[string]any{{"revoked": true}, {"revoked": false, "archived": true}, {"archived": false, "expires_at": time.Now().Add(-time.Hour)}, {"expires_at": time.Time{}, "sealed_key": []byte("invalid")}} {
		f.Service.DB.Model(&key).Updates(updates)
		if status := bearerCall(t, f, "POST", "/key-directory/"+key.ID+"/reveal", second[0].Key, nil, nil); status == 200 {
			t.Fatal("inactive or tampered secret returned")
		}
	}
	agentToken := service.ID()
	f.Service.DB.Create(&service.Agent{ID: service.ID(), Owner: key.Owner, TokenHash: service.Hash(agentToken), Expires: time.Now().Add(time.Hour), Scopes: "read"})
	if status := bearerCall(t, f, "POST", "/key-directory/"+key.ID+"/reveal", agentToken, nil, nil); status != 403 {
		t.Fatal("agent secret access", status)
	}
}
