package service_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestMediaThumbnailFixedVersionAndIsolation(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, c := agent(t, h, "", "read,write")
	picture := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			picture.Set(x, y, color.RGBA{80, 160, 220, 255})
		}
	}
	var data bytes.Buffer
	png.Encode(&data, picture)
	op, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "preview.png"}, "thumb", bytes.NewReader(data.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	var preview service.Preview
	if status := h.call(t, "POST", "/files/"+op.FileID+"/previews", map[string]string{"version_id": op.VersionID, "kind": "thumbnail"}, &preview); status != 200 {
		t.Fatal(status)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var rows []service.Preview
		h.call(t, "GET", "/files/"+op.FileID+"/previews?version="+op.VersionID, nil, &rows)
		if len(rows) > 0 {
			preview = rows[0]
			if preview.Status != "processing" {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if preview.Status != "ready" {
		t.Fatalf("thumbnail not ready: %s %s", preview.Status, preview.Error)
	}
	resp, err := h.http.Get(h.base + "/api/v1/previews/" + preview.ID + "/content")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	decoded, err := jpeg.Decode(resp.Body)
	if err != nil || decoded.Bounds().Dx() != 64 {
		t.Fatal("invalid thumbnail", err)
	}
	foreign, err := bob.http.Get(h.base + "/api/v1/previews/" + preview.ID + "/content")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, foreign.Body)
	foreign.Body.Close()
	if foreign.StatusCode != 404 {
		t.Fatal("foreign thumbnail exposed", foreign.StatusCode)
	}
	var same service.Preview
	h.call(t, "POST", "/files/"+op.FileID+"/previews", map[string]string{"version_id": op.VersionID, "kind": "thumbnail"}, &same)
	if same.ID != preview.ID {
		t.Fatal("cache identity changed")
	}
}
