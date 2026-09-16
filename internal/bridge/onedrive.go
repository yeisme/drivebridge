package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/onedrive"
	onedriveapi "github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/oauthutil"
)

// OneDrive uses rclone as the only refresh-token authority. Graph borrows the
// current access token in memory after a rclone authenticated request. It never
// copies tokens to DriveBridge state, MCP output or a second credential store.
type OneDrive struct {
	mu        sync.Mutex
	clients   map[string]fs.Fs
	HTTP      *http.Client
	authToken func(context.Context, Space) (string, error)
}

func NewOneDrive() *OneDrive {
	return &OneDrive{clients: map[string]fs.Fs{}, HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (r *OneDrive) mapper(p Space) (*configmap.Map, error) {
	if config.GetValue(p.Remote, "type") != "onedrive" || config.GetValue(p.Remote, "drive_id") != p.DriveID {
		return nil, errors.New("space_not_allowed")
	}
	region := config.GetValue(p.Remote, "region")
	if region != "" && region != "global" {
		return nil, errors.New("region_not_supported")
	}
	reg, err := fs.Find("onedrive")
	if err != nil {
		return nil, errors.New("authentication_required")
	}
	return fs.ConfigMap("onedrive", reg.Options, p.Remote, configmap.Simple{"drive_id": p.DriveID, "root_folder_id": p.RootID}), nil
}
func (r *OneDrive) client(ctx context.Context, p Space) (fs.Fs, error) {
	m, err := r.mapper(p)
	if err != nil {
		return nil, err
	}
	if f, ok := r.clients[p.ID]; ok {
		return f, nil
	}
	f, err := onedrive.NewFs(ctx, p.Remote, "", m)
	if err != nil {
		return nil, errors.New("authentication_required")
	}
	r.clients[p.ID] = f
	return f, nil
}
func (r *OneDrive) token(ctx context.Context, p Space) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, err := r.client(ctx, p)
	if err != nil {
		return "", err
	}
	// About performs an authenticated request and lets rclone refresh/save tokens.
	if f.Features().About == nil {
		return "", errors.New("authentication_required")
	}
	if _, err = f.Features().About(ctx); err != nil {
		return "", errors.New("authentication_required")
	}
	m, err := r.mapper(p)
	if err != nil {
		return "", err
	}
	t, err := oauthutil.GetToken(p.Remote, m)
	if err != nil || !t.Valid() {
		return "", errors.New("authentication_required")
	}
	return t.AccessToken, nil
}

type driveItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	ETag   string `json:"eTag"`
	Parent struct {
		ID    string `json:"id"`
		Drive string `json:"driveId"`
	} `json:"parentReference"`
	Folder *struct{} `json:"folder"`
	File   *struct {
		MIME string `json:"mimeType"`
	} `json:"file"`
	Video *struct {
		Duration int64 `json:"duration"`
	} `json:"video"`
	Download string `json:"@microsoft.graph.downloadUrl"`
}

func (i driveItem) file(p Space) File {
	f := File{ItemID: i.ID, DriveID: i.Parent.Drive, Version: i.ETag, Name: i.Name, Size: i.Size}
	if i.File != nil {
		f.MIME = i.File.MIME
	}
	if i.Video != nil {
		f.Duration = int((i.Video.Duration + 999) / 1000)
	}
	return f
}
func (r *OneDrive) graph(ctx context.Context, p Space, path string, out any) error {
	getToken := r.token
	if r.authToken != nil {
		getToken = r.authToken
	}
	token, err := getToken(ctx, p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.microsoft.com/v1.0/drives/"+url.PathEscape(p.DriveID)+"/"+path, nil)
	if err != nil {
		return errors.New("invalid_graph_request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := r.HTTP.Do(req)
	if err != nil {
		return errors.New("graph_unreachable")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200:
	case 401, 403:
		return errors.New("authentication_required")
	case 404:
		return errors.New("remote_not_found")
	case 429:
		return errors.New("throttled")
	case 507:
		return errors.New("quota_exceeded")
	default:
		return errors.New("graph_unavailable")
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(out) != nil {
		return errors.New("invalid_graph_response")
	}
	return nil
}
func (r *OneDrive) item(ctx context.Context, p Space, id string, access bool) (driveItem, error) {
	var i driveItem
	fields := "id,name,size,eTag,parentReference,file,video,folder"
	if access {
		fields += ",@microsoft.graph.downloadUrl"
	}
	err := r.graph(ctx, p, "items/"+url.PathEscape(id)+"?$select="+url.QueryEscape(fields), &i)
	if err != nil {
		return i, err
	}
	if i.ID != id || i.Parent.Drive != p.DriveID {
		return i, errors.New("remote_outside_space")
	}
	parent := i.Parent.ID
	for n := 0; n < 128; n++ {
		if parent == p.RootID {
			return i, nil
		}
		if parent == "" || parent == id {
			return i, errors.New("remote_outside_space")
		}
		var ancestor driveItem
		if err = r.graph(ctx, p, "items/"+url.PathEscape(parent)+"?$select=id,parentReference", &ancestor); err != nil {
			return i, err
		}
		if ancestor.Parent.Drive != "" && ancestor.Parent.Drive != p.DriveID {
			return i, errors.New("remote_outside_space")
		}
		parent = ancestor.Parent.ID
	}
	return i, errors.New("remote_outside_space")
}

// folder resolves a workspace-relative directory to its item ID; empty
// means the space root itself. Path addressing keeps the answer inside the
// pinned root by construction.
func (r *OneDrive) folder(ctx context.Context, p Space, dir string) (string, error) {
	if dir == "" {
		return p.RootID, nil
	}
	var i driveItem
	if err := r.graph(ctx, p, "root:/"+dir+":?$select=id,name,parentReference,folder", &i); err != nil {
		return "", err
	}
	if i.ID == "" || i.Folder == nil || i.Parent.Drive != p.DriveID {
		return "", errors.New("remote_not_found")
	}
	return i.ID, nil
}
func (r *OneDrive) List(ctx context.Context, p Space, dir string) ([]File, error) {
	folderID, err := r.folder(ctx, p, dir)
	if err != nil {
		return nil, err
	}
	path := "items/" + url.PathEscape(folderID) + "/children?$top=200&$select=id,name,size,eTag,parentReference,file,video,folder"
	out := []File{}
	for page := 0; page < 100; page++ {
		var data struct {
			Items []driveItem `json:"value"`
			Next  string      `json:"@odata.nextLink"`
		}
		if err := r.graph(ctx, p, path, &data); err != nil {
			return nil, err
		}
		for _, i := range data.Items {
			if i.Folder != nil && i.Parent.ID == folderID && i.Parent.Drive == p.DriveID {
				out = append(out, File{Directory: true, Name: i.Name, Dir: dir})
				continue
			}
			if i.File != nil && i.Parent.ID == folderID && i.Parent.Drive == p.DriveID {
				out = append(out, i.file(p))
			}
		}
		if data.Next == "" {
			return out, nil
		}
		prefix := "https://graph.microsoft.com/v1.0/drives/" + url.PathEscape(p.DriveID) + "/"
		if !strings.HasPrefix(data.Next, prefix) {
			return nil, errors.New("invalid_graph_pagination")
		}
		path = strings.TrimPrefix(data.Next, prefix)
	}
	return nil, errors.New("listing_limit_exceeded")
}

// Mkdir creates each path segment under the pinned root. An existing folder
// segment is descended; an existing file with that name is a conflict.
func (r *OneDrive) Mkdir(ctx context.Context, p Space, dir string) error {
	folderID, err := r.folder(ctx, p, dir)
	if err == nil {
		_ = folderID
		return nil
	} else if err.Error() != "remote_not_found" {
		return err
	}
	current := p.RootID
	segments := strings.Split(dir, "/")
	for _, segment := range segments {
		if !safeName(segment) {
			return errors.New("path_not_allowed")
		}
		var created driveItem
		body := `{"name":` + strconv.Quote(segment) + `,"folder":{},"@name.conflictBehavior":"fail"}`
		err := r.graphPost(ctx, p, "items/"+url.PathEscape(current)+"/children", body, &created)
		if err != nil {
			if err.Error() != "target_conflict" {
				return err
			}
			next, listErr := r.childFolder(ctx, p, current, segment)
			if listErr != nil {
				return listErr
			}
			current = next
			continue
		}
		if created.ID == "" || created.Parent.Drive != p.DriveID {
			return errors.New("remote_outside_space")
		}
		current = created.ID
	}
	return nil
}
func (r *OneDrive) childFolder(ctx context.Context, p Space, parentID, name string) (string, error) {
	var data struct {
		Items []driveItem `json:"value"`
	}
	if err := r.graph(ctx, p, "items/"+url.PathEscape(parentID)+"/children?$top=200&$select=id,name,parentReference,folder", &data); err != nil {
		return "", err
	}
	for _, i := range data.Items {
		if i.Name == name && i.Folder != nil && i.Parent.Drive == p.DriveID {
			return i.ID, nil
		}
	}
	return "", errors.New("target_conflict")
}
func (r *OneDrive) graphPost(ctx context.Context, p Space, path, body string, out any) error {
	getToken := r.token
	if r.authToken != nil {
		getToken = r.authToken
	}
	token, err := getToken(ctx, p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.microsoft.com/v1.0/drives/"+url.PathEscape(p.DriveID)+"/"+path, strings.NewReader(body))
	if err != nil {
		return errors.New("invalid_graph_request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := r.HTTP.Do(req)
	if err != nil {
		return errors.New("graph_unreachable")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200, 201:
	case 401, 403:
		return errors.New("authentication_required")
	case 404:
		return errors.New("remote_not_found")
	case 409:
		return errors.New("target_conflict")
	case 429:
		return errors.New("throttled")
	case 507:
		return errors.New("quota_exceeded")
	default:
		return errors.New("graph_unavailable")
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out) != nil {
		return errors.New("invalid_graph_response")
	}
	return nil
}
func (r *OneDrive) graphPatch(ctx context.Context, p Space, path, body string, out any) error {
	err := r.graphMutate(ctx, http.MethodPatch, p, path, body, out)
	return err
}
func (r *OneDrive) graphDelete(ctx context.Context, p Space, path string) error {
	return r.graphMutate(ctx, http.MethodDelete, p, path, "", nil)
}
func (r *OneDrive) graphMutate(ctx context.Context, method string, p Space, path, body string, out any) error {
	getToken := r.token
	if r.authToken != nil {
		getToken = r.authToken
	}
	token, err := getToken(ctx, p)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://graph.microsoft.com/v1.0/drives/"+url.PathEscape(p.DriveID)+"/"+path, reader)
	if err != nil {
		return errors.New("invalid_graph_request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := r.HTTP.Do(req)
	if err != nil {
		return errors.New("graph_unreachable")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200, 201, 204:
	case 401, 403:
		return errors.New("authentication_required")
	case 404:
		return errors.New("remote_not_found")
	case 409:
		return errors.New("target_conflict")
	case 429:
		return errors.New("throttled")
	case 507:
		return errors.New("quota_exceeded")
	default:
		return errors.New("graph_unavailable")
	}
	if out != nil {
		if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out) != nil {
			return errors.New("invalid_graph_response")
		}
	}
	return nil
}
func (r *OneDrive) Stat(ctx context.Context, p Space, id string) (File, error) {
	i, err := r.item(ctx, p, id, false)
	if err != nil {
		return File{}, err
	}
	if i.File == nil {
		return File{}, errors.New("not_a_file")
	}
	return i.file(p), nil
}
func (r *OneDrive) Upload(ctx context.Context, p Space, name string, in io.Reader, n int64) (File, error) {
	r.mu.Lock()
	f, err := r.client(ctx, p)
	r.mu.Unlock()
	if err != nil {
		return File{}, err
	}
	if _, err = f.NewObject(ctx, name); err == nil {
		return File{}, errors.New("target_conflict")
	} else if !errors.Is(err, fs.ErrorObjectNotFound) {
		return File{}, errors.New("remote_unavailable")
	}
	o, err := f.Put(ctx, in, object.NewStaticObjectInfo(name, time.Now(), n, true, nil, f))
	if err != nil {
		return File{}, classifyOneDriveError(err)
	}
	ider, ok := o.(fs.IDer)
	if !ok {
		return File{}, errors.New("invalid_file_metadata")
	}
	id := ider.ID()
	if pos := strings.LastIndex(id, "#"); pos >= 0 {
		id = id[pos+1:]
	}
	return r.Stat(ctx, p, id)
}
func (r *OneDrive) Access(ctx context.Context, p Space, f File) (string, error) {
	i, err := r.item(ctx, p, f.ItemID, true)
	if err != nil {
		return "", err
	}
	if i.ETag != f.Version || i.Size != f.Size {
		return "", errors.New("file_version_changed")
	}
	if !validDownloadURL(i.Download) {
		return "", errors.New("download_access_unavailable")
	}
	return i.Download, nil
}
func (r *OneDrive) Open(ctx context.Context, p Space, f File) (io.ReadCloser, error) {
	u, err := r.Access(ctx, p, f)
	if err != nil {
		return nil, err
	}
	return openDownload(ctx, u)
}
func validDownloadURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range []string{".1drv.com", ".sharepoint.com", ".storage.live.com", ".onedrive.com"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}
func openDownload(ctx context.Context, raw string) (io.ReadCloser, error) {
	if !validDownloadURL(raw) {
		return nil, errors.New("download_access_unavailable")
	}
	client := &http.Client{Timeout: 30 * time.Minute, Transport: &http.Transport{Proxy: nil, DialContext: publicDial}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !validDownloadURL(req.URL.String()) {
			return errors.New("redirect_not_allowed")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, errors.New("download_access_unavailable")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("download_unavailable")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, errors.New("download_unavailable")
	}
	return resp.Body, nil
}
func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, errors.New("dns_unavailable")
	}
	if len(ips) == 0 {
		return nil, errors.New("dns_unavailable")
	}
	for _, ip := range ips {
		if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
			return nil, errors.New("address_not_allowed")
		}
	}
	return (&net.Dialer{Timeout: 20 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func classifyOneDriveError(err error) error {
	var apiError *onedriveapi.Error
	if errors.As(err, &apiError) {
		switch apiError.ErrorInfo.Code {
		case "quotaLimitReached", "insufficientStorage":
			return errors.New("quota_exceeded")
		case "throttledRequest", "tooManyRequests", "activityLimitReached":
			return errors.New("throttled")
		case "unauthenticated", "invalidAuthenticationToken", "accessDenied":
			return errors.New("authentication_required")
		case "nameAlreadyExists":
			return errors.New("target_conflict")
		}
	}
	return errors.New("transfer_failed")
}

// Move relocates an item by its stable id: parentReference + name via PATCH.
// The destination directory must already exist inside the pinned root.
func (r *OneDrive) Move(ctx context.Context, p Space, f File, destDir, destName string) error {
	if _, err := remoteDir(destDir); err != nil {
		return err
	}
	if !safeName(destName) {
		return errors.New("path_not_allowed")
	}
	destID, err := r.folder(ctx, p, destDir)
	if err != nil {
		return err
	}
	source := f.ItemID
	if f.Trashed != nil && f.TrashPath != "" {
		// Restoring from trash: the item kept its id, no extra lookup needed.
		source = f.ItemID
	}
	var moved driveItem
	body := `{"parentReference":{"id":` + strconv.Quote(destID) + `},"name":` + strconv.Quote(destName) + `}`
	if err = r.graphPatch(ctx, p, "items/"+url.PathEscape(source), body, &moved); err != nil {
		return err
	}
	if moved.ID == "" || moved.Parent.Drive != p.DriveID {
		return errors.New("remote_outside_space")
	}
	return nil
}

// Delete permanently removes the item by its stable id.
func (r *OneDrive) Delete(ctx context.Context, p Space, f File) error {
	return r.graphDelete(ctx, p, "items/"+url.PathEscape(f.ItemID))
}

// DiscoverRoot resolves a user-selected folder using the existing rclone
// identity. It does not create another OAuth credential or change the remote.
func (r *OneDrive) DiscoverRoot(ctx context.Context, remote, folder string) (string, string, error) {
	folder, err := remoteDir(folder)
	if err != nil || folder == "" {
		return "", "", errors.New("folder_path_required")
	}
	drive := config.GetValue(remote, "drive_id")
	if drive == "" {
		return "", "", errors.New("authentication_required")
	}
	p := Space{ID: "discovery-" + remote, Remote: remote, DriveID: drive}
	parts := strings.Split(folder, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	var item driveItem
	if err = r.graph(ctx, p, "root:/"+strings.Join(parts, "/")+"?$select=id,parentReference,folder", &item); err != nil {
		return "", "", err
	}
	if item.ID == "" || item.Folder == nil || (item.Parent.Drive != "" && item.Parent.Drive != drive) {
		return "", "", errors.New("folder_not_found")
	}
	return drive, item.ID, nil
}
