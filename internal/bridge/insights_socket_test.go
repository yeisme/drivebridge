package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dialUnix(t *testing.T, socket string) *http.Client {
	t.Helper()
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
}

func postSocket(t *testing.T, client *http.Client, path, body string) (int, string) {
	t.Helper()
	resp, e := client.Post("http://localhost"+path, "application/json", strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, string(out)
}

func listFixture(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 8 << 20}); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"a.bin", "b.bin"} {
		if e := os.WriteFile(filepath.Join(remoteRoot, name), []byte("payload-"+name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Mkdir(filepath.Join(remoteRoot, "sub"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(remoteRoot, "sub", "inner.bin"), []byte("inner-payload"), 0600); e != nil {
		t.Fatal(e)
	}
	socket := filepath.Join(t.TempDir(), "insights.sock")
	srv, e := s.ListenInsights(socket)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { srv.Close() })
	return s, local, remoteRoot, socket
}

func TestInsightsSocketListRemembersIntoIndex(t *testing.T) {
	s, _, _, socket := listFixture(t)
	client := dialUnix(t, socket)

	code, body := postSocket(t, client, "/list", `{"space_id":"nas","path":""}`)
	if code != 200 {
		t.Fatalf("root list failed %d %s", code, body)
	}
	var envelope struct {
		Data struct {
			Items    []File `json:"items"`
			Partial  bool   `json:"partial"`
			Coverage string `json:"coverage"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &envelope) != nil {
		t.Fatal("invalid envelope")
	}
	if len(envelope.Data.Items) != 2 || envelope.Data.Partial || envelope.Data.Coverage != "live_listing" {
		t.Fatalf("unexpected root listing %+v", envelope.Data)
	}
	if code, body = postSocket(t, client, "/list", `{"space_id":"nas","path":"sub"}`); code != 200 {
		t.Fatalf("sub list failed %d %s", code, body)
	}
	if json.Unmarshal([]byte(body), &envelope) != nil || len(envelope.Data.Items) != 1 {
		t.Fatalf("unexpected sub listing %s", body)
	}
	var count int64
	s.DB.Model(&File{}).Count(&count)
	if count != 3 {
		t.Fatalf("listing did not remember into index: %d", count)
	}
}

func TestInsightsSocketListFilesKindDirProjection(t *testing.T) {
	s, _, _, socket := listFixture(t)
	client := dialUnix(t, socket)
	for _, path := range []string{"", "sub"} {
		if _, body := postSocket(t, client, "/list", `{"space_id":"nas","path":"`+path+`"}`); len(body) == 0 {
			t.Fatal("list failed")
		}
	}
	q := url.Values{"space_id": {"nas"}, "dir": {""}}
	out, e := QueryInsights(s.Insights, "files", q)
	if e != nil {
		t.Fatal(e)
	}
	items := out.(map[string]any)["items"].([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("root dir projection expected 2 files, got %v", items)
	}
	for _, item := range items {
		if item["dir"] != "" {
			t.Fatalf("projection missing enrichment fields: %v", item)
		}
		// MIME stays empty when a generic listing cannot infer it; the key
		// itself must exist so the client can rely on the shape.
		if _, ok := item["mime_type"]; !ok {
			t.Fatalf("projection missing mime_type key: %v", item)
		}
		if _, seen := item["observed_at"]; !seen {
			t.Fatalf("projection missing observed_at: %v", item)
		}
	}
	q = url.Values{"space_id": {"nas"}, "dir": {"sub"}}
	out, _ = QueryInsights(s.Insights, "files", q)
	if items := out.(map[string]any)["items"].([]map[string]any); len(items) != 1 {
		t.Fatalf("sub dir projection expected 1 file, got %v", items)
	}
}

func TestInsightsSocketListErrors(t *testing.T) {
	s, _, _, socket := listFixture(t)
	client := dialUnix(t, socket)

	if code, body := postSocket(t, client, "/list", `{"space_id":"missing"}`); code != 400 || !strings.Contains(body, `"space_not_allowed"`) {
		t.Fatalf("unknown space: %d %s", code, body)
	}
	if code, body := postSocket(t, client, "/list", `{"space_id":"nas","path":"../escape"}`); code != 400 || !strings.Contains(body, `"path_not_allowed"`) {
		t.Fatalf("traversal: %d %s", code, body)
	}
	if code, body := postSocket(t, client, "/list", `not json`); code != 400 {
		t.Fatalf("bad body accepted: %d %s", code, body)
	}
	if _, e := s.RunListRequest(context.Background(), ListRequest{Space: "nas", Path: strings.Repeat("a", 1025)}); listError(e) != "invalid_list" {
		t.Fatalf("oversized path: %v", e)
	}

	// A service that was started without --enabled refuses live listing.
	disabled, e := New(filepath.Join(t.TempDir(), "state2"), NewOneDrive(), false)
	if e != nil {
		t.Fatal(e)
	}
	defer disabled.Close()
	if e := disabled.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: t.TempDir(), MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	if _, e := disabled.RunListRequest(context.Background(), ListRequest{Space: "nas"}); listError(e) != "listing_disabled" {
		t.Fatalf("disabled listing: %v", e)
	}
}

func TestInsightsSocketListTruncation(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 8 << 20}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 501; i++ {
		name := fmt.Sprintf("f%04d.bin", i)
		if e := os.WriteFile(filepath.Join(remoteRoot, name), []byte("x"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	socket := filepath.Join(t.TempDir(), "insights.sock")
	srv, e := s.ListenInsights(socket)
	if e != nil {
		t.Fatal(e)
	}
	defer srv.Close()
	code, body := postSocket(t, dialUnix(t, socket), "/list", `{"space_id":"nas"}`)
	if code != 200 {
		t.Fatalf("list failed %d %s", code, body)
	}
	var envelope struct {
		Data struct {
			Items   []File `json:"items"`
			Partial bool   `json:"partial"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &envelope) != nil || len(envelope.Data.Items) != 500 || !envelope.Data.Partial {
		t.Fatalf("truncation contract broken: %d items partial=%v", len(envelope.Data.Items), envelope.Data.Partial)
	}
}
