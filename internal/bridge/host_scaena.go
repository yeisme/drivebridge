package bridge

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OutputSource belongs to the host adapter: it reads an existing successful
// domain artifact and cannot submit, approve or retry generation.
type OutputSource interface {
	OpenOutput(context.Context, Space, string) (io.ReadCloser, int64, string, error)
}
type ScaenaOutput struct {
	BaseURL string
	Token   func() (string, error)
	Client  *http.Client
}

func NewScaenaOutput(base string, token func() (string, error)) (*ScaenaOutput, error) {
	u, e := url.Parse(base)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
		return nil, errors.New("scaena_endpoint_invalid")
	}
	return &ScaenaOutput{BaseURL: base, Token: token, Client: &http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *ScaenaOutput) request(ctx context.Context, method, path, key string) (*http.Response, error) {
	if !strings.HasPrefix(path, "/api/v1/projects/") || strings.ContainsAny(path, "?#") {
		return nil, errors.New("scaena_path_invalid")
	}
	token, err := c.Token()
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, errors.New("authentication_required")
	}
	var body io.Reader
	if method == "POST" {
		body = bytes.NewBufferString(`{"ttl_seconds":900}`)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, errors.New("scaena_request_invalid")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	response, err := c.Client.Do(req)
	if err != nil {
		return nil, errors.New("scaena_unavailable")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, errors.New("scaena_unavailable")
	}
	return response, nil
}
func (c *ScaenaOutput) OpenOutput(ctx context.Context, p Space, submission string) (io.ReadCloser, int64, string, error) {
	if p.Project == "" || p.Actor == "" || !strings.HasPrefix(submission, "video-provider-task:") {
		return nil, 0, "", errors.New("output_not_allowed")
	}
	prefix := "/api/v1/projects/" + url.PathEscape(p.Project)
	response, e := c.request(ctx, "GET", prefix+"/video-tasks/"+url.PathEscape(submission), "")
	if e != nil {
		return nil, 0, "", e
	}
	var task struct {
		Data struct {
			Status  string `json:"status"`
			Project string `json:"project_ref"`
			Ref     string `json:"submission_ref"`
			Asset   string `json:"asset_ref"`
			Import  string `json:"import_status"`
		} `json:"data"`
	}
	e = json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&task)
	response.Body.Close()
	if e != nil || task.Data.Ref != submission || task.Data.Project != p.Project || task.Data.Status != "succeeded" || task.Data.Import != "imported" || task.Data.Asset == "" {
		return nil, 0, "", errors.New("output_not_ready")
	}
	path := prefix + "/artifacts/" + url.PathEscape(task.Data.Asset) + "/access-grants"
	// A new read grant can be acquired after expiry. This never creates a task.
	response, e = c.request(ctx, "POST", path, "drivebridge-output-"+newID())
	if e != nil {
		return nil, 0, "", e
	}
	var grant struct {
		Data struct {
			Grant  string `json:"grant_ref"`
			Size   int64  `json:"size_bytes"`
			Digest string `json:"digest"`
			Kind   string `json:"access_kind"`
		} `json:"data"`
	}
	e = json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&grant)
	response.Body.Close()
	if e != nil || grant.Data.Grant == "" || grant.Data.Kind != "resource_link" || grant.Data.Size <= 0 || grant.Data.Size > p.MaxBytes || !strings.HasPrefix(grant.Data.Digest, "sha256:") {
		return nil, 0, "", errors.New("output_grant_invalid")
	}
	digest := strings.TrimPrefix(grant.Data.Digest, "sha256:")
	decoded, decodeErr := hex.DecodeString(digest)
	if decodeErr != nil || len(decoded) != 32 || digest != strings.ToLower(digest) {
		return nil, 0, "", errors.New("output_grant_invalid")
	}
	// Construct the authenticated owner path, ignoring any resource_link URL.
	response, e = c.request(ctx, "GET", path+"/"+url.PathEscape(grant.Data.Grant), "")
	if e != nil {
		return nil, 0, "", e
	}
	return response.Body, grant.Data.Size, strings.TrimPrefix(grant.Data.Digest, "sha256:"), nil
}
func (s *Service) SaveOutput(space, submission, key string) (Operation, error) {
	if !s.Enabled {
		return Operation{}, errors.New("disabled")
	}
	if s.Output == nil {
		return Operation{}, errors.New("output_adapter_unavailable")
	}
	p, e := s.Space(space)
	if e != nil {
		return Operation{}, e
	}
	if p.Project == "" || p.Actor == "" || key == "" || !strings.HasPrefix(submission, "video-provider-task:") {
		return Operation{}, errors.New("output_not_allowed")
	}
	id := newID()
	return s.enqueue(Operation{ID: id, Key: operationKey(space, "save_output", key), SpaceID: space, Kind: "save_output", SourceRef: submission, Status: "queued", LocalPath: filepath.Join(".drivebridge-output", id+".mp4"), Target: id + "-output.mp4", Fingerprint: submission})
}
func (s *Service) saveOutput(ctx context.Context, op *Operation) error {
	if err := s.rejectOperationMutation(*op); err != nil {
		return err
	}
	if s.Output == nil {
		return errors.New("output_adapter_unavailable")
	}
	p, e := s.Space(op.SpaceID)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(p.LocalRoot)
	if e != nil {
		return e
	}
	defer root.Close()
	in, n, digest, e := s.Output.OpenOutput(ctx, p, op.SourceRef)
	if e != nil {
		return e
	}
	defer in.Close()
	// Opening the owner stream may block or install new adoption facts. Only
	// acquire publication admission afterwards, before any local mutation.
	e = func() error {
		s.guardMu.Lock()
		defer s.guardMu.Unlock()
		temp := op.LocalPath + "-" + newID()
		if err := s.rejectOperationMutation(*op); err != nil {
			return err
		}
		for _, rel := range []string{".drivebridge-output", temp} {
			if err := s.rejectLocalOwnerProtected(op.SpaceID, rel); err != nil {
				return err
			}
		}
		if err := root.MkdirAll(".drivebridge-output", 0700); err != nil {
			return err
		}
		out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		op.Bytes = n
		s.DB.Model(&Operation{}).Where("id = ?", op.ID).Update("bytes", n)
		actual, count, err := digestReader(io.TeeReader(io.LimitReader(&progressReader{r: in, s: s, id: op.ID}, p.MaxBytes+1), out))
		syncErr := out.Sync()
		out.Close()
		if err != nil || syncErr != nil || count != n || digest != actual {
			return errors.New("content_mismatch")
		}
		if err := s.rejectOperationMutation(*op); err != nil {
			return err
		}
		if err := root.Link(temp, op.LocalPath); err != nil {
			existing, openErr := root.Open(op.LocalPath)
			if openErr != nil {
				return errors.New("target_conflict")
			}
			h, oldN, hashErr := digestReader(io.LimitReader(existing, p.MaxBytes+1))
			existing.Close()
			if hashErr != nil || h != digest || oldN != n {
				return errors.New("target_conflict")
			}
		}
		_ = root.Remove(temp)
		return nil
	}()
	if e != nil {
		return e
	}
	// Keep the original operation's idempotency identity. Reuse only its transfer
	// implementation, never the generation submit/reconcile path.
	copy := *op
	copy.Kind = "upload"
	copy.Bytes = n
	copy.Fingerprint = digest + ":output.mp4"
	e = s.transfer(ctx, &copy)
	op.FileRef = copy.FileRef
	op.ContentSHA256 = copy.ContentSHA256
	op.Bytes = n
	if e != nil {
		return e
	}
	// Default retention: retain verified local output for recovery. Cleanup is an
	// explicit operator action, not implicit deletion of the user's files.
	return nil
}
