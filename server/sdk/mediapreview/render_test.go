package mediapreview_test

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk/mediapreview"
)

func TestConverterSelectionForExtendedFormats(t *testing.T) {
	if got := mediapreview.Converter("Forza Horizon 5 2025_10_8 0_55_36.jxr"); got == "" {
		t.Fatal("expected converter for .jxr")
	}
	if got := mediapreview.Converter("photo.heic"); got == "" {
		t.Fatal("expected converter for .heic")
	}
	if got := mediapreview.Converter("camera.pef"); got != "dcraw" && got != "dcraw_emu" {
		t.Fatalf("unexpected raw converter: %s", got)
	}
}

func TestJxrPreviewConversionIfToolsAvailable(t *testing.T) {
	if _, err := exec.LookPath("JxrDecApp"); err != nil {
		t.Skip("JxrDecApp unavailable")
	}
	if _, err := exec.LookPath("JxrEncApp"); err != nil {
		t.Skip("JxrEncApp unavailable")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap unavailable")
	}

	dir := t.TempDir()
	srcPng := filepath.Join(dir, "src.png")
	srcBmp := filepath.Join(dir, "src.bmp")
	srcJxr := filepath.Join(dir, "source.jxr")
	dest := filepath.Join(dir, "output")

	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{40, 140, 95, 255})
		}
	}
	f, err := os.Create(srcPng)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	if err := exec.Command("ffmpeg", "-y", "-i", srcPng, srcBmp).Run(); err != nil {
		t.Skipf("ffmpeg bmp encode skipped: %v", err)
	}
	if err := exec.Command("JxrEncApp", "-i", srcBmp, "-o", srcJxr).Run(); err != nil {
		t.Skipf("JxrEncApp skipped: %v", err)
	}

	mimeType, err := mediapreview.Convert(context.Background(), srcJxr, dest, "compatible", "application/octet-stream")
	if err != nil {
		t.Fatalf("Convert(.jxr) failed: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", mimeType)
	}
	outFile, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer outFile.Close()
	decoded, err := jpeg.Decode(outFile)
	if err != nil || decoded.Bounds().Dx() != 64 {
		t.Fatalf("invalid converted jpeg: %v", err)
	}
}
