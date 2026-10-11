// Package sdk is the shared pre-1.0 HTTP client. It never logs bearer credentials
// or private response bodies and never follows authenticated redirects.
package sdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	access *aigoraAccess
	tenant string
	space  string
	base   string
	token  string
	http   *http.Client
}

// Error is the typed service failure. Error() keeps returning the bare code
// so existing string comparisons stay byte-compatible; callers that want the
// human-readable remediation read Message (empty when the server omitted it).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code }

type File struct {
	Caller    string `json:"caller,omitempty"`
	TaskRef   string `json:"task_ref,omitempty"`
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	Path      string `json:"path"`
	VersionID string `json:"version_id"`
	Size      int64  `json:"size_bytes"`
	MIME      string `json:"mime_type"`
	Trashed   bool   `json:"trashed"`
}
type Operation struct {
	Caller        string `json:"caller,omitempty"`
	TaskRef       string `json:"task_ref,omitempty"`
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	FileID        string `json:"file_id"`
	VersionID     string `json:"version_id"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size_bytes"`
	Transferred   int64  `json:"bytes_transferred"`
	BlobReused    bool   `json:"blob_reused,omitempty"`
	Error         string `json:"error"`
	SourceID      string `json:"source_id,omitempty"`
	SourceVersion string `json:"source_version,omitempty"`
}
type UploadRequest struct {
	Caller          string `json:"caller,omitempty"`
	TaskRef         string `json:"task_ref,omitempty"`
	SpaceID         string `json:"space_id"`
	Path            string `json:"path"`
	Size            int64  `json:"size_bytes"`
	SHA256          string `json:"sha256"`
	FileID          string `json:"file_id,omitempty"`
	ExpectedVersion string `json:"expected_version,omitempty"`
	// BatchID/ItemKey schedule this upload as one sealed batch item; both
	// join the server-side idempotency fingerprint as business inputs.
	BatchID string `json:"batch_id,omitempty"`
	ItemKey string `json:"item_key,omitempty"`
}
type OperationState struct {
	Operation Operation `json:"operation"`
	Received  []int     `json:"received_parts"`
}
type ArchiveRequest struct {
	SourceFileID    string `json:"source_file_id"`
	SourceVersionID string `json:"source_version_id"`
	TargetSpaceID   string `json:"target_space_id"`
	TargetPath      string `json:"target_path"`
}

// UserImport is one batch entry for provisioning: an operator-supplied key
// (imported as-is) or an empty key (generated server-side).
type UserImport struct {
	Name  string `json:"name"`
	Key   string `json:"key,omitempty"`
	Days  int    `json:"days,omitempty"`
	Admin bool   `json:"admin,omitempty"`
}

// UserKeyIssued is the one-time issuance record; plaintext never persists.
type UserKeyIssued struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	Admin     bool   `json:"admin"`
	ExpiresAt string `json:"expires_at"`
}

// UserKey is the lifecycle view of a provisioned key; no hashes.
type UserKey struct {
	ID      string `json:"id"`
	User    string `json:"user"`
	Name    string `json:"name"`
	Admin   bool   `json:"admin"`
	Created string `json:"created_at"`
	Expires string `json:"expires_at"`
	Revoked bool   `json:"revoked"`
}

func New(base, token string) (*Client, error) {
	base = strings.TrimSuffix(base, "/")
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, errors.New("invalid_service_url")
	}
	return &Client{base: base, token: token, http: &http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method, path, key string, body io.Reader) (*http.Response, error) {
	if !strings.HasPrefix(path, "/api/v1/") {
		return nil, errors.New("invalid_api_path")
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if e != nil {
		return nil, errors.New("invalid_request")
	}
	token := c.token
	if c.access != nil {
		token, e = c.access.credential(ctx, method)
		if e != nil {
			return nil, e
		}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if c.tenant != "" {
		req.Header.Set("X-DriveBridge-Tenant", c.tenant)
	}
	if c.space != "" {
		req.Header.Set("X-DriveBridge-Space", c.space)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, errors.New("service_unavailable")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		var out struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out)
		if safe(out.Error.Code) {
			return nil, &Error{Code: out.Error.Code, Message: out.Error.Message}
		}
		return nil, errors.New("service_request_failed")
	}
	return resp, nil
}
func safe(s string) bool {
	if s == "" || len(s) > 80 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return false
		}
	}
	return true
}
func (c *Client) JSON(ctx context.Context, method, path, key string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return errors.New("invalid_request")
		}
		r = bytes.NewReader(b)
	}
	resp, e := c.request(ctx, method, path, key, r)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&env) != nil {
		return errors.New("invalid_response")
	}
	if out != nil && json.Unmarshal(env.Data, out) != nil {
		return errors.New("invalid_response")
	}
	return nil
}
func (c *Client) Operation(ctx context.Context, id string) (OperationState, error) {
	var v OperationState
	e := c.JSON(ctx, "GET", "/api/v1/operations/"+url.PathEscape(id), "", nil, &v)
	return v, e
}
func (c *Client) Inbox(ctx context.Context, status, caller, spaceID string) ([]Operation, error) {
	var out []Operation
	path := "/api/v1/inbox"
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if caller != "" {
		q.Set("caller", caller)
	}
	if spaceID != "" {
		q.Set("space_id", spaceID)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return out, c.JSON(ctx, "GET", path, "", nil, &out)
}
func (c *Client) Archive(ctx context.Context, q ArchiveRequest, key string) (Operation, error) {
	var out Operation
	e := c.JSON(ctx, "POST", "/api/v1/archive", key, q, &out)
	return out, e
}

type CopyRequest struct {
	TargetSpaceID   string `json:"target_space_id"`
	TargetPath      string `json:"target_path"`
	ExpectedVersion string `json:"expected_version"`
	Conflict        string `json:"conflict,omitempty"`
}

func (c *Client) Copy(ctx context.Context, fileID string, q CopyRequest, key string) (Operation, error) {
	var out Operation
	e := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/copy", key, q, &out)
	return out, e
}

type DirectoryCopyRequest struct {
	SpaceID       string `json:"space_id"`
	Path          string `json:"path"`
	TargetSpaceID string `json:"target_space_id"`
	TargetPath    string `json:"target_path"`
	Conflict      string `json:"conflict,omitempty"`
}

func (c *Client) CopyDirectory(ctx context.Context, q DirectoryCopyRequest, key string) (Operation, error) {
	var out Operation
	e := c.JSON(ctx, "POST", "/api/v1/directories/copy", key, q, &out)
	return out, e
}

// UsersCreate batch-provisions key users; the caller's token must be an
// admin user key. Plaintext keys appear exactly once in the response.
func (c *Client) UsersCreate(ctx context.Context, entries []UserImport) ([]UserKeyIssued, error) {
	var out struct {
		Issued []UserKeyIssued `json:"issued"`
	}
	e := c.JSON(ctx, "POST", "/api/v1/users", "", map[string]any{"users": entries}, &out)
	return out.Issued, e
}

// UsersList reports user-key lifecycle metadata only; never hashes.
func (c *Client) UsersList(ctx context.Context) ([]UserKey, error) {
	var out struct {
		Keys []UserKey `json:"keys"`
	}
	e := c.JSON(ctx, "GET", "/api/v1/users", "", nil, &out)
	return out.Keys, e
}

// UsersRevoke flips the revocation switch on one user key by id.
func (c *Client) UsersRevoke(ctx context.Context, keyID string) error {
	var out map[string]any
	return c.JSON(ctx, "POST", "/api/v1/users/"+url.PathEscape(keyID)+"/revoke", "", nil, &out)
}
func (c *Client) Upload(ctx context.Context, q UploadRequest, key string, source io.ReadSeeker, progress func(Operation)) (Operation, error) {
	if _, e := source.Seek(0, 0); e != nil {
		return Operation{}, e
	}
	h := sha256.New()
	n, e := io.Copy(h, source)
	if e != nil {
		return Operation{}, errors.New("local_read_failed")
	}
	q.Size = n
	q.SHA256 = hex.EncodeToString(h.Sum(nil))
	var op Operation
	if e = c.JSON(ctx, "POST", "/api/v1/uploads", key, q, &op); e != nil {
		return op, e
	}
	if op.Status == "completed" {
		return op, nil
	}
	state, e := c.Operation(ctx, op.ID)
	if e != nil {
		return op, e
	}
	present := map[int]bool{}
	for _, v := range state.Received {
		present[v] = true
	}
	const chunk int64 = 8 << 20
	for index := 0; int64(index)*chunk < n; index++ {
		if present[index] {
			continue
		}
		if _, e = source.Seek(int64(index)*chunk, 0); e != nil {
			return op, e
		}
		part := make([]byte, int(min(chunk, n-int64(index)*chunk)))
		if _, e = io.ReadFull(source, part); e != nil {
			return op, errors.New("local_read_failed")
		}
		resp, e := c.request(ctx, "PUT", "/api/v1/operations/"+url.PathEscape(op.ID)+"/parts/"+strconv.Itoa(index), "", bytes.NewReader(part))
		if e != nil {
			return op, e
		}
		var env struct {
			Data Operation `json:"data"`
		}
		e = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&env)
		resp.Body.Close()
		if e != nil {
			return op, errors.New("invalid_response")
		}
		op = env.Data
		if progress != nil {
			progress(op)
		}
	}
	e = c.JSON(ctx, "POST", "/api/v1/operations/"+url.PathEscape(op.ID)+"/complete", "", nil, &op)
	return op, e
}
func (c *Client) Download(ctx context.Context, id, version string, dest io.Writer) error {
	p := "/api/v1/files/" + url.PathEscape(id) + "/content"
	if version != "" {
		p += "?version=" + url.QueryEscape(version)
	}
	resp, e := c.request(ctx, "GET", p, "", nil)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(dest, h), resp.Body)
	if e != nil {
		return errors.New("download_interrupted")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return errors.New("download_interrupted")
	}
	if want := resp.Header.Get("X-Content-SHA256"); want == "" || hex.EncodeToString(h.Sum(nil)) != want {
		return errors.New("content_mismatch")
	}
	return nil
}

// Access is intentionally non-serializable: bearer URLs belong only in private
// application-to-application handoff, never model-visible tool output.
type Access struct {
	URL       string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	VersionID string    `json:"version_id"`
}

func (c *Client) Acquire(ctx context.Context, fileID, versionID string) (Access, error) {
	var wire struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
		VersionID string    `json:"version_id"`
	}
	e := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/access", "", map[string]string{"version_id": versionID}, &wire)
	return Access{wire.URL, wire.ExpiresAt, wire.VersionID}, e
}

// Scoped returns an independent client targeting a tenant and optional space.
// The server validates membership; context never replaces the authenticated actor.
func (c *Client) Scoped(tenant, space string) *Client {
	next := *c
	next.tenant, next.space = tenant, space
	return &next
}
