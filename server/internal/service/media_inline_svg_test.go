package service_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestInlineSvgHtmlMedia(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write,delete")

	fetch := func(url string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("GET", h.base+"/api/v1"+url, nil)
		r, e := h.http.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { r.Body.Close() })
		return r
	}

	// A chart-style SVG sniffs as text/plain; the upload path must record the
	// image/svg+xml type so the inline lane accepts it.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 40"><text x="4" y="26">chart</text></svg>`
	uploaded, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "charts/bars.svg"}, "inline-svg", strings.NewReader(svg), nil)
	if err != nil {
		t.Fatal(err)
	}
	var listing struct {
		Entries []service.BrowserEntry `json:"entries"`
	}
	if got := h.call(t, "GET", "/browser?space_id="+h.space.ID+"&path=charts", nil, &listing); got != 200 || len(listing.Entries) != 1 {
		t.Fatalf("listing %d %+v", got, listing.Entries)
	}
	if m := listing.Entries[0].File.MIME; m != "image/svg+xml" {
		t.Fatalf("svg sniffed mime %q, want image/svg+xml", m)
	}

	r := fetch("/files/" + uploaded.FileID + "/media?version=" + uploaded.VersionID)
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("svg inline %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}
	csp := r.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox") || !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "script-src") {
		t.Fatalf("svg csp %q", csp)
	}
	if disp := r.Header.Get("Content-Disposition"); !strings.HasPrefix(disp, "inline") {
		t.Fatalf("svg disposition %q", disp)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "chart") {
		t.Fatal("svg body missing content")
	}

	page, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "charts/report.html"}, "inline-html", strings.NewReader("<!DOCTYPE html><html><head><style>h1{color:#20494c}</style></head><body><h1>report</h1></body></html>"), nil)
	if err != nil {
		t.Fatal(err)
	}
	hr := fetch("/files/" + page.FileID + "/media?version=" + page.VersionID)
	if hr.StatusCode != 200 || !strings.HasPrefix(hr.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("html inline %d %s", hr.StatusCode, hr.Header.Get("Content-Type"))
	}
	if csp := hr.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "img-src data:") || !strings.Contains(csp, "font-src data:") || !strings.Contains(csp, "style-src 'unsafe-inline'") {
		t.Fatalf("html csp %q", csp)
	}

	// The download lane keeps serving attachments with its own CSP.
	ir := fetch("/files/" + page.FileID + "/media?version=" + page.VersionID + "&preview=interactive")
	if ir.StatusCode != 200 {
		t.Fatalf("interactive status %d", ir.StatusCode)
	}
	icsp := ir.Header.Get("Content-Security-Policy")
	for _, directive := range []string{"sandbox allow-scripts", "script-src 'unsafe-inline'", "connect-src 'none'", "form-action 'none'", "frame-ancestors 'self'"} {
		if !strings.Contains(icsp, directive) {
			t.Fatalf("missing %s", directive)
		}
	}
	if strings.Contains(icsp, "allow-same-origin") {
		t.Fatal("interactive preview must retain opaque origin")
	}
	denied := fetch("/files/" + uploaded.FileID + "/media?version=" + uploaded.VersionID + "&preview=interactive")
	if denied.StatusCode == 200 {
		t.Fatal("SVG must not acquire script permission")
	}
	downloadDenied := fetch("/files/" + page.FileID + "/content?version=" + page.VersionID + "&preview=interactive")
	if strings.Contains(downloadDenied.Header.Get("Content-Security-Policy"), "allow-scripts") || !strings.HasPrefix(downloadDenied.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("attachment must not acquire script permission or become inline")
	}
	dr := fetch("/files/" + page.FileID + "/content?version=" + page.VersionID)
	if dr.StatusCode != 200 || !strings.HasPrefix(dr.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("download %d %s", dr.StatusCode, dr.Header.Get("Content-Disposition"))
	}
}
