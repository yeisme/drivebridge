// Package mediapreview contains the bounded local renderer shared by the web
// service and the private local owner. Callers own authorization and caching.
package mediapreview

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Converter(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".pdf" {
		return "pdftoppm"
	}
	if ext == ".docx" || ext == ".xlsx" || ext == ".pptx" {
		return "libreoffice"
	}
	if strings.Contains(",.dng,.cr2,.cr3,.nef,.arw,.raf,.orf,.rw2,", ","+ext+",") {
		if _, err := exec.LookPath("dcraw_emu"); err == nil {
			return "dcraw_emu"
		}
		return "dcraw"
	}
	return "ffmpeg"
}
func sandbox(ctx context.Context, dir, executable string, args ...string) *exec.Cmd {
	prefix := []string{"--unshare-all", "--die-with-parent", "--clearenv", "--setenv", "HOME", "/work", "--setenv", "PATH", "/usr/bin:/bin:/home/linuxbrew/.linuxbrew/bin", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind-try", "/lib64", "/lib64", "--ro-bind-try", "/home/linuxbrew/.linuxbrew", "/home/linuxbrew/.linuxbrew", "--ro-bind-try", "/etc/fonts", "/etc/fonts", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", dir, "/work", "--chdir", "/work", "--", executable}
	return exec.CommandContext(ctx, "bwrap", append(prefix, args...)...)
}
func pdf(ctx context.Context, dir, input, dest string) error {
	executable, err := exec.LookPath("pdftoppm")
	if err != nil {
		return errors.New("preview_converter_unavailable")
	}
	if _, err = exec.LookPath("bwrap"); err != nil {
		return errors.New("preview_sandbox_unavailable")
	}
	if sandbox(ctx, dir, executable, "-f", "1", "-singlefile", "-scale-to", "480", "-jpeg", "/work/"+filepath.Base(input), "/work/pdf-thumb").Run() != nil {
		return errors.New("preview_conversion_failed")
	}
	if os.Rename(filepath.Join(dir, "pdf-thumb.jpg"), dest) != nil {
		return errors.New("preview_conversion_failed")
	}
	return nil
}

// Convert writes a JPEG thumbnail, a compatible MP4 or a read-only PDF. Input
// and destination must be within the caller's private temporary directory.
func Convert(ctx context.Context, input, dest, kind, mimeType string) (string, error) {
	fail := errors.New("preview_conversion_failed")
	dir := filepath.Dir(input)
	if filepath.Dir(dest) != dir || kind != "thumbnail" && kind != "compatible" {
		return "", errors.New("invalid_preview")
	}
	converter := Converter(input)
	executable, err := exec.LookPath(converter)
	if err != nil {
		return "", errors.New("preview_converter_unavailable")
	}
	if converter == "libreoffice" {
		if _, err = exec.LookPath("bwrap"); err != nil {
			return "", errors.New("preview_sandbox_unavailable")
		}
		if sandbox(ctx, dir, executable, "--headless", "--safe-mode", "-env:UserInstallation=file:///work/profile", "--convert-to", "pdf", "--outdir", "/work", "/work/"+filepath.Base(input)).Run() != nil {
			return "", fail
		}
		converted := strings.TrimSuffix(input, filepath.Ext(input)) + ".pdf"
		if kind == "thumbnail" {
			if err = pdf(ctx, dir, converted, dest); err != nil {
				return "", err
			}
			return "image/jpeg", nil
		}
		if os.Rename(converted, dest) != nil {
			return "", fail
		}
		return "application/pdf", nil
	}
	if converter == "pdftoppm" {
		if err = pdf(ctx, dir, input, dest); err != nil {
			return "", err
		}
		return "image/jpeg", nil
	}
	if converter == "dcraw" || converter == "dcraw_emu" {
		if _, err = exec.LookPath("bwrap"); err != nil {
			return "", errors.New("preview_sandbox_unavailable")
		}
		ppm := filepath.Join(dir, "decoded.ppm")
		defer os.Remove(ppm)
		output, err := os.OpenFile(ppm, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return "", fail
		}
		cmd := sandbox(ctx, dir, executable, "-c", "-w", "-q", "0", "/work/"+filepath.Base(input))
		cmd.Stdout = &boundedWriter{output, 512 << 20}
		runErr := cmd.Run()
		closeErr := output.Close()
		if runErr != nil || closeErr != nil {
			return "", fail
		}
		input = ppm
	}
	args := []string{"-nostdin", "-v", "error", "-y", "-threads", "1", "-max_alloc", "268435456", "-protocol_whitelist", "file,pipe", "-format_whitelist", "mov,matroska,avi,mpegts,mpeg,mpegvideo,ogg,mp3,wav,flac,image2,png_pipe,jpeg_pipe,ppm_pipe,tiff_pipe,psd_pipe,gif,webp_pipe,bmp_pipe", "-i", input, "-map_metadata", "-1", "-threads", "1", "-filter_threads", "1"}
	outMIME := "image/jpeg"
	if kind == "compatible" && strings.HasPrefix(mimeType, "video/") {
		outMIME = "video/mp4"
		args = append(args, "-vf", "scale='min(1280,iw)':-2", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", "-fs", "536870912", "-f", "mp4", dest)
	} else {
		width := "480"
		if kind == "compatible" {
			width = "1600"
		}
		args = append(args, "-frames:v", "1", "-vf", "scale='min("+width+",iw)':-2", "-fs", "16777216", "-f", "image2", dest)
	}
	if exec.CommandContext(ctx, "ffmpeg", args...).Run() != nil {
		return "", fail
	}
	return outMIME, nil
}

type boundedWriter struct {
	writer io.Writer
	left   int64
}

func (w *boundedWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.left {
		return 0, errors.New("preview_size_limit")
	}
	n, err := w.writer.Write(b)
	w.left -= int64(n)
	return n, err
}
