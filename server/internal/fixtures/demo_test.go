package fixtures

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

func TestResumeDemoKeepsExistingStorage(t *testing.T) {
	state := t.TempDir()
	f, err := StartWithOptions(state, "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true})
	if err != nil {
		t.Fatal(err)
	}
	var root service.User
	f.Service.DB.First(&root, "name = ?", "本地体验")
	var space service.Space
	f.Service.DB.First(&space, "owner = ?", root.ID)
	actor := service.Principal{UserID: root.ID, TenantID: f.Service.Instance}
	digest := sha256.Sum256([]byte("abc"))
	op, err := f.Service.CreateUpload(actor, service.UploadRequest{SpaceID: space.ID, Path: "retained.txt", Size: 3, SHA256: hex.EncodeToString(digest[:])}, service.ID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(actor, op.ID, 0, bytes.NewReader([]byte("abc"))); err != nil {
		t.Fatal(err)
	}
	op, err = f.Service.Complete(context.Background(), actor, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.App.Close()
	f.Service.Close()
	f.OIDC.Close()
	defer f.S3.Close()
	resumed, err := StartWithOptions(state, "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true, ResumeDemo: true, KeepStorage: true})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	_, _, body, err := resumed.Service.Download(context.Background(), actor, op.FileID, op.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil || string(content) != "abc" {
		t.Fatal("preserved fixture storage unavailable")
	}
}

func TestResumeDemoPreservesAccountsAndDirectories(t *testing.T) {
	state := t.TempDir()
	f, err := StartWithOptions(state, "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := f.Service.ImportUsers([]service.UserImport{{Name: "Preserved operator"}})
	if err != nil {
		t.Fatal(err)
	}
	id := f.Service.Instance
	directory := service.Directory{ID: service.ID(), Owner: issued[0].UserID, SpaceID: service.ID(), Path: "preserved"}
	f.Service.DB.Create(&directory)
	f.Close()
	resumed, err := StartWithOptions(state, "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true, ResumeDemo: true})
	if err != nil {
		t.Fatal(err)
	}
	key, err := resumed.Service.VerifyUserKey(issued[0].Key)
	if err != nil || key.Owner != issued[0].UserID || resumed.Service.Instance != id {
		t.Fatal("resume replaced an identity")
	}
	var count int64
	resumed.Service.DB.Model(&service.Directory{}).Where("id = ?", directory.ID).Count(&count)
	if count != 1 {
		t.Fatal("directory lost")
	}
	resumed.Service.DB.Model(&service.User{}).Count(&count)
	if count != 4 {
		t.Fatal("resume duplicated seed users", count)
	}
	response, err := http.Post(resumed.App.URL+"/auth/demo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("resumed demo login unavailable")
	}
	resumed.Service.DB.Create(&service.File{ID: service.ID(), Owner: issued[0].UserID, SpaceID: directory.SpaceID, Path: "do-not-reset.txt"})
	resumed.Close()
	if unexpected, err := StartWithOptions(state, "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true, ResumeDemo: true}); err == nil {
		unexpected.Close()
		t.Fatal("resume discarded a file store")
	}
}

func TestDemoSessionAndOrigin(t *testing.T) {
	f, err := StartWithOptions(t.TempDir(), "", "127.0.0.1:0", Options{Demo: true, PublicOrigin: "http://localhost:10414"})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	for _, origin := range []string{"", "null", "http://localhost:9999", "https://preview.example.invalid", f.App.URL} {
		req, _ := http.NewRequest("POST", f.App.URL+"/auth/demo", nil)
		req.Header.Set("Origin", origin)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("demo forwarding origin rejected: %d", res.StatusCode)
		}
	}
	req, _ := http.NewRequest("POST", f.App.URL+"/auth/demo", nil)
	req.Header.Set("Origin", "https://preview.example.invalid")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("demo login: %d", res.StatusCode)
	}
	res, err = client.Get(f.App.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	var me struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	if err = json.NewDecoder(res.Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || me.Data.CSRF == "" {
		t.Fatal("missing authenticated session")
	}
	req, _ = http.NewRequest("POST", f.App.URL+"/api/v1/directories", strings.NewReader(`{"space_id":"none","path":"test"}`))
	req.Header.Set("Origin", "https://preview.example.invalid")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		}
	}
	json.NewDecoder(res.Body).Decode(&envelope)
	res.Body.Close()
	if envelope.Error.Code != "csrf_rejected" {
		t.Fatal("demo bypassed mutation CSRF")
	}
	req, _ = http.NewRequest("POST", f.App.URL+"/auth/logout", nil)
	req.Header.Set("Origin", "https://preview.example.invalid")
	req.Header.Set("X-CSRF-Token", me.Data.CSRF)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		t.Fatalf("forwarded demo write failed: %d", res.StatusCode)
	}
	var key service.UserKey
	if err := f.Service.DB.Where("name = ?", "本地体验").First(&key).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.RotateUserKey(key.ID, 0); err != nil {
		t.Fatal(err)
	}
	res, err = client.Post(f.App.URL+"/auth/demo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("demo login did not follow rotated key", res.StatusCode)
	}

}

func TestDemoDisabledByDefault(t *testing.T) {
	f, err := Start(t.TempDir(), "", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := http.Post(f.App.URL+"/auth/demo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("demo enabled without opt-in")
	}
}

func TestDemoRejectsNonLocalConfig(t *testing.T) {
	for _, options := range []Options{{Demo: true}, {Demo: true, PublicOrigin: "https://example.com"}} {
		if f, err := StartWithOptions(t.TempDir(), "", "0.0.0.0:0", options); err == nil {
			f.Close()
			t.Fatal("non-loopback listener accepted")
		}
	}
	if f, err := StartWithOptions(t.TempDir(), "", "127.0.0.1:0", Options{PublicOrigin: "http://example.com"}); err == nil {
		f.Close()
		t.Fatal("non-loopback origin accepted")
	}
}

func TestTeamDemoUsesRealRoles(t *testing.T) {
	f, err := StartWithOptions(t.TempDir(), "", "127.0.0.1:0", Options{Demo: true, TeamDemo: true})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	var oldCookie *http.Cookie
	for _, persona := range []string{"admin", "custodian", "member"} {
		req, _ := http.NewRequest("POST", f.App.URL+"/auth/demo", strings.NewReader(`{"persona":"`+persona+`"}`))
		req.Header.Set("Origin", "https://local-preview.invalid")
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatal("persona exchange", res.StatusCode)
		}
		if persona == "admin" {
			for _, c := range res.Cookies() {
				if c.Name == "db_session" {
					oldCookie = c
				}
			}
		}
		res, e = client.Get(f.App.URL + "/api/v1/me")
		if e != nil {
			t.Fatal(e)
		}
		var me struct {
			Data struct {
				CSRF    string `json:"csrf_token"`
				Tenant  string `json:"tenant_id"`
				Enabled bool   `json:"governance_enabled"`
			}
		}
		json.NewDecoder(res.Body).Decode(&me)
		res.Body.Close()
		if !me.Data.Enabled {
			t.Fatal("demo simulated governance only in UI")
		}
		var sp service.Space
		f.Service.DB.First(&sp, "team_id <> ?", "")
		req, _ = http.NewRequest("POST", f.App.URL+"/api/v1/directories", strings.NewReader(`{"space_id":"`+sp.ID+`","path":"`+persona+`"}`))
		req.Header.Set("X-CSRF-Token", me.Data.CSRF)
		req.Header.Set("X-DriveBridge-Tenant", me.Data.Tenant)
		req.Header.Set("X-DriveBridge-Space", sp.ID)
		res, e = client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		want := 403
		if persona == "custodian" {
			want = 200
		}
		if res.StatusCode != want {
			t.Fatalf("persona %s: status %d", persona, res.StatusCode)
		}
	}
	if oldCookie == nil {
		t.Fatal("missing original session cookie")
	}
	req, _ := http.NewRequest("GET", f.App.URL+"/api/v1/me", nil)
	req.AddCookie(oldCookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("persona switch retained previous browser session")
	}
}
