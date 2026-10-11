package service_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestBrowserMediaShareLifecycle(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, c := agent(t, h, "", "read,write,delete")
	first, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "project/first.txt"}, "media-first", strings.NewReader("original content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "empty"}, nil); got != 200 {
		t.Fatal(got)
	}
	var listing struct {
		Entries []service.BrowserEntry `json:"entries"`
	}
	if got := h.call(t, "GET", "/browser?space_id="+h.space.ID, nil, &listing); got != 200 || len(listing.Entries) != 2 {
		t.Fatalf("directory listing %d %+v", got, listing)
	}
	if got := bob.call(t, "GET", "/browser?space_id="+h.space.ID, nil, nil); got != 404 {
		t.Fatal("foreign directory", got)
	}
	if got := h.call(t, "POST", "/files/"+first.FileID+"/move", map[string]string{"path": "project/moved.txt", "expected_version": first.VersionID}, nil); got != 200 {
		t.Fatal(got)
	}
	var share struct {
		Share service.Share `json:"share"`
		Link  string        `json:"link"`
	}
	request := map[string]any{"name": "Delivery", "space_id": h.space.ID, "path": "project", "mode": "fixed", "audience": "link", "days": 7, "passcode": "example-code", "allow_download": false, "items": []map[string]string{{"file_id": first.FileID, "version_id": first.VersionID}}}
	if got := h.call(t, "POST", "/shares", request, &share); got != 200 {
		t.Fatal("share", got)
	}
	jar, _ := cookiejar.New(nil)
	guest := &human{base: h.base, http: &http.Client{Jar: jar}}
	secret := strings.Split(share.Link, "/")
	grant := secret[len(secret)-1]
	endpoint := "/shared/" + share.Share.ID
	if got := guest.call(t, "POST", endpoint+"/open", map[string]string{"grant": grant, "passcode": "wrong"}, nil); got == 200 {
		t.Fatal("incorrect passcode admitted")
	}
	if got := guest.call(t, "POST", endpoint+"/open", map[string]string{"grant": grant, "passcode": "example-code"}, nil); got != 200 {
		t.Fatal("open", got)
	}
	second, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "project/moved.txt", FileID: first.FileID, ExpectedVersion: first.VersionID}, "media-next", strings.NewReader("replacement content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var shared struct {
		Files []service.File `json:"files"`
	}
	if got := guest.call(t, "GET", endpoint, nil, &shared); got != 200 || len(shared.Files) != 1 || shared.Files[0].VersionID != first.VersionID {
		t.Fatal("share drifted", got)
	}
	get := func(client *http.Client, url, rangeHeader string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", h.base+"/api/v1"+url, nil)
		req.Header.Set("Range", rangeHeader)
		r, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(body)
	}
	if status, body := get(h.http, "/files/"+first.FileID+"/media?version="+first.VersionID, "bytes=2-5"); status != 206 || body != "igin" {
		t.Fatalf("range %d %q", status, body)
	}
	if status, body := get(h.http, "/files/"+first.FileID+"/media?version="+second.VersionID, "bytes=-7"); status != 206 || body != "content" {
		t.Fatalf("suffix %d %q", status, body)
	}
	if status, _ := get(h.http, "/files/"+first.FileID+"/media?version="+first.VersionID, "bytes=999-"); status != 416 {
		t.Fatal("invalid range", status)
	}
	if status, body := get(guest.http, endpoint+"/files/"+first.FileID+"/content", ""); status != 200 || body != "original content" {
		t.Fatalf("fixed content %d %q", status, body)
	}
	if status, _ := get(guest.http, endpoint+"/files/"+first.FileID+"/content?download=true", ""); status != 403 {
		t.Fatal("download permission", status)
	}
	var use struct {
		Allowed bool `json:"permanent_delete_allowed"`
	}
	h.call(t, "GET", "/files/"+first.FileID+"/usage", nil, &use)
	if use.Allowed {
		t.Fatal("shared file purge allowed")
	}
	h.call(t, "POST", "/files/"+first.FileID+"/trash", map[string]any{}, nil)
	if err = f.Service.Purge(context.Background(), service.Principal{UserID: h.user.ID}, first.FileID); err == nil || err.Error() != "file_in_use" {
		t.Fatal("purge bypass", err)
	}
	if got := h.call(t, "POST", "/shares/"+share.Share.ID+"/revoke", map[string]any{}, nil); got != 200 {
		t.Fatal(got)
	}
	if got := guest.call(t, "GET", endpoint, nil, nil); got == 200 {
		t.Fatal("revoked share accessible")
	}
}

func TestPermanentShareAndPresentation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	file, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "permanent.txt"}, "permanent", strings.NewReader("permanent content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Share service.Share `json:"share"`
		Link  string        `json:"link"`
	}
	request := map[string]any{"name": "Permanent", "space_id": h.space.ID, "mode": "fixed", "audience": "link", "days": 0, "presentation": map[string]any{"preset": "dark", "font": "serif", "accent_color": "#123456", "welcome_message": "欢迎查看"}, "items": []map[string]string{{"file_id": file.FileID, "version_id": file.VersionID}}}
	if got := h.call(t, "POST", "/shares", request, &out); got != 200 {
		t.Fatalf("create permanent share %d", got)
	}
	if out.Share.Expires.IsZero() == false {
		t.Fatal("permanent share has expiry")
	}
	if got := h.call(t, "GET", "/shares", nil, &[]service.Share{}); got != 200 {
		t.Fatal("list permanent shares", got)
	}
	secret := strings.Split(out.Link, "/")
	guest := &human{base: h.base, http: h.http}
	if got := guest.call(t, "POST", "/shared/"+out.Share.ID+"/open", map[string]string{"grant": secret[len(secret)-1]}, nil); got != 200 {
		t.Fatalf("open permanent share %d", got)
	}
	var shared struct {
		IsPermanent  bool                      `json:"is_permanent"`
		Presentation service.SharePresentation `json:"presentation"`
	}
	if got := guest.call(t, "GET", "/shared/"+out.Share.ID, nil, &shared); got != 200 || !shared.IsPermanent || shared.Presentation.Preset != "dark" {
		t.Fatalf("permanent detail %d %+v", got, shared)
	}
}

func TestShareLiveQuarantineAndUserIsolation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, c := agent(t, h, "", "read,write")
	upload := func(name string) service.Operation {
		t.Helper()
		o, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: name}, name, strings.NewReader("body"), nil)
		if e != nil {
			t.Fatal(e)
		}
		var op service.Operation
		f.Service.DB.First(&op, "id = ?", o.ID)
		return op
	}
	upload("live/a.txt")
	var out struct {
		Share service.Share `json:"share"`
		Link  string        `json:"link"`
	}
	q := map[string]any{"name": "Live", "space_id": h.space.ID, "path": "live", "mode": "live", "audience": "users", "recipients": []string{bob.user.ID}, "days": 1, "allow_download": true}
	if got := h.call(t, "POST", "/shares", q, &out); got != 200 {
		t.Fatal(got)
	}
	p := strings.Split(out.Link, "/")
	grant := p[len(p)-1]
	endpoint := "/shared/" + out.Share.ID
	if got := bob.call(t, "POST", endpoint+"/open", map[string]string{"grant": grant}, nil); got != 200 {
		t.Fatal(got)
	}
	upload("live/b.txt")
	pending := upload("live/pending.txt")
	f.Service.DB.Model(&service.File{}).Where("id = ?", pending.FileID).Update("review_pending", true)
	var list struct {
		Files []service.File `json:"files"`
	}
	if got := bob.call(t, "GET", endpoint, nil, &list); got != 200 || len(list.Files) != 2 {
		t.Fatal("live/quarantine", got, len(list.Files))
	}
	f.Service.DB.Model(&service.Share{}).Where("id = ?", out.Share.ID).Update("expires", time.Now().Add(-time.Minute))
	if got := bob.call(t, "GET", endpoint, nil, nil); got == 200 {
		t.Fatal("expired share admitted")
	}
}
