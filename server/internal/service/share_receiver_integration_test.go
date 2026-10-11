package service_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestFPShareReceiverMediaQuotaLifecycle(t *testing.T) {
	f := rig(t)
	f.Service.Config.ShareReceivers = true
	h, _ := login(t, f, "alice")
	_, client := agent(t, h, "", "read,write")
	first, err := client.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "receiver/first.txt"}, "receiver-first", strings.NewReader("original content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "receiver/second.txt"}, "receiver-second", strings.NewReader("second content"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Share service.Share `json:"share"`
		Link  string        `json:"link"`
	}
	body := map[string]any{"name": "Single receiver", "space_id": h.space.ID, "mode": "fixed", "audience": "link", "allow_download": true, "single_use": true}
	if status := h.call(t, "POST", "/shares", body, &created); status != 200 {
		t.Fatal("create", status)
	}
	if created.Share.MaxOpens == nil || *created.Share.MaxOpens != 1 || created.Share.MaxDownloads == nil || *created.Share.MaxDownloads != 1 {
		t.Fatal("single-use not derived")
	}
	var persisted service.Share
	if err = f.Service.DB.First(&persisted, "id = ?", created.Share.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !persisted.SessionRequired {
		t.Fatal("new share grandfathered raw-grant cookies")
	}
	jar, _ := cookiejar.New(nil)
	guest := &human{base: h.base, http: &http.Client{Jar: jar}}
	parts := strings.Split(created.Link, "/")
	grant := parts[len(parts)-1]
	endpoint := "/shared/" + created.Share.ID
	if status := guest.call(t, "POST", endpoint+"/open", map[string]string{"grant": grant}, nil); status != 200 {
		t.Fatal("open", status)
	}
	get := func(file, byteRange, download string) (int, string) {
		t.Helper()
		r, _ := http.NewRequest("GET", h.base+"/api/v1"+endpoint+"/files/"+file+"/content"+download, nil)
		r.Header.Set("Range", byteRange)
		response, err := guest.http.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		content, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(content)
	}
	if status, _ := get(first.FileID, "bytes=999-", "?download=true"); status != 416 {
		t.Fatal("invalid range", status)
	}
	var share service.Share
	f.Service.DB.First(&share, "id = ?", created.Share.ID)
	if share.DownloadsUsed != 0 {
		t.Fatal("bad range charged")
	}
	if status, content := get(first.FileID, "bytes=0-3", "?download=true"); status != 206 || content != "orig" {
		t.Fatal("initial range", status, content)
	}
	if status, content := get(first.FileID, "bytes=4-", "?download=true"); status != 206 || content != "inal content" {
		t.Fatal("resume", status, content)
	}
	if status, _ := get(second.FileID, "", "?download=true"); status != 400 {
		t.Fatal("quota status", status)
	}
	if status, _ := get(second.FileID, "", ""); status != 400 {
		t.Fatal("inline quota bypass", status)
	}
	if status := guest.call(t, "GET", endpoint, nil, nil); status != 200 {
		t.Fatal("exhaustion blocked listing", status)
	}
	if status := guest.call(t, "POST", endpoint+"/open", map[string]string{"grant": grant}, nil); status == 200 {
		t.Fatal("single-use reopened")
	}
	f.Service.DB.First(&share, "id = ?", created.Share.ID)
	if share.OpensUsed != 1 || share.DownloadsUsed != 1 || share.Revoked {
		t.Fatal("quota count drift")
	}
	var receivers service.ShareReceivers
	if status := h.call(t, "GET", "/shares/"+share.ID+"/receivers", nil, &receivers); status != 200 || receivers.DownloadsUsed != 1 {
		t.Fatal("owner quota", status)
	}
}
