package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/lib/oauthutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGraphScopesVersionsAndErrors(t *testing.T) {
	p := Space{ID: "space", DriveID: "drive", RootID: "allowed"}
	provider := NewOneDrive()
	provider.authToken = func(context.Context, Space) (string, error) { return "fixture", nil }
	var parent = "allowed"
	var status = 200
	provider.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer fixture" || r.URL.Host != "graph.microsoft.com" {
			t.Fatal("wrong Graph authentication target")
		}
		body := fmt.Sprintf(`{"id":"item","name":"video.mp4","size":10,"eTag":"v1","parentReference":{"id":%q,"driveId":"drive"},"file":{"mimeType":"video/mp4"},"@microsoft.graph.downloadUrl":"https://fixture.files.1drv.com/data?opaque=fixture"}`, parent)
		if strings.Contains(r.URL.Path, "outside") {
			body = `{"id":"outside","parentReference":{"id":"","driveId":"drive"}}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	file, e := provider.Stat(context.Background(), p, "item")
	if e != nil || file.Version != "v1" {
		t.Fatal("metadata contract failed")
	}
	if _, e = provider.Access(context.Background(), p, File{ItemID: "item", Version: "v2", Size: 10}); e == nil {
		t.Fatal("changed version accepted")
	}
	parent = "outside"
	if _, e = provider.Stat(context.Background(), p, "item"); e == nil {
		t.Fatal("out-of-root item accepted")
	}
	parent = "allowed"
	for code, want := range map[int]string{401: "authentication_required", 403: "authentication_required", 404: "remote_not_found", 429: "throttled", 507: "quota_exceeded", 302: "graph_unavailable"} {
		status = code
		if _, e = provider.Stat(context.Background(), p, "item"); e == nil || e.Error() != want {
			t.Fatalf("Graph status %d misclassified", code)
		}
	}
}
func TestRcloneRefreshSharesOneCredentialMapper(t *testing.T) {
	var refreshed atomic.Int32
	oauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshed.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"fixture-new","refresh_token":"fixture-refresh-next","token_type":"Bearer","expires_in":3600}`)
	}))
	defer oauth.Close()
	token, _ := json.Marshal(map[string]any{"access_token": "fixture-old", "refresh_token": "fixture-refresh", "token_type": "Bearer", "expiry": time.Now().Add(-time.Hour)})
	mapper := configmap.Simple{"token": string(token)}
	client, _, e := oauthutil.NewClientWithBaseClient(context.Background(), "fixture", mapper, &oauthutil.Config{ClientID: "fixture-client", TokenURL: oauth.URL, AuthURL: oauth.URL}, &http.Client{})
	if e != nil {
		t.Fatal("rclone OAuth initialization failed")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-new" {
			t.Error("renewed authentication not used")
		}
		w.WriteHeader(204)
	}))
	defer target.Close()
	for i := 0; i < 2; i++ {
		response, e := client.Get(target.URL)
		if e != nil {
			t.Fatal("authenticated fixture request failed")
		}
		response.Body.Close()
	}
	current, e := oauthutil.GetToken("fixture", mapper)
	if e != nil || !current.Valid() || refreshed.Load() != 1 {
		t.Fatal("shared refreshed token was not reusable")
	}
}
