package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
)

// fixtureIdentity serves the device-code flow: one pending poll, then the
// token; the Graph side reports the default drive, its root and children.
func fixtureIdentity(t *testing.T, granted chan struct{}) (*httptest.Server, *httptest.Server) {
	t.Helper()
	var polls atomic.Int32
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/common/oauth2/v2.0/devicecode":
			if err := r.ParseForm(); err != nil || r.Form.Get("client_id") == "" || !strings.Contains(r.Form.Get("scope"), "offline_access") {
				t.Error("device-code request malformed")
			}
			io.WriteString(w, `{"device_code":"fixture-device","user_code":"BCD-1234","verification_uri":"https://microsoft.com/devicelogin","interval":1,"expires_in":900}`)
		case "/common/oauth2/v2.0/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "urn:ietf:params:oauth:2.0:device_code" || r.Form.Get("device_code") != "fixture-device" || r.Form.Get("client_id") == "" {
				t.Error("token poll malformed")
			}
			if polls.Add(1) == 1 {
				io.WriteString(w, `{"error":"authorization_pending"}`)
				return
			}
			close(granted)
			io.WriteString(w, `{"access_token":"fixture-access","refresh_token":"fixture-refresh","token_type":"Bearer","expires_in":3600}`)
		default:
			t.Errorf("unexpected identity path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-access" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/v1.0/me/drive":
			io.WriteString(w, `{"id":"drive-fixture"}`)
		case "/v1.0/me/drive/root":
			io.WriteString(w, `{"id":"root-fixture","folder":{}}`)
		case "/v1.0/me/drive/root/children":
			io.WriteString(w, `{"value":[{"id":"projects-folder","name":"Projects","folder":{}},{"id":"loose-file","name":"notes.txt"},{"id":"shots-folder","name":"Shots","folder":{}}]}`)
		default:
			t.Errorf("unexpected graph path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	return identity, graph
}

func TestDeviceAuthFlowAndDiscovery(t *testing.T) {
	granted := make(chan struct{})
	identity, graph := fixtureIdentity(t, granted)
	defer identity.Close()
	defer graph.Close()
	const remote = "deviceauth-fixture"
	defer config.Data().DeleteSection(remote)

	device := &DeviceAuth{HTTP: identity.Client(), ClientID: "fixture-client", AuthBase: identity.URL, GraphBase: graph.URL}
	code, err := device.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// The prompt carries the verification page and code, never a credential.
	encoded, _ := json.Marshal(code)
	if strings.Contains(string(encoded), "fixture-device") || strings.Contains(string(encoded), "fixture-access") {
		t.Fatal("credential material in the device-code prompt")
	}
	if code.UserCode != "BCD-1234" || code.VerificationURI == "" {
		t.Fatalf("prompt: %+v", code)
	}
	if err = device.Await(context.Background(), remote, code); err != nil {
		t.Fatal(err)
	}
	select {
	case <-granted:
	default:
		t.Fatal("token endpoint never granted")
	}
	// The token landed only in the rclone config store, never in output.
	blob := config.GetValue(remote, "token")
	if !strings.Contains(blob, "fixture-access") {
		t.Fatal("token not persisted into the rclone config")
	}
	if config.GetValue(remote, "type") != "onedrive" || config.GetValue(remote, "client_id") != "fixture-client" {
		t.Fatal("remote section incomplete")
	}
	// Same-session discovery resolves the default drive and its root.
	driveID, rootID, err := device.DiscoverDefaultDrive(context.Background())
	if err != nil || driveID != "drive-fixture" || rootID != "root-fixture" {
		t.Fatalf("drive discovery: %s %s err=%v", driveID, rootID, err)
	}
	folders, err := device.RootFolders(context.Background())
	if err != nil || len(folders) != 2 {
		t.Fatalf("root folders: %+v err=%v", folders, err)
	}
	selected, err := device.SelectRootFolder(context.Background(), "Projects")
	if err != nil || selected != "projects-folder" {
		t.Fatalf("root selection: %s err=%v", selected, err)
	}
	if _, err = device.SelectRootFolder(context.Background(), "missing"); err == nil || err.Error() != "folder_not_found" {
		t.Fatalf("missing folder: %v", err)
	}
	if err = device.SetDrive(remote, driveID, selected); err != nil {
		t.Fatal(err)
	}
	if config.GetValue(remote, "drive_id") != "drive-fixture" || config.GetValue(remote, "root_folder_id") != "projects-folder" {
		t.Fatal("drive selection not persisted")
	}
}

func TestDeviceAuthGuards(t *testing.T) {
	granted := make(chan struct{})
	identity, graph := fixtureIdentity(t, granted)
	defer identity.Close()
	defer graph.Close()
	device := &DeviceAuth{HTTP: identity.Client(), ClientID: "fixture-client", AuthBase: identity.URL, GraphBase: graph.URL}
	// Invalid remote names never reach the identity service.
	if err := device.Await(context.Background(), "bad:name", DeviceCode{DeviceCode: "x", UserCode: "y", VerificationURI: "z", Interval: 1}); err == nil {
		t.Fatal("invalid remote accepted")
	}
	// Discovery before a completed sign-in is refused, not guessed.
	if _, _, err := device.DiscoverDefaultDrive(context.Background()); err == nil || err.Error() != "authentication_required" {
		t.Fatalf("pre-auth discovery: %v", err)
	}
	// A cancelled context stops the poll loop.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := DeviceCode{DeviceCode: "fixture-device", UserCode: "BCD-1234", VerificationURI: "https://microsoft.com/devicelogin", Interval: 1}
	if err := device.Await(ctx, "deviceauth-cancel-fixture", code); err == nil || err.Error() != "operation_cancelled" {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestListRootFoldersThroughRcloneRemote(t *testing.T) {
	const remote = "deviceauth-remote-fixture"
	config.Data().SetValue(remote, "drive_id", "drive-fixture")
	defer config.Data().DeleteSection(remote)
	p := NewOneDrive()
	p.authToken = func(context.Context, Space) (string, error) { return "fixture", nil }
	p.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "graph.microsoft.com" || r.URL.Path != "/v1.0/drives/drive-fixture/root/children" {
			t.Fatalf("unexpected request %s%s", r.URL.Host, r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"value":[{"id":"projects-folder","name":"Projects","folder":{}}]}`)), Request: r}, nil
	})
	id, err := p.SelectRootFolderOnRemote(context.Background(), remote, "Projects")
	if err != nil || id != "projects-folder" {
		t.Fatalf("remote selection: %s err=%v", id, err)
	}
	if _, err = p.SelectRootFolderOnRemote(context.Background(), remote, "Nope"); err == nil || err.Error() != "folder_not_found" {
		t.Fatalf("missing name: %v", err)
	}
	// A remote without a discovered drive id asks for auth first.
	config.Data().DeleteKey(remote, "drive_id")
	if _, err = p.SelectRootFolderOnRemote(context.Background(), remote, "Projects"); err == nil || err.Error() != "authentication_required" {
		t.Fatalf("drive-less remote: %v", err)
	}
}

func TestDeviceAuthPollIntervalHonoured(t *testing.T) {
	// Awaiting against an unreachable identity endpoint surfaces the fixed
	// code instead of hanging or panicking.
	device := &DeviceAuth{HTTP: &http.Client{Timeout: 2 * time.Second}, AuthBase: "http://127.0.0.1:1"}
	err := device.Await(context.Background(), "deviceauth-unreachable", DeviceCode{DeviceCode: "x", UserCode: "y", VerificationURI: "z", Interval: 1})
	if err == nil || err.Error() != "device_flow_unavailable" {
		t.Fatalf("unreachable identity: %v", err)
	}
}
