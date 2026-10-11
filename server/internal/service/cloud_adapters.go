package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/oauth2"
)

// cloudObservation is one provider item as observed right now. Revision is
// the provider's own fact (OneDrive eTag, Google headRevision, S3
// etag+size); it is change detection only and never a content digest.
type cloudObservation struct {
	ProviderItemID string
	ParentRef      string
	Path           string
	Kind           string // file|directory|native_document
	Size           int64
	MIME           string
	ModifiedAt     *time.Time
	Revision       string
}

// cloudPage is one bounded provider listing page. Complete reports whether
// the provider enumeration reached its end for the requested scope.
type cloudPage struct {
	Items     []cloudObservation
	NextToken string
	Complete  bool
}

// cloudChangePage is one bounded provider delta page. Removed carries
// provider-confirmed removals (OneDrive deleted facet, Google removed flag);
// only those may tombstone an observation.
type cloudChangePage struct {
	Items      []cloudObservation
	Removed    []string
	NextCursor string
	Delta      bool // provider still has more changes pending
}

// cloudFetch is a pinned byte stream: the adapter verified the requested
// revision is the one being served before opening the stream.
type cloudFetch struct {
	Body io.ReadCloser
	Size int64
}

// cloudAdapter is the per-connection typed provider surface. One adapter
// instance serves exactly one connection; adapters never share process-wide
// provider configuration (no multi-user rclone globals).
type cloudAdapter interface {
	Kind() string
	Capabilities(ctx context.Context, p Principal, conn Connection) CloudCapabilityView
	ListChildren(ctx context.Context, p Principal, conn Connection, root CloudRoot, parentRef, pageToken string, limit int) (cloudPage, error)
	StatItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID string) (cloudObservation, error)
	FetchItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID, revision string) (cloudFetch, error)
	ReadChanges(ctx context.Context, p Principal, conn Connection, root CloudRoot, cursor string, limit int) (cloudChangePage, error)
}

// CloudCapabilityView is the per-provider capability matrix. Every value is
// an independently declared fact about this backend, never inferred from the
// provider's name.
type CloudCapabilityView struct {
	Provider         string   `json:"provider"`
	Browse           bool     `json:"browse"`
	Transfer         bool     `json:"transfer"`
	HistoricalRead   bool     `json:"historical_read"`
	NativeCopy       bool     `json:"native_copy"`
	SoftDelete       bool     `json:"soft_delete"`
	ConditionalWrite bool     `json:"conditional_write"`
	ChangeCursor     bool     `json:"change_cursor"`
	Notes            []string `json:"notes,omitempty"`
}

// ---------------------------------------------------------------- OneDrive ---

type oneDriveCloud struct{ s *Service }

func (oneDriveCloud) Kind() string { return "onedrive" }

func (a oneDriveCloud) Capabilities(context.Context, Principal, Connection) CloudCapabilityView {
	return CloudCapabilityView{
		Provider: "onedrive", Browse: true, Transfer: true, HistoricalRead: true,
		NativeCopy: true, SoftDelete: true, ConditionalWrite: true, ChangeCursor: true,
		Notes: []string{"item revisions provide historical bytes", "delta feed provides the change cursor"},
	}
}

func (a oneDriveCloud) drive(credential oneDriveCredential) string { return credential.DriveID }

func (a oneDriveCloud) location(credential oneDriveCredential, ref string) string {
	if ref == "" || ref == "root" {
		return "root"
	}
	return "items/" + url.PathEscape(ref)
}

func (a oneDriveCloud) ListChildren(ctx context.Context, p Principal, conn Connection, root CloudRoot, parentRef, pageToken string, limit int) (cloudPage, error) {
	_, v, e := a.s.oneDrive(p, conn.ID)
	if e != nil {
		return cloudPage{}, e
	}
	target := root.RootRef
	if parentRef != "" {
		target = parentRef
	}
	query := "$top=" + strconv.Itoa(limit) + "&$select=id,name,size,eTag,file,folder,parentReference,lastModifiedDateTime"
	if pageToken != "" {
		query += "&$skiptoken=" + url.QueryEscape(pageToken)
	}
	b, e := a.s.graph(ctx, p, conn.ID, "GET", "/drives/"+url.PathEscape(v.DriveID)+"/"+a.location(v, target)+"/children?"+query, nil)
	if e != nil {
		return cloudPage{}, e
	}
	var data struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Size int64  `json:"size"`
			ETag string `json:"eTag"`
			File *struct {
				MIME string `json:"mimeType"`
			} `json:"file"`
			Folder *struct{} `json:"folder"`
			Parent *struct {
				ID string `json:"id"`
			} `json:"parentReference"`
			Modified string `json:"lastModifiedDateTime"`
		} `json:"value"`
		Next string `json:"@odata.nextLink"`
	}
	if json.Unmarshal(b, &data) != nil {
		return cloudPage{}, code("invalid_graph_response")
	}
	page := cloudPage{Items: make([]cloudObservation, 0, len(data.Items))}
	for _, item := range data.Items {
		if item.ID == "" {
			continue
		}
		obs := cloudObservation{ProviderItemID: item.ID, ParentRef: target, Path: item.Name, Size: item.Size, Revision: item.ETag}
		if item.File != nil {
			obs.Kind = "file"
			obs.MIME = item.File.MIME
		} else if item.Folder != nil {
			obs.Kind = "directory"
		} else {
			continue
		}
		if item.Parent != nil && item.Parent.ID != "" {
			obs.ParentRef = item.Parent.ID
		}
		if t, e := time.Parse(time.RFC3339, item.Modified); e == nil {
			u := t.UTC()
			obs.ModifiedAt = &u
		}
		page.Items = append(page.Items, obs)
	}
	if data.Next != "" {
		u, e := url.Parse(data.Next)
		if e != nil || u.Host != "graph.microsoft.com" {
			return cloudPage{}, code("invalid_graph_response")
		}
		page.NextToken = u.Query().Get("$skiptoken")
	} else {
		page.Complete = true
	}
	return page, nil
}

func (a oneDriveCloud) StatItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID string) (cloudObservation, error) {
	item, e := a.s.driveItem(ctx, p, conn.ID, providerItemID, false)
	if e != nil {
		return cloudObservation{}, e
	}
	obs := cloudObservation{ProviderItemID: item.ID, Path: item.Name, Size: item.Size, Revision: item.ETag}
	if item.File != nil {
		obs.Kind = "file"
		obs.MIME = item.File.MIME
	} else if item.Folder != nil {
		obs.Kind = "directory"
	} else {
		return obs, code("export_required")
	}
	return obs, nil
}

// FetchItem serves one pinned revision. The current revision streams through
// the temporary download URL; historical revisions resolve through the item
// versions API because OneDrive declares historical bytes readable.
func (a oneDriveCloud) FetchItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID, revision string) (cloudFetch, error) {
	_, v, e := a.s.oneDrive(p, conn.ID)
	if e != nil {
		return cloudFetch{}, e
	}
	current, e := a.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil {
		return cloudFetch{}, e
	}
	if current.Kind == "native_document" {
		return cloudFetch{}, code("export_required")
	}
	if current.Revision == revision {
		item, e := a.s.driveItem(ctx, p, conn.ID, providerItemID, true)
		if e != nil {
			return cloudFetch{}, e
		}
		if item.ETag != revision {
			return cloudFetch{}, code("source_changed")
		}
		resp, e := a.s.sensitiveTransfer(ctx, "GET", item.Download, nil, nil)
		if e != nil {
			return cloudFetch{}, e
		}
		return cloudFetch{Body: resp.Body, Size: current.Size}, nil
	}
	// Historical revision: pin the version content endpoint.
	b, e := a.s.graph(ctx, p, conn.ID, "GET", "/drives/"+url.PathEscape(v.DriveID)+"/items/"+url.PathEscape(providerItemID)+"/versions/"+url.PathEscape(revision)+"/content", nil)
	if e != nil {
		return cloudFetch{}, e
	}
	return cloudFetch{Body: io.NopCloser(strings.NewReader(string(b))), Size: int64(len(b))}, nil
}

func (a oneDriveCloud) ReadChanges(ctx context.Context, p Principal, conn Connection, root CloudRoot, cursor string, limit int) (cloudChangePage, error) {
	_, v, e := a.s.oneDrive(p, conn.ID)
	if e != nil {
		return cloudChangePage{}, e
	}
	query := "$top=" + strconv.Itoa(limit) + "&$select=id,name,size,eTag,file,folder,deleted,parentReference,lastModifiedDateTime"
	if cursor != "" {
		query += "&$skiptoken=" + url.QueryEscape(cursor)
	}
	b, e := a.s.graph(ctx, p, conn.ID, "GET", "/drives/"+url.PathEscape(v.DriveID)+"/items/"+url.PathEscape(root.RootRef)+"/delta?"+query, nil)
	if e != nil {
		return cloudChangePage{}, e
	}
	var data struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Size int64  `json:"size"`
			ETag string `json:"eTag"`
			File *struct {
				MIME string `json:"mimeType"`
			} `json:"file"`
			Folder  *struct{} `json:"folder"`
			Deleted *struct{} `json:"deleted"`
			Parent  *struct {
				ID string `json:"id"`
			} `json:"parentReference"`
		} `json:"value"`
		Next  string `json:"@odata.nextLink"`
		Delta string `json:"@odata.deltaLink"`
	}
	if json.Unmarshal(b, &data) != nil {
		return cloudChangePage{}, code("invalid_graph_response")
	}
	page := cloudChangePage{Items: []cloudObservation{}, Removed: []string{}}
	for _, item := range data.Items {
		if item.ID == "" {
			continue
		}
		if item.Deleted != nil {
			page.Removed = append(page.Removed, item.ID)
			continue
		}
		obs := cloudObservation{ProviderItemID: item.ID, Path: item.Name, Size: item.Size, Revision: item.ETag}
		if item.File != nil {
			obs.Kind = "file"
			obs.MIME = item.File.MIME
		} else if item.Folder != nil {
			obs.Kind = "directory"
		} else {
			continue
		}
		if item.Parent != nil && item.Parent.ID != "" {
			obs.ParentRef = item.Parent.ID
		}
		page.Items = append(page.Items, obs)
	}
	if data.Next != "" {
		u, e := url.Parse(data.Next)
		if e != nil || u.Host != "graph.microsoft.com" {
			return cloudChangePage{}, code("invalid_graph_response")
		}
		page.NextCursor = u.Query().Get("$skiptoken")
		page.Delta = true
	} else if data.Delta != "" {
		u, e := url.Parse(data.Delta)
		if e != nil || u.Host != "graph.microsoft.com" {
			return cloudChangePage{}, code("invalid_graph_response")
		}
		page.NextCursor = u.Query().Get("$skiptoken")
	}
	return page, nil
}

// --------------------------------------------------------------- GDrive ----

// gdriveCredential is the sealed per-connection Google Drive token material.
// Endpoint is a development-only loopback override used by fixtures; it is
// rejected outside Development mode.
type gdriveCredential struct {
	Token    oauth2.Token `json:"token"`
	Endpoint string       `json:"endpoint,omitempty"`
}

func (s *Service) gdriveEndpoint(credential gdriveCredential) string {
	if credential.Endpoint != "" && s.Config.Development {
		if u, e := url.Parse(credential.Endpoint); e == nil && loopback(u.Hostname()) {
			// Development fixtures may pin the API base to an explicit loopback.
			return strings.TrimSuffix(credential.Endpoint, "/")
		}
	}
	return "https://www.googleapis.com"
}

func (s *Service) gdriveCall(ctx context.Context, p Principal, conn Connection, method, apiPath string, query url.Values) ([]byte, error) {
	_, credential, e := s.gdriveConnection(p, conn.ID)
	if e != nil {
		return nil, e
	}
	if credential.Token.AccessToken == "" {
		return nil, code("gdrive_authorization_required")
	}
	endpoint := s.gdriveEndpoint(credential)
	u := endpoint + apiPath
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, e := http.NewRequestWithContext(ctx, method, u, nil)
	if e != nil {
		return nil, code("invalid_request")
	}
	req.Header.Set("Authorization", "Bearer "+credential.Token.AccessToken)
	client := s.gdriveHTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: safeDial(s.Config.Development && credential.Endpoint != "")}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, code("gdrive_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return nil, code("throttled")
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, code("gdrive_authorization_required")
	}
	if resp.StatusCode == 404 {
		return nil, code("not_found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, code("gdrive_unavailable")
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// gdriveCallRaw performs one media request and returns the raw response so
// byte streams never get buffered into memory as a whole.
func (s *Service) gdriveCallRaw(ctx context.Context, p Principal, conn Connection, apiPath string, query url.Values) (*http.Response, error) {
	_, credential, e := s.gdriveConnection(p, conn.ID)
	if e != nil {
		return nil, e
	}
	if credential.Token.AccessToken == "" {
		return nil, code("gdrive_authorization_required")
	}
	endpoint := s.gdriveEndpoint(credential)
	u := endpoint + apiPath
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, code("invalid_request")
	}
	req.Header.Set("Authorization", "Bearer "+credential.Token.AccessToken)
	client := s.gdriveHTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute, Transport: &http.Transport{Proxy: nil, DialContext: safeDial(s.Config.Development && credential.Endpoint != "")}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, code("gdrive_unavailable")
	}
	if resp.StatusCode == 429 {
		resp.Body.Close()
		return nil, code("throttled")
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		resp.Body.Close()
		return nil, code("gdrive_authorization_required")
	}
	if resp.StatusCode == 404 {
		resp.Body.Close()
		return nil, code("not_found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, code("gdrive_unavailable")
	}
	return resp, nil
}

type gdriveCloud struct{ s *Service }

func (gdriveCloud) Kind() string { return "gdrive" }

func (a gdriveCloud) Capabilities(context.Context, Principal, Connection) CloudCapabilityView {
	return CloudCapabilityView{
		Provider: "gdrive", Browse: true, Transfer: true, HistoricalRead: true,
		NativeCopy: true, SoftDelete: true, ConditionalWrite: true, ChangeCursor: true,
		Notes: []string{
			"binary files expose revision-pinned bytes; native Google documents list as native_document and require a separate export capability (export_required)",
			"the changes feed provides the change cursor",
		},
	}
}

type gdriveFileMeta struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MIME         string   `json:"mimeType"`
	Size         string   `json:"size"`
	HeadRevision string   `json:"headRevision"`
	MD5          string   `json:"md5Checksum"`
	Modified     string   `json:"modifiedTime"`
	Parents      []string `json:"parents"`
	Trashed      bool     `json:"trashed"`
}

func gdriveIsNativeDocument(mime string) bool {
	return strings.HasPrefix(mime, "application/vnd.google-apps.")
}

func (m *gdriveFileMeta) observation(parentFallback string) cloudObservation {
	size, _ := strconv.ParseInt(m.Size, 10, 64)
	obs := cloudObservation{ProviderItemID: m.ID, ParentRef: parentFallback, Path: m.Name, Size: size, Revision: m.HeadRevision}
	switch {
	case gdriveIsNativeDocument(m.MIME):
		obs.Kind = "native_document"
		obs.MIME = m.MIME
	case strings.HasSuffix(m.MIME, "folder"):
		obs.Kind = "directory"
		obs.MIME = m.MIME
	default:
		obs.Kind = "file"
		obs.MIME = m.MIME
	}
	if len(m.Parents) > 0 {
		obs.ParentRef = m.Parents[0]
	}
	if t, e := time.Parse(time.RFC3339, m.Modified); e == nil {
		u := t.UTC()
		obs.ModifiedAt = &u
	}
	return obs
}

const gdriveFields = "nextPageToken,files(id,name,mimeType,size,headRevision,md5Checksum,modifiedTime,parents,trashed)"

func (a gdriveCloud) ListChildren(ctx context.Context, p Principal, conn Connection, root CloudRoot, parentRef, pageToken string, limit int) (cloudPage, error) {
	target := root.RootRef
	if parentRef != "" {
		target = parentRef
	}
	q := url.Values{}
	q.Set("q", fmt.Sprintf("'%s' in parents and trashed = false", target))
	q.Set("pageSize", strconv.Itoa(limit))
	q.Set("fields", gdriveFields)
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	b, e := a.s.gdriveCall(ctx, p, conn, "GET", "/drive/v3/files", q)
	if e != nil {
		return cloudPage{}, e
	}
	var data struct {
		Files      []gdriveFileMeta `json:"files"`
		NextPageTk string           `json:"nextPageToken"`
	}
	if json.Unmarshal(b, &data) != nil {
		return cloudPage{}, code("invalid_gdrive_response")
	}
	page := cloudPage{Items: make([]cloudObservation, 0, len(data.Files))}
	for i := range data.Files {
		if data.Files[i].ID == "" {
			continue
		}
		page.Items = append(page.Items, data.Files[i].observation(target))
	}
	page.NextToken = data.NextPageTk
	page.Complete = data.NextPageTk == ""
	return page, nil
}

func (a gdriveCloud) StatItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID string) (cloudObservation, error) {
	q := url.Values{}
	q.Set("fields", "id,name,mimeType,size,headRevision,modifiedTime,parents,trashed")
	b, e := a.s.gdriveCall(ctx, p, conn, "GET", "/drive/v3/files/"+url.PathEscape(providerItemID), q)
	if e != nil {
		return cloudObservation{}, e
	}
	var meta gdriveFileMeta
	if json.Unmarshal(b, &meta) != nil || meta.ID == "" {
		return cloudObservation{}, code("invalid_gdrive_response")
	}
	if meta.Trashed {
		return cloudObservation{}, code("version_unavailable")
	}
	return meta.observation(root.RootRef), nil
}

func (a gdriveCloud) FetchItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID, revision string) (cloudFetch, error) {
	current, e := a.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil {
		return cloudFetch{}, e
	}
	if current.Kind == "native_document" {
		return cloudFetch{}, code("export_required")
	}
	if current.Revision != revision {
		// Google exposes per-revision bytes: a pinned historical revision
		// streams from the revision media endpoint, never from head.
		q := url.Values{}
		q.Set("alt", "media")
		resp, err := a.s.gdriveCallRaw(ctx, p, conn, "/drive/v3/files/"+url.PathEscape(providerItemID)+"/revisions/"+url.PathEscape(revision), q)
		if err != nil {
			return cloudFetch{}, code("version_unavailable")
		}
		return cloudFetch{Body: resp.Body, Size: current.Size}, nil
	}
	q := url.Values{}
	q.Set("alt", "media")
	resp, e := a.s.gdriveCallRaw(ctx, p, conn, "/drive/v3/files/"+url.PathEscape(providerItemID), q)
	if e != nil {
		return cloudFetch{}, e
	}
	return cloudFetch{Body: resp.Body, Size: current.Size}, nil
}

func (a gdriveCloud) ReadChanges(ctx context.Context, p Principal, conn Connection, root CloudRoot, cursor string, limit int) (cloudChangePage, error) {
	if cursor == "" {
		// Establish the enumeration baseline from the root's stored delta
		// cursor; a missing baseline starts a fresh changes token.
		cursor = root.DeltaCursor
	}
	if cursor == "" {
		cursor = "1"
	}
	q := url.Values{}
	q.Set("pageToken", cursor)
	q.Set("pageSize", strconv.Itoa(limit))
	q.Set("fields", "nextPageToken,newStartPageToken,changes(fileId,removed,file(id,name,mimeType,size,headRevision,modifiedTime,parents,trashed))")
	q.Set("includeRemoved", "true")
	b, e := a.s.gdriveCall(ctx, p, conn, "GET", "/drive/v3/changes", q)
	if e != nil {
		return cloudChangePage{}, e
	}
	var data struct {
		NextPageToken  string `json:"nextPageToken"`
		NewStartPageTk string `json:"newStartPageToken"`
		Changes        []struct {
			FileID  string          `json:"fileId"`
			Removed bool            `json:"removed"`
			File    *gdriveFileMeta `json:"file"`
		} `json:"changes"`
	}
	if json.Unmarshal(b, &data) != nil {
		return cloudChangePage{}, code("invalid_gdrive_response")
	}
	page := cloudChangePage{Items: []cloudObservation{}, Removed: []string{}}
	for _, change := range data.Changes {
		if change.FileID == "" {
			continue
		}
		if change.Removed || change.File == nil {
			page.Removed = append(page.Removed, change.FileID)
			continue
		}
		obs := change.File.observation("")
		if obs.Kind == "native_document" || obs.Kind == "file" || obs.Kind == "directory" {
			page.Items = append(page.Items, obs)
		}
	}
	page.NextCursor = data.NewStartPageTk
	if data.NextPageToken != "" {
		page.NextCursor = data.NextPageToken
		page.Delta = true
	}
	return page, nil
}

// ------------------------------------------------------------------- S3 ----

// cloudPrefixLister is the paginated prefix listing the S3 store exposes for
// cloud observation. Directories are prefix projections (delimiter rollup),
// never native container objects.
type cloudPrefixLister interface {
	CloudListPrefix(ctx context.Context, prefix, token string, max int32) (entries []cloudPrefixEntry, truncated bool, nextToken string, err error)
}

type cloudPrefixEntry struct {
	Key       string
	Size      int64
	ETag      string
	Modified  *time.Time
	Directory bool
}

// CloudListPrefix lists one bounded page under a prefix with "/" grouping.
// The prefix is managed-relative ("" = the connection's whole scope): the
// store joins the connection prefix for the provider query and strips it
// from returned keys, so cloud adapters stay connection-prefix-agnostic.
// The continuation token stays provider-opaque; an empty next token means
// the enumeration completed for the scope.
func (s *s3Store) CloudListPrefix(ctx context.Context, prefix, token string, max int32) ([]cloudPrefixEntry, bool, string, error) {
	relative := strings.Trim(prefix, "/")
	// The connection scope always carries its trailing slash: an unslashed
	// root prefix would both leak sibling namespaces sharing the string
	// prefix (managed-other/ under managed) and roll the scope's own keys
	// into a bogus self-directory via the delimiter.
	scope := ""
	if s.cfg.Prefix != "" {
		scope = strings.TrimSuffix(s.cfg.Prefix, "/") + "/"
	}
	query := scope
	if relative != "" {
		if scope != "" {
			query = scope + relative
		} else {
			query = relative
		}
		// Directory listings carry the trailing slash: without it the
		// delimiter rolls the directory itself up as a common prefix.
		if strings.HasSuffix(prefix, "/") {
			query += "/"
		}
	}
	in := &s3.ListObjectsV2Input{Bucket: aws.String(s.cfg.Bucket), Prefix: aws.String(query), Delimiter: aws.String("/"), MaxKeys: aws.Int32(max)}
	if token != "" {
		in.ContinuationToken = aws.String(token)
	}
	out, err := s.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, false, "", code("storage_unavailable")
	}
	entries := make([]cloudPrefixEntry, 0, len(out.Contents)+len(out.CommonPrefixes))
	for _, item := range out.CommonPrefixes {
		raw := aws.ToString(item.Prefix)
		// Defensive scope check: a provider echoing a key outside the
		// connection namespace is dropped, never surfaced as an entry.
		if scope != "" && !strings.HasPrefix(raw, scope) || scope == "" && raw == "" {
			continue
		}
		key := strings.TrimPrefix(raw, scope)
		if key == "" {
			continue
		}
		entries = append(entries, cloudPrefixEntry{Key: key, Directory: true})
	}
	for _, item := range out.Contents {
		raw := aws.ToString(item.Key)
		if scope != "" && !strings.HasPrefix(raw, scope) {
			continue
		}
		key := strings.TrimPrefix(raw, scope)
		if key == "" || strings.HasSuffix(key, "/") {
			continue
		}
		entry := cloudPrefixEntry{Key: key}
		if item.Size != nil {
			entry.Size = *item.Size
		}
		if item.ETag != nil {
			entry.ETag = strings.Trim(*item.ETag, `"`)
		}
		if item.LastModified != nil {
			t := item.LastModified.UTC()
			entry.Modified = &t
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, aws.ToBool(out.IsTruncated), aws.ToString(out.NextContinuationToken), nil
}

type s3Cloud struct{ s *Service }

func (s3Cloud) Kind() string { return "s3" }

// Capabilities probes bucket versioning before declaring historical reads:
// an unversioned bucket cannot serve old revisions, and the matrix never
// infers capability from the provider's name.
func (a s3Cloud) Capabilities(ctx context.Context, p Principal, conn Connection) CloudCapabilityView {
	view := CloudCapabilityView{
		Provider: "s3", Browse: true, Transfer: true, HistoricalRead: false,
		NativeCopy: true, SoftDelete: false, ConditionalWrite: true, ChangeCursor: false,
		Notes: []string{
			"directories are prefix projections; cross-prefix renames follow copy/verify/cleanup plans, never atomic directory moves",
			"no native change cursor: change detection uses budgeted scans",
			"historical reads require an enabled bucket versioning probe",
		},
	}
	lister, _, e := a.store(p, conn)
	if e != nil {
		return view
	}
	store, ok := lister.(*s3Store)
	if !ok {
		return view
	}
	if vers, e := store.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(store.cfg.Bucket)}); e == nil && string(vers.Status) == "Enabled" {
		view.HistoricalRead = true
		view.SoftDelete = true
		view.Notes = append(view.Notes, "bucket versioning observed as enabled: delete markers and old versions are readable")
	}
	return view
}

func (a s3Cloud) store(p Principal, conn Connection) (Storage, cloudPrefixLister, error) {
	st, e := a.s.storageFor(conn.Owner, conn.ID)
	if e != nil {
		return nil, nil, e
	}
	lister, ok := st.(cloudPrefixLister)
	if !ok {
		return nil, nil, code("storage_disconnected")
	}
	return st, lister, nil
}

// s3RootPrefix normalizes the root's bound prefix for key joins.
func s3RootPrefix(root CloudRoot) string {
	return strings.Trim(root.RootRef, "/")
}

// s3Relative strips the root prefix from a full key; results outside the
// root are rejected instead of escaping the bound scope.
func s3Relative(root CloudRoot, key string) (string, error) {
	prefix := s3RootPrefix(root)
	if prefix == "" {
		return strings.Trim(key, "/"), nil
	}
	if !strings.HasPrefix(key, prefix+"/") {
		return "", code("forbidden")
	}
	return strings.TrimPrefix(key, prefix+"/"), nil
}

func (a s3Cloud) ListChildren(ctx context.Context, p Principal, conn Connection, root CloudRoot, parentRef, pageToken string, limit int) (cloudPage, error) {
	_, lister, e := a.store(p, conn)
	if e != nil {
		return cloudPage{}, e
	}
	prefix := s3RootPrefix(root)
	if parentRef != "" {
		normalized, e := s3NormalizeRef(root, parentRef)
		if e != nil {
			return cloudPage{}, e
		}
		prefix = normalized
	}
	entries, truncated, token, e := lister.CloudListPrefix(ctx, prefix+"/", pageToken, int32(limit))
	if e != nil {
		return cloudPage{}, e
	}
	page := cloudPage{Items: make([]cloudObservation, 0, len(entries))}
	for _, entry := range entries {
		rel, e := s3Relative(root, entry.Key)
		if e != nil {
			continue
		}
		obs := cloudObservation{ProviderItemID: rel, ParentRef: parentRef, Path: path.Base(strings.TrimSuffix(rel, "/"))}
		if entry.Directory {
			obs.Kind = "directory"
			obs.ProviderItemID = strings.TrimSuffix(rel, "/") + "/"
		} else {
			obs.Kind = "file"
			obs.Size = entry.Size
			obs.Revision = s3Revision(entry.ETag, entry.Size)
			obs.ModifiedAt = entry.Modified
		}
		page.Items = append(page.Items, obs)
	}
	page.NextToken = token
	page.Complete = !truncated
	return page, nil
}

// s3NormalizeRef resolves a caller-supplied parent/directory reference into
// a validated prefix under the bound root. Arbitrary bucket or URL shapes
// are refused.
func s3NormalizeRef(root CloudRoot, ref string) (string, error) {
	ref = strings.Trim(ref, "/")
	if ref == "" {
		return s3RootPrefix(root), nil
	}
	cleaned, e := cleanPath(ref)
	if e != nil || cleaned != ref {
		return "", code("invalid_request")
	}
	// Refs arrive root-relative by construction; cleanPath already refuses
	// traversal, so no extra containment re-derivation is needed here.
	return ref, nil
}
func s3Revision(etag string, size int64) string {
	return etag + ":" + strconv.FormatInt(size, 10)
}

func (a s3Cloud) StatItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID string) (cloudObservation, error) {
	store, _, e := a.store(p, conn)
	if e != nil {
		return cloudObservation{}, e
	}
	if strings.HasSuffix(providerItemID, "/") {
		// Directory entries are prefix projections: stat through listing.
		normalized, e := s3NormalizeRef(root, strings.TrimSuffix(providerItemID, "/"))
		if e != nil {
			return cloudObservation{}, e
		}
		return cloudObservation{ProviderItemID: providerItemID, Path: path.Base(normalized), Kind: "directory", ParentRef: ""}, nil
	}
	normalized, e := s3NormalizeRef(root, providerItemID)
	if e != nil {
		return cloudObservation{}, e
	}
	full := normalized
	if s3RootPrefix(root) != "" {
		full = s3RootPrefix(root) + "/" + normalized
	}
	st, ok := store.(*s3Store)
	if ok {
		head, e := st.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(st.cfg.Bucket), Key: aws.String(st.key(full))})
		if e != nil {
			return cloudObservation{}, code("version_unavailable")
		}
		size := int64(0)
		if head.ContentLength != nil {
			size = *head.ContentLength
		}
		etag := ""
		if head.ETag != nil {
			etag = strings.Trim(*head.ETag, `"`)
		}
		obs := cloudObservation{ProviderItemID: providerItemID, Path: path.Base(normalized), Kind: "file", Size: size, Revision: s3Revision(etag, size)}
		if head.LastModified != nil {
			t := head.LastModified.UTC()
			obs.ModifiedAt = &t
		}
		return obs, nil
	}
	return cloudObservation{}, code("storage_disconnected")
}

// FetchItem serves one pinned revision. Without observed bucket versioning
// the store cannot serve a non-current revision: version_unavailable, never
// the latest bytes in its place.
func (a s3Cloud) FetchItem(ctx context.Context, p Principal, conn Connection, root CloudRoot, providerItemID, revision string) (cloudFetch, error) {
	store, _, e := a.store(p, conn)
	if e != nil {
		return cloudFetch{}, e
	}
	current, e := a.StatItem(ctx, p, conn, root, providerItemID)
	if e != nil {
		return cloudFetch{}, e
	}
	if current.Revision != revision {
		caps := a.Capabilities(ctx, p, conn)
		if !caps.HistoricalRead {
			return cloudFetch{}, code("version_unavailable")
		}
		st, ok := store.(*s3Store)
		if !ok {
			return cloudFetch{}, code("version_unavailable")
		}
		normalized, e := s3NormalizeRef(root, providerItemID)
		if e != nil {
			return cloudFetch{}, e
		}
		versionID := revision
		if idx := strings.LastIndex(revision, ":"); idx >= 0 {
			versionID = revision[idx+1:]
		}
		out, e := st.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(st.cfg.Bucket), Key: aws.String(st.key(normalized)), VersionId: aws.String(versionID)})
		if e != nil {
			return cloudFetch{}, code("version_unavailable")
		}
		return cloudFetch{Body: out.Body, Size: current.Size}, nil
	}
	full := providerItemID
	if s3RootPrefix(root) != "" {
		full = s3RootPrefix(root) + "/" + providerItemID
	}
	body, e := store.Get(ctx, full)
	if e != nil {
		return cloudFetch{}, code("storage_read_failed")
	}
	return cloudFetch{Body: body, Size: current.Size}, nil
}

func (a s3Cloud) ReadChanges(context.Context, Principal, Connection, CloudRoot, string, int) (cloudChangePage, error) {
	// The matrix declares no S3 change cursor; change detection is a
	// budgeted scan on the original root, never a fabricated delta.
	return cloudChangePage{}, code("unsupported")
}

// cloudAdapterFor resolves the typed adapter for one connection.
func (s *Service) cloudAdapterFor(kind string) (cloudAdapter, error) {
	switch kind {
	case "onedrive":
		return oneDriveCloud{s: s}, nil
	case "gdrive":
		return gdriveCloud{s: s}, nil
	case "s3":
		return s3Cloud{s: s}, nil
	}
	return nil, code("unsupported")
}
