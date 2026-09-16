package bridge

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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

type RelayTarget struct {
	ID           string `json:"id" gorm:"primaryKey"`
	SpaceID      string `json:"space_id"`
	Project      string `json:"project"`
	ConnectionID string `json:"connection_id"`
	Owner        string `json:"owner" gorm:"default:eikona"`
	Origin       string `json:"-"`
	KeyFile      string `json:"-"`
	Disabled     bool   `json:"disabled"`
}

// relayProfile is the small explicit per-owner adapter for original input
// completion: each owner keeps its own domain pending state and reference
// scheme, and media pre-filtering mirrors what that owner accepts. Domain
// validation and adoption stay with the owner's own input service.
type relayProfile struct {
	domainState string
	refPrefix   string
	media       map[string]bool
}

var relayProfiles = map[string]relayProfile{
	"eikona": {domainState: "unreviewed", refPrefix: "eikona://asset/", media: map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}},
	"scaena": {domainState: "requires_finalize", refPrefix: "scaena://", media: map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "video/mp4": true}},
	"sonora": {domainState: "requires_permission", refPrefix: "sonora://", media: map[string]bool{"audio/mpeg": true, "audio/wav": true, "audio/x-wav": true, "audio/wave": true, "audio/flac": true, "audio/ogg": true, "audio/mp4": true}},
}

// relayOwner resolves the configured owner; targets registered before the
// owner column existed are Eikona bindings from the first deployed slice.
func relayOwner(name string) (string, relayProfile, bool) {
	if name == "" {
		name = "eikona"
	}
	p, ok := relayProfiles[name]
	return name, p, ok
}

func relayToken(file string) (string, error) {
	i, err := os.Lstat(file)
	if err != nil || !filepath.IsAbs(file) || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 || i.Size() > 8192 {
		return "", errors.New("private_token_file_required")
	}
	f, err := os.Open(file)
	if err != nil {
		return "", errors.New("credential_unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(i, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 {
		return "", errors.New("private_token_file_required")
	}
	b, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(b) > 8192 {
		return "", errors.New("credential_unavailable")
	}
	value := strings.TrimSpace(string(b))
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("credential_unavailable")
	}
	return value, nil
}
func (s *Service) ConfigureRelay(q RelayTarget) error {
	if q.ID == "" || len(q.ID) > 128 || q.Project == "" || q.ConnectionID == "" {
		return errors.New("invalid_relay_target")
	}
	if _, err := s.Space(q.SpaceID); err != nil {
		return err
	}
	u, err := url.Parse(q.Origin)
	if err != nil || u.Host == "" || u.Path != "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
		return errors.New("invalid_relay_target")
	}
	if _, err := relayToken(q.KeyFile); err != nil {
		return err
	}
	owner, _, ok := relayOwner(q.Owner)
	if !ok {
		return errors.New("invalid_relay_target")
	}
	q.Owner = owner
	if s.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&q).Error != nil {
		return errors.New("state_unavailable")
	}
	var existing RelayTarget
	if s.DB.Where(&RelayTarget{ID: q.ID}).First(&existing).Error != nil {
		return errors.New("state_unavailable")
	}
	if existing != q {
		return errors.New("relay_target_conflict")
	}
	return nil
}
func (s *Service) RelayTargets() ([]RelayTarget, error) {
	var rows []RelayTarget
	err := s.DB.Find(&rows).Error
	return rows, err
}
func (s *Service) DisableRelay(id string) error {
	if id == "" {
		return errors.New("relay_target_not_found")
	}
	r := s.DB.Model(&RelayTarget{}).Where(&RelayTarget{ID: id}).Update("disabled", true)
	if r.Error != nil {
		return errors.New("state_unavailable")
	}
	if r.RowsAffected != 1 {
		return errors.New("relay_target_not_found")
	}
	return nil
}
func (s *Service) relayTarget(id string) (RelayTarget, error) {
	if id == "" {
		return RelayTarget{}, errors.New("relay_target_not_found")
	}
	var target RelayTarget
	if s.DB.Where(&RelayTarget{ID: id}).First(&target).Error != nil || target.Disabled {
		return target, errors.New("relay_target_unavailable")
	}
	return target, nil
}

func (s *Service) StartRelay(ctx context.Context, ref, targetID, key string) (Operation, error) {
	if !s.Enabled {
		return Operation{}, errors.New("disabled")
	}
	if key == "" || len(key) > 256 {
		return Operation{}, errors.New("invalid_idempotency_key")
	}
	target, err := s.relayTarget(targetID)
	if err != nil {
		return Operation{}, err
	}
	file, err := s.Stat(ctx, ref)
	if err != nil {
		return Operation{}, err
	}
	if file.SpaceID != target.SpaceID || file.Trashed != nil || file.Size <= 0 || file.Size > 128<<20 {
		return Operation{}, errors.New("source_not_allowed")
	}
	fp := sha256.Sum256([]byte(ref + "\x00" + file.Version + "\x00" + targetID))
	return s.enqueue(Operation{ID: newID(), Kind: "creative_relay", SpaceID: file.SpaceID, SourceRef: ref, SourceVersion: file.Version, RelayTargetID: targetID, Key: operationKey(file.SpaceID, "creative_relay", key), Fingerprint: hex.EncodeToString(fp[:]), Bytes: file.Size, Status: "queued"})
}

type relayImport struct {
	ID          string `json:"import_id"`
	InputID     string `json:"input_request_id"`
	State       string `json:"state"`
	Ref         string `json:"asset_uri"`
	SHA256      string `json:"sha256"`
	DomainState string `json:"domain_state"`
}
type relayHTTP struct {
	target RelayTarget
	token  string
	client *http.Client
}

func newRelayHTTP(target RelayTarget) (relayHTTP, error) {
	token, err := relayToken(target.KeyFile)
	return relayHTTP{target, token, &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, err
}
func (c relayHTTP) json(ctx context.Context, method, path string, value, out any) error {
	var body io.Reader
	if value != nil {
		b, err := json.Marshal(value)
		if err != nil {
			return errors.New("invalid_relay_request")
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.target.Origin+path, body)
	if err != nil {
		return errors.New("invalid_relay_request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return errors.New("relay_target_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("relay_target_rejected")
	}
	if out == nil {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&envelope) != nil || json.Unmarshal(envelope.Data, out) != nil {
		return errors.New("invalid_relay_response")
	}
	return nil
}

func (s *Service) runRelay(ctx context.Context, op *Operation) error {
	target, err := s.relayTarget(op.RelayTargetID)
	if err != nil {
		return err
	}
	_, profile, ok := relayOwner(target.Owner)
	if !ok {
		return errors.New("relay_target_unavailable")
	}
	client, err := newRelayHTTP(target)
	if err != nil {
		return err
	}
	query := "?scope=" + url.QueryEscape(target.Project)
	var imported relayImport
	if op.ImportID != "" {
		if err := client.json(ctx, "GET", "/api/v1/drivebridge/imports/"+url.PathEscape(op.ImportID)+query, nil, &imported); err != nil {
			return err
		}
		switch imported.State {
		case "ready":
			if imported.SHA256 != op.ContentSHA256 {
				return errors.New("content_mismatch")
			}
			op.ResultRef = imported.Ref
			op.Transferred = op.Bytes
			return nil
		case "expired", "cancelled", "rejected":
			// A terminated or expired original input is never resurrected:
			// recovery fails explicitly instead of re-uploading or creating a
			// second domain input under the same idempotency key.
			return errors.New("relay_input_terminal")
		}
	}
	file, err := s.Stat(ctx, op.SourceRef)
	if err != nil {
		return err
	}
	if file.SpaceID != target.SpaceID || file.Version != op.SourceVersion || file.Size != op.Bytes || file.Trashed != nil {
		return errors.New("source_changed")
	}
	space, err := s.Space(file.SpaceID)
	if err != nil {
		return err
	}
	source, err := s.providerFor(space).Open(ctx, space, file)
	if err != nil {
		return err
	}
	defer source.Close()
	temp, err := os.CreateTemp(s.state, "relay-")
	if err != nil {
		return errors.New("state_unavailable")
	}
	defer func() { temp.Close(); os.Remove(temp.Name()) }()
	digest := sha256.New()
	n, err := io.Copy(io.MultiWriter(temp, digest), io.LimitReader(source, op.Bytes+1))
	if err != nil || n != op.Bytes {
		return errors.New("source_changed")
	}
	sha := hex.EncodeToString(digest.Sum(nil))
	if op.ContentSHA256 != "" && op.ContentSHA256 != sha {
		return errors.New("source_changed")
	}
	op.ContentSHA256 = sha
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var head [512]byte
	size, _ := temp.Read(head[:])
	mime := http.DetectContentType(head[:size])
	if !profile.media[mime] {
		return errors.New("unsupported_media")
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if op.ImportID == "" {
		q := map[string]any{"connection_id": target.ConnectionID, "source_ref": op.SourceRef, "sha256": sha, "idempotency_key": op.ID, "file": map[string]any{"name": "reference", "mime": mime, "size": op.Bytes, "sha256": sha}}
		if err := client.json(ctx, "POST", "/api/v1/drivebridge/imports"+query, q, &imported); err != nil {
			return err
		}
		if imported.ID == "" || imported.InputID == "" {
			return errors.New("invalid_relay_response")
		}
		op.ImportID, op.InputID = imported.ID, imported.InputID
		if s.DB.Model(&Operation{}).Where(&Operation{ID: op.ID}).Updates(map[string]any{"import_id": op.ImportID, "input_id": op.InputID, "content_sha256": sha}).Error != nil {
			return errors.New("state_unavailable")
		}
	}
	var access struct {
		URL     string `json:"transfer_url"`
		Request struct {
			ID string `json:"input_request_id"`
		}
	}
	if err := client.json(ctx, "POST", "/api/v1/drivebridge/imports/"+url.PathEscape(op.ImportID)+"/transfer"+query, struct{}{}, &access); err != nil {
		return err
	}
	u, err := url.Parse(access.URL)
	origin, _ := url.Parse(target.Origin)
	if err != nil || u.Scheme != origin.Scheme || u.Host != origin.Host || u.User != nil || u.RawQuery != "" || u.Path != "/input-requests/"+op.InputID || access.Request.ID != op.InputID {
		return errors.New("untrusted_transfer")
	}
	grant := strings.TrimPrefix(u.Fragment, "grant=")
	if grant == "" || grant == u.Fragment {
		return errors.New("untrusted_transfer")
	}
	u.Fragment = ""
	// The short-lived grant is used only here, never persisted or returned to MCP.
	transfer := func(method, suffix string, body io.Reader, length int64) error {
		if _, err := s.relayTarget(target.ID); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String()+suffix, body)
		if err != nil {
			return errors.New("invalid_relay_request")
		}
		req.Header.Set("X-Input-Grant", grant)
		req.Header.Set("Content-Type", mime)
		req.ContentLength = length
		res, err := client.client.Do(req)
		if err != nil {
			return errors.New("relay_transfer_failed")
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return errors.New("relay_transfer_rejected")
		}
		return nil
	}
	if err := transfer("PUT", "/content", &progressReader{r: temp, s: s, id: op.ID}, op.Bytes); err != nil {
		return err
	}
	if err := transfer("POST", "/complete", nil, 0); err != nil {
		return err
	}
	if err := client.json(ctx, "GET", "/api/v1/drivebridge/imports/"+url.PathEscape(op.ImportID)+query, nil, &imported); err != nil {
		return err
	}
	if imported.State != "ready" || imported.SHA256 != sha || imported.DomainState != profile.domainState || !strings.HasPrefix(imported.Ref, profile.refPrefix) {
		return errors.New("relay_completion_pending")
	}
	op.ResultRef = imported.Ref
	op.Transferred = op.Bytes
	return nil
}

func (s *Service) CancelRelay(ctx context.Context, id string) (Operation, error) {
	op, err := s.Get(id)
	if err != nil {
		return op, err
	}
	if op.Kind != "creative_relay" {
		return op, errors.New("not_a_relay")
	}
	op, err = s.Cancel(id)
	if err != nil {
		return op, err
	}
	if op.Kind != "creative_relay" || op.Status == "completed" || op.ImportID == "" {
		return op, nil
	}
	target, err := s.relayTarget(op.RelayTargetID)
	if err != nil {
		return op, errors.New("remote_cancel_pending")
	}
	client, err := newRelayHTTP(target)
	if err != nil {
		return op, errors.New("remote_cancel_pending")
	}
	if err := client.json(ctx, "POST", "/api/v1/drivebridge/imports/"+url.PathEscape(op.ImportID)+"/cancel?scope="+url.QueryEscape(target.Project), struct{}{}, nil); err != nil {
		return op, errors.New("remote_cancel_pending")
	}
	return op, nil
}

// RelayDiscovery is a read-only projection of local configuration, not a probe.
func (s *Service) RelayDiscovery() map[string]any {
	targets, err := s.RelayTargets()
	if targets == nil {
		targets = []RelayTarget{}
	}
	owners := map[string]bool{}
	for _, t := range targets {
		if name, _, ok := relayOwner(t.Owner); ok && !t.Disabled {
			owners[name] = true
		}
	}
	list := make([]string, 0, len(owners))
	for name := range owners {
		list = append(list, name)
	}
	sort.Strings(list)
	out := map[string]any{"targets": targets, "enabled": s.Enabled, "owners": list, "network_readiness": "not_probed", "max_relay_bytes": 128 << 20, "recovery_tool": "operation_get", "configuration_command": "drivebridge relay target-set --help"}
	if err != nil {
		out["reason"] = "state_unavailable"
		out["targets"] = []RelayTarget{}
		out["owners"] = []string{}
	}
	return out
}
