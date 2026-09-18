package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rclone/rclone/fs/config"
)

// rclonePublicClientID is rclone's embedded public OneDrive client. A public
// client has no secret: the device-code flow needs no app registration.
const rclonePublicClientID = "b15665d9-eda6-4092-8539-0eec376afd59"

const rcloneOneDriveScopes = "Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All Sites.Read.All offline_access"

// DeviceAuth runs the one-time device-code sign-in against the Microsoft
// identity endpoint using the rclone public client. Tokens are persisted only
// into the rclone config store and never appear in output, logs or receipts.
type DeviceAuth struct {
	HTTP      *http.Client
	ClientID  string
	AuthBase  string
	GraphBase string

	// accessToken is held only in memory for same-session drive discovery.
	accessToken string
}

// NewDeviceAuth targets the production identity and Graph endpoints.
func NewDeviceAuth() *DeviceAuth {
	return &DeviceAuth{
		HTTP:      &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		ClientID:  rclonePublicClientID,
		AuthBase:  "https://login.microsoftonline.com",
		GraphBase: "https://graph.microsoft.com",
	}
}

func (d *DeviceAuth) clientID() string {
	if d.ClientID != "" {
		return d.ClientID
	}
	return rclonePublicClientID
}

// DeviceCode is the sign-in prompt for the user: a code to enter at the
// verification page. It carries no credential material.
type DeviceCode struct {
	DeviceCode      string `json:"-"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval_seconds,omitempty"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
}

// Start begins the device-code flow and returns the prompt to show the user.
func (d *DeviceAuth) Start(ctx context.Context) (DeviceCode, error) {
	form := url.Values{"client_id": {d.clientID()}, "scope": {rcloneOneDriveScopes}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.AuthBase+"/common/oauth2/v2.0/devicecode", strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCode{}, errors.New("device_flow_unavailable")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return DeviceCode{}, errors.New("device_flow_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return DeviceCode{}, errors.New("device_flow_unavailable")
	}
	// The wire device_code is a credential-grade secret: decode it, but never
	// expose it through the returned prompt struct.
	var wire struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wire) != nil || wire.DeviceCode == "" || wire.UserCode == "" || wire.VerificationURI == "" {
		return DeviceCode{}, errors.New("device_flow_unavailable")
	}
	code := DeviceCode{DeviceCode: wire.DeviceCode, UserCode: wire.UserCode, VerificationURI: wire.VerificationURI, Interval: wire.Interval, ExpiresIn: wire.ExpiresIn}
	if code.Interval <= 0 {
		code.Interval = 5
	}
	return code, nil
}

// storedToken is the rclone-compatible OAuth token persisted verbatim.
type storedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// Await polls the token endpoint until the user completes sign-in, then
// persists the token into the named rclone remote. The token itself never
// leaves this package.
func (d *DeviceAuth) Await(ctx context.Context, remote string, code DeviceCode) error {
	if remote == "" || strings.ContainsAny(remote, ":/\\") {
		return errors.New("invalid_remote")
	}
	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(15 * time.Minute)
	if code.ExpiresIn > 0 {
		deadline = time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	}
	for {
		select {
		case <-ctx.Done():
			return errors.New("operation_cancelled")
		case <-time.After(interval):
		}
		form := url.Values{"client_id": {d.clientID()}, "grant_type": {"urn:ietf:params:oauth:2.0:device_code"}, "device_code": {code.DeviceCode}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.AuthBase+"/common/oauth2/v2.0/token", strings.NewReader(form.Encode()))
		if err != nil {
			return errors.New("device_flow_unavailable")
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := d.HTTP.Do(req)
		if err != nil {
			return errors.New("device_flow_unavailable")
		}
		var body struct {
			storedToken
			Error     string `json:"error"`
			Interval  int    `json:"interval"`
			ExpiresIn int    `json:"expires_in"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body)
		resp.Body.Close()
		if decodeErr != nil {
			return errors.New("device_flow_unavailable")
		}
		switch {
		case resp.StatusCode == 200 && body.AccessToken != "":
			if body.ExpiresIn > 0 {
				body.Expiry = time.Now().UTC().Add(time.Duration(body.ExpiresIn) * time.Second)
			} else {
				body.Expiry = time.Now().UTC().Add(time.Hour)
			}
			if body.TokenType == "" {
				body.TokenType = "Bearer"
			}
			if err = d.saveRemote(remote, body.storedToken); err != nil {
				return err
			}
			d.accessToken = body.AccessToken
			return nil
		case body.Error == "authorization_pending":
			continue
		case body.Error == "slow_down":
			interval += 5 * time.Second
			continue
		case body.Error == "expired_token", time.Now().After(deadline):
			return errors.New("authentication_required")
		default:
			return errors.New("authentication_required")
		}
	}
}

// saveRemote writes the token into the rclone config store. Credentials only
// ever live in the rclone configuration, never in DriveBridge state.
func (d *DeviceAuth) saveRemote(remote string, t storedToken) error {
	blob, err := json.Marshal(t)
	if err != nil {
		return errors.New("device_flow_unavailable")
	}
	config.Data().SetValue(remote, "type", "onedrive")
	config.Data().SetValue(remote, "client_id", d.clientID())
	config.Data().SetValue(remote, "token", string(blob))
	if err := config.Data().Save(); err != nil {
		return errors.New("config_write_failed")
	}
	return nil
}

func (d *DeviceAuth) meGET(ctx context.Context, path string, out any) error {
	if d.accessToken == "" {
		return errors.New("authentication_required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.GraphBase+path, nil)
	if err != nil {
		return errors.New("graph_unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+d.accessToken)
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return errors.New("graph_unreachable")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 401, 403:
		return errors.New("authentication_required")
	case 404:
		return errors.New("remote_not_found")
	case 429:
		return errors.New("throttled")
	default:
		return errors.New("graph_unavailable")
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out) != nil {
		return errors.New("invalid_graph_response")
	}
	return nil
}

// DiscoverDefaultDrive resolves the account's default drive and its root item
// immediately after a completed sign-in.
func (d *DeviceAuth) DiscoverDefaultDrive(ctx context.Context) (driveID, rootID string, err error) {
	var drive struct {
		ID string `json:"id"`
	}
	if err = d.meGET(ctx, "/v1.0/me/drive", &drive); err != nil {
		return "", "", err
	}
	if drive.ID == "" {
		return "", "", errors.New("invalid_graph_response")
	}
	var root struct {
		ID     string    `json:"id"`
		Folder *struct{} `json:"folder"`
	}
	if err = d.meGET(ctx, "/v1.0/me/drive/root", &root); err != nil {
		return "", "", err
	}
	if root.ID == "" || root.Folder == nil {
		return "", "", errors.New("invalid_graph_response")
	}
	return drive.ID, root.ID, nil
}

// RootFolder is a selectable folder directly under the drive root.
type RootFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RootFolders lists the drive root's folder children for name-based root
// selection. Files are skipped.
func (d *DeviceAuth) RootFolders(ctx context.Context) ([]RootFolder, error) {
	var data struct {
		Items []struct {
			ID     string    `json:"id"`
			Name   string    `json:"name"`
			Folder *struct{} `json:"folder"`
		} `json:"value"`
	}
	if err := d.meGET(ctx, "/v1.0/me/drive/root/children?$top=200&$select=id,name,folder", &data); err != nil {
		return nil, err
	}
	out := []RootFolder{}
	for _, i := range data.Items {
		if i.Folder != nil && i.ID != "" && i.Name != "" {
			out = append(out, RootFolder{ID: i.ID, Name: i.Name})
		}
	}
	return out, nil
}

// SelectRootFolder resolves a root by exact folder name under the drive root.
func (d *DeviceAuth) SelectRootFolder(ctx context.Context, name string) (string, error) {
	folders, err := d.RootFolders(ctx)
	if err != nil {
		return "", err
	}
	for _, f := range folders {
		if f.Name == name {
			return f.ID, nil
		}
	}
	return "", errors.New("folder_not_found")
}

// SetDrive persists the discovered drive and root selection into the rclone
// remote so later commands (space add --auto) resolve them offline.
func (d *DeviceAuth) SetDrive(remote, driveID, rootID string) error {
	if remote == "" || driveID == "" {
		return errors.New("invalid_remote")
	}
	config.Data().SetValue(remote, "drive_id", driveID)
	if rootID != "" {
		config.Data().SetValue(remote, "root_folder_id", rootID)
	}
	if err := config.Data().Save(); err != nil {
		return errors.New("config_write_failed")
	}
	return nil
}

// ListRootFolders lists the drive-root folders through an already-configured
// rclone remote; it borrows rclone's credential and refresh machinery.
func (r *OneDrive) ListRootFolders(ctx context.Context, remoteName string) ([]RootFolder, error) {
	drive := config.GetValue(remoteName, "drive_id")
	if drive == "" {
		return nil, errors.New("authentication_required")
	}
	p := Space{ID: "discovery-" + remoteName, Remote: remoteName, DriveID: drive}
	var data struct {
		Items []struct {
			ID     string    `json:"id"`
			Name   string    `json:"name"`
			Folder *struct{} `json:"folder"`
		} `json:"value"`
	}
	if err := r.graph(ctx, p, "root/children?$top=200&$select=id,name,folder", &data); err != nil {
		return nil, err
	}
	out := []RootFolder{}
	for _, i := range data.Items {
		if i.Folder != nil && i.ID != "" && i.Name != "" {
			out = append(out, RootFolder{ID: i.ID, Name: i.Name})
		}
	}
	return out, nil
}

// SelectRootFolderOnRemote matches one drive-root folder by exact name.
func (r *OneDrive) SelectRootFolderOnRemote(ctx context.Context, remoteName, name string) (string, error) {
	folders, err := r.ListRootFolders(ctx, remoteName)
	if err != nil {
		return "", err
	}
	for _, f := range folders {
		if f.Name == name {
			return f.ID, nil
		}
	}
	return "", errors.New("folder_not_found")
}

// Probe performs one authenticated request through the rclone credential
// path (rclone refreshes and stores tokens as usual). It backs the optional
// doctor --probe-auth check; the default doctor run never calls it, and no
// credential material ever leaves this call.
func (r *OneDrive) Probe(ctx context.Context, remoteName string) error {
	drive := config.GetValue(remoteName, "drive_id")
	if drive == "" {
		return errors.New("authentication_required")
	}
	p := Space{ID: "doctor-probe", Remote: remoteName, DriveID: drive}
	_, err := r.token(ctx, p)
	return err
}
