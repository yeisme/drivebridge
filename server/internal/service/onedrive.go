package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type oneDriveCredential struct {
	Token   oauth2.Token `json:"token"`
	DriveID string       `json:"drive_id"`
}
type DriveItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	ETag string `json:"eTag"`
	File *struct {
		MIME string `json:"mimeType"`
	} `json:"file"`
	Folder *struct {
		Count int `json:"childCount"`
	} `json:"folder"`
	Download string `json:"@microsoft.graph.downloadUrl,omitempty"`
}

func (s *Service) msOAuth() oauth2.Config {
	return oauth2.Config{ClientID: s.Config.MicrosoftClientID, ClientSecret: s.Config.MicrosoftSecret, Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token"}, Scopes: []string{"https://graph.microsoft.com/Files.ReadWrite", "offline_access"}, RedirectURL: s.Config.Origin + "/auth/onedrive/callback"}
}
func (s *Service) beginOneDrive(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent != nil {
		return code("forbidden")
	}
	if s.Config.MicrosoftClientID == "" {
		return code("onedrive_not_configured")
	}
	state, verifier := ID(), oauth2.GenerateVerifier()
	secret, _ := s.seal(p.dataOwner(), flowSecret{Verifier: verifier})
	flow := Flow{Hash: Hash(state), Owner: p.dataOwner(), Purpose: "onedrive", Secret: secret, Expires: time.Now().Add(10 * time.Minute)}
	if s.DB.Create(&flow).Error != nil {
		return code("database_unavailable")
	}
	s.cookie(w, "db_flow", state, 600)
	cfg := s.msOAuth()
	respond(w, map[string]string{"authorization_url": cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))})
	return nil
}
func (s *Service) finishOneDrive(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled {
		writeError(w, code("disabled"))
		return
	}
	p, e := s.authenticate(r)
	if e != nil || p.Agent != nil {
		writeError(w, code("unauthorized"))
		return
	}
	flow, e := s.consumeFlow(r, "onedrive")
	s.cookie(w, "db_flow", "", -1)
	if e == nil && s.governanceActive() {
		var m Membership
		if s.DB.First(&m, "data_owner = ? AND user_id = ? AND status = ?", flow.Owner, p.UserID, "active").Error == nil {
			if _, err := s.membership(p.UserID, m.TenantID); err == nil {
				p.DataOwner, p.TenantID = m.DataOwner, m.TenantID
			}
		}
	}
	if e != nil || flow.Owner != p.dataOwner() {
		writeError(w, code("invalid_auth_state"))
		return
	}
	var secret flowSecret
	if e = s.unseal(p.dataOwner(), flow.Secret, &secret); e != nil {
		writeError(w, e)
		return
	}
	cfg := s.msOAuth()
	token, e := cfg.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(secret.Verifier))
	if e != nil {
		writeError(w, code("onedrive_authorization_failed"))
		return
	}
	cipher, e := s.seal(p.dataOwner(), oneDriveCredential{Token: *token})
	if e != nil {
		writeError(w, e)
		return
	}
	conn := Connection{ID: ID(), Owner: p.dataOwner(), Kind: "onedrive", Name: "OneDrive", Secret: cipher, Status: "connected"}
	if s.DB.Create(&conn).Error != nil {
		writeError(w, code("database_unavailable"))
		return
	}
	b, e := s.graph(r.Context(), p, conn.ID, "GET", "/me/drive?$select=id", nil)
	if e != nil {
		s.DB.Delete(&conn)
		writeError(w, e)
		return
	}
	var drive struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &drive) != nil || drive.ID == "" {
		s.DB.Delete(&conn)
		writeError(w, code("onedrive_unavailable"))
		return
	}
	var latest Connection
	s.DB.First(&latest, "id = ? AND owner = ?", conn.ID, p.dataOwner())
	var credential oneDriveCredential
	if e = s.unseal(p.dataOwner(), latest.Secret, &credential); e != nil {
		writeError(w, e)
		return
	}
	credential.DriveID = drive.ID
	cipher, _ = s.seal(p.dataOwner(), credential)
	s.DB.Model(&conn).Updates(map[string]any{"secret": cipher, "status": "verified"})
	http.Redirect(w, r, "/#connections", 302)
}
func (s *Service) oneDrive(p Principal, id string) (Connection, oneDriveCredential, error) {
	var c Connection
	var v oneDriveCredential
	if s.DB.First(&c, "id = ? AND owner = ? AND kind = ? AND disabled = ?", id, p.dataOwner(), "onedrive", false).Error != nil {
		return c, v, code("storage_disconnected")
	}
	if p.Agent != nil && p.Agent.ConnectionID != id {
		return c, v, code("forbidden")
	}
	e := s.unseal(p.dataOwner(), c.Secret, &v)
	return c, v, e
}
func (s *Service) graph(ctx context.Context, p Principal, id, method, path string, body any) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, code("invalid_graph_path")
	}
	h := sha256.Sum256([]byte(id))
	lock := &s.connectionLocks[int(h[0])%len(s.connectionLocks)]
	lock.Lock()
	c, v, e := s.oneDrive(p, id)
	if e != nil {
		lock.Unlock()
		return nil, e
	}
	cfg := s.msOAuth()
	fresh, e := cfg.TokenSource(ctx, &v.Token).Token()
	if e != nil {
		lock.Unlock()
		return nil, code("onedrive_authorization_required")
	}
	v.Token = *fresh
	cipher, e := s.seal(p.dataOwner(), v)
	if e == nil {
		e = s.DB.Model(&c).Update("secret", cipher).Error
	}
	lock.Unlock()
	if e != nil {
		return nil, code("credential_unavailable")
	}
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, "https://graph.microsoft.com/v1.0"+path, reader)
	if e != nil {
		return nil, code("invalid_graph_path")
	}
	req.Header.Set("Authorization", "Bearer "+fresh.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if s.graphHTTP != nil {
		client = s.graphHTTP
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, code("onedrive_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return nil, code("throttled")
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, code("onedrive_authorization_required")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, code("onedrive_unavailable")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, code("onedrive_unavailable")
	}
	return b, nil
}
func (s *Service) driveItem(ctx context.Context, p Principal, connection, item string, access bool) (DriveItem, error) {
	_, v, e := s.oneDrive(p, connection)
	if e != nil {
		return DriveItem{}, e
	}
	fields := "id,name,size,eTag,file,folder"
	if access {
		fields += ",@microsoft.graph.downloadUrl"
	}
	b, e := s.graph(ctx, p, connection, "GET", "/drives/"+url.PathEscape(v.DriveID)+"/items/"+url.PathEscape(item)+"?$select="+url.QueryEscape(fields), nil)
	if e != nil {
		return DriveItem{}, e
	}
	var out DriveItem
	if json.Unmarshal(b, &out) != nil || out.ID != item {
		return out, code("invalid_graph_response")
	}
	return out, nil
}
func (s *Service) browseOneDrive(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent != nil {
		if p.Agent.ConnectionID != r.PathValue("id") {
			return code("forbidden")
		}
		if e := s.allow(p, p.Agent.SpaceID, p.Agent.Prefix, "import"); e != nil {
			return e
		}
	}
	id := r.PathValue("id")
	_, v, e := s.oneDrive(p, id)
	if e != nil {
		return e
	}
	parent := r.URL.Query().Get("parent")
	location := "root"
	if parent != "" {
		location = "items/" + url.PathEscape(parent)
	}
	q := r.URL.Query().Get("cursor")
	extra := ""
	if q != "" {
		extra = "&$skiptoken=" + url.QueryEscape(q)
	}
	b, e := s.graph(r.Context(), p, id, "GET", "/drives/"+url.PathEscape(v.DriveID)+"/"+location+"/children?$top=100&$select=id,name,size,eTag,file,folder"+extra, nil)
	if e != nil {
		return e
	}
	var data struct {
		Items []DriveItem `json:"value"`
		Next  string      `json:"@odata.nextLink"`
	}
	if json.Unmarshal(b, &data) != nil {
		return code("invalid_graph_response")
	}
	cursor := ""
	if data.Next != "" {
		u, e := url.Parse(data.Next)
		if e != nil || u.Host != "graph.microsoft.com" {
			return code("invalid_graph_response")
		}
		cursor = u.Query().Get("$skiptoken")
	}
	respond(w, map[string]any{"items": data.Items, "cursor": cursor})
	return nil
}
func microsoftURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, suffix := range []string{".1drv.com", ".sharepoint.com", ".storage.live.com", ".onedrive.com", ".up.1drv.com"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}
func transferHTTP(ctx context.Context, method, raw string, reader io.Reader, headers map[string]string) (*http.Response, error) {
	if !microsoftURL(raw) {
		return nil, code("delivery_host_rejected")
	}
	req, e := http.NewRequestWithContext(ctx, method, raw, reader)
	if e != nil {
		return nil, code("delivery_unavailable")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if v := headers["Content-Length"]; v != "" {
		req.ContentLength, _ = strconv.ParseInt(v, 10, 64)
	}
	client := &http.Client{Timeout: 30 * time.Minute, Transport: &http.Transport{Proxy: nil, DialContext: safeDial(false)}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !microsoftURL(req.URL.String()) {
			return code("delivery_redirect_rejected")
		}
		return nil
	}}
	resp, e := client.Do(req)
	if e != nil {
		return nil, code("delivery_unavailable")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, code("delivery_unavailable")
	}
	return resp, nil
}
func (s *Service) importOneDrive(w http.ResponseWriter, r *http.Request, p Principal) error {
	var q struct {
		ItemID  string `json:"item_id"`
		Version string `json:"version"`
		SpaceID string `json:"space_id"`
		Path    string `json:"path"`
	}
	if e := decode(r, &q); e != nil {
		return e
	}
	if e := s.allow(p, q.SpaceID, q.Path, "import"); e != nil {
		return e
	}
	item, e := s.driveItem(r.Context(), p, r.PathValue("id"), q.ItemID, false)
	if e != nil {
		return e
	}
	if item.ETag != q.Version || item.File == nil {
		return code("source_version_changed")
	}
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: q.SpaceID, Path: q.Path, Size: item.Size, SHA256: Hash(r.PathValue("id") + q.ItemID + q.Version)}, r.Header.Get("Idempotency-Key"))
	if e != nil {
		return e
	}
	if op.Status == "completed" {
		respond(w, op)
		return nil
	}
	op.Kind = "import"
	op.SourceID = q.ItemID
	op.SourceVersion = q.Version // The destination connection remains pinned on the upload.
	op.ExternalConnectionID = r.PathValue("id")
	s.DB.Model(&Operation{}).Where("id = ? AND status <> ?", op.ID, "completed").Updates(map[string]any{"kind": op.Kind, "source_id": op.SourceID, "source_version": op.SourceVersion, "external_connection_id": op.ExternalConnectionID})
	op, e = s.runImport(r.Context(), p, op)
	if e != nil {
		return e
	}
	respond(w, op)
	return nil
}
func (s *Service) runImport(ctx context.Context, p Principal, op Operation) (result Operation, failure error) {
	defer func() { s.failedOperation(op.ID, failure) }()
	hash := sha256.Sum256([]byte(op.ID))
	lock := &s.sourceLocks[int(hash[0])%len(s.sourceLocks)]
	lock.Lock()
	defer lock.Unlock()
	ctx, finish := s.operationContext(ctx, op.ID)
	defer finish()
	currentOp, e := s.operation(p, op.ID, "write")
	if e != nil {
		return op, e
	}
	if currentOp.Status == "completed" || currentOp.Status == "cancelled" {
		return currentOp, nil
	}

	if e := s.allow(p, op.SpaceID, op.Path, "import"); e != nil {
		return op, e
	}
	item, e := s.driveItem(ctx, p, op.ExternalConnectionID, op.SourceID, true)
	if e != nil {
		return op, e
	}
	if item.ETag != op.SourceVersion || item.Size != op.Size {
		return op, code("source_version_changed")
	}
	resp, e := s.sensitiveTransfer(ctx, "GET", item.Download, nil, nil)
	if e != nil {
		return op, e
	}
	defer resp.Body.Close()
	h := sha256.New()
	for i := 0; int64(i)*PartSize < op.Size; i++ {
		size := min(PartSize, op.Size-int64(i)*PartSize)
		part := make([]byte, int(size))
		if _, e = io.ReadFull(resp.Body, part); e != nil {
			return op, code("incomplete_part")
		}
		h.Write(part)
		if _, e = s.PutPart(p, op.ID, i, bytes.NewReader(part)); e != nil {
			return op, e
		}
	}
	one := make([]byte, 1)
	if n, _ := resp.Body.Read(one); n != 0 {
		return op, code("content_mismatch")
	}
	current, e := s.driveItem(ctx, p, op.ExternalConnectionID, op.SourceID, false)
	if e != nil || current.ETag != op.SourceVersion {
		return op, code("source_version_changed")
	}
	s.DB.Model(&Operation{}).Where("id = ?", op.ID).Update("sha256", hex.EncodeToString(h.Sum(nil)))
	return s.Complete(ctx, p, op.ID)
}
func (s *Service) ExportFile(ctx context.Context, p Principal, fileID, connectionID, key string) (Operation, error) {
	if key == "" || len(key) > 200 {
		return Operation{}, code("idempotency_key_required")
	}
	f, e := s.file(p, fileID, "export")
	if e != nil {
		return Operation{}, e
	}
	if _, _, e = s.oneDrive(p, connectionID); e != nil {
		return Operation{}, e
	}
	fingerprint := Hash("export:" + f.ID + f.VersionID + connectionID)
	s.mu.Lock()
	var op Operation
	if s.DB.First(&op, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
		if op.Fingerprint != fingerprint {
			s.mu.Unlock()
			return Operation{}, code("idempotency_conflict")
		}
	} else {
		op = Operation{ID: ID(), Owner: p.dataOwner(), Key: key, Fingerprint: fingerprint, Kind: "export", SpaceID: f.SpaceID, Path: f.Path, FileID: f.ID, VersionID: f.VersionID, Size: f.Size, Status: "awaiting_upload", ExternalConnectionID: connectionID, Updated: time.Now().UTC()}
		if s.DB.Create(&op).Error != nil {
			s.mu.Unlock()
			return Operation{}, code("database_unavailable")
		}
	}
	s.mu.Unlock()
	if op.Status == "completed" {
		return op, nil
	}
	return s.runExport(ctx, p, op)
}

func (s *Service) exportOneDrive(w http.ResponseWriter, r *http.Request, p Principal) error {
	var q struct {
		FileID string `json:"file_id"`
	}
	if e := decode(r, &q); e != nil {
		return e
	}
	op, e := s.ExportFile(r.Context(), p, q.FileID, r.PathValue("id"), r.Header.Get("Idempotency-Key"))
	if e != nil {
		return e
	}
	respond(w, op)
	return nil
}

type DirectoryExportRequest struct {
	SpaceID      string `json:"space_id"`
	Path         string `json:"path"`
	ConnectionID string `json:"connection_id"`
}

func (s *Service) ExportDirectory(ctx context.Context, p Principal, q DirectoryExportRequest, key string) (Operation, error) {
	if key == "" || len(key) > 200 || q.SpaceID == "" || q.ConnectionID == "" {
		return Operation{}, code("invalid_directory")
	}
	src, err := cleanPath(q.Path)
	if err != nil {
		return Operation{}, err
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return Operation{}, e
	}
	if e := s.allow(p, q.SpaceID, src, "export"); e != nil {
		return Operation{}, e
	}
	if _, _, e := s.oneDrive(p, q.ConnectionID); e != nil {
		return Operation{}, e
	}
	files, _, err := s.listPrefix(p, q.SpaceID, src)
	if err != nil {
		return Operation{}, err
	}
	if len(files) > directoryCopyLimit {
		return Operation{}, code("directory_too_large")
	}
	if len(files) == 0 {
		return Operation{}, code("not_found")
	}
	body, _ := json.Marshal(q)
	fingerprint := Hash("directory_export:" + string(body))
	s.mu.Lock()
	var op Operation
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var old Operation
		if tx.First(&old, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
			if old.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			op = old
			return nil
		}
		var size int64
		for _, f := range files {
			size += f.Size
		}
		op = Operation{
			ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), Key: key, Fingerprint: fingerprint,
			Kind: "directory_export", SpaceID: q.SpaceID, Path: src, SourcePath: src,
			ExternalConnectionID: q.ConnectionID, Status: "running", Size: size,
			TotalFiles: len(files), Updated: time.Now().UTC(),
		}
		return tx.Create(&op).Error
	})
	s.mu.Unlock()
	if err != nil {
		return Operation{}, err
	}
	if op.Status == "completed" || op.Status == "cancelled" {
		return op, nil
	}
	done := 0
	for _, f := range files {
		sub := op.Key + "/" + f.ID
		if len(sub) > 200 {
			sub = Hash(op.Key + ":" + f.ID)
		}
		child, e := s.ExportFile(ctx, p, f.ID, q.ConnectionID, sub)
		if e != nil {
			s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": safeCode(e), "progress_files": done, "updated": time.Now().UTC()})
			op.Status, op.Error, op.ProgressFiles = "failed", safeCode(e), done
			return op, e
		}
		if child.Status != "completed" {
			s.DB.Model(&op).Updates(map[string]any{"status": "running", "progress_files": done, "updated": time.Now().UTC()})
		}
		done++
		s.DB.Model(&op).Updates(map[string]any{"progress_files": done, "updated": time.Now().UTC()})
	}
	now := time.Now().UTC()
	s.DB.Model(&op).Updates(map[string]any{"status": "completed", "progress_files": done, "error": "", "updated": now})
	op.Status, op.ProgressFiles, op.Error, op.Updated = "completed", done, "", now
	return op, nil
}
func (s *Service) runExport(ctx context.Context, p Principal, op Operation) (result Operation, failure error) {
	defer func() {
		if s.Insights != nil {
			if failure == nil && result.Status == "completed" {
				_ = s.Insights.Record(insights.Fact{ID: "completed-" + result.ID, Dimensions: s.dimensions(p, result), Metric: "effective_bytes", Direction: "export", Purpose: "export", Bytes: result.Size})
			} else if failure != nil {
				_ = s.Insights.Record(insights.Fact{Dimensions: s.dimensions(p, op), Metric: "failed_attempt", Purpose: "export"})
			}
		}
	}()
	defer func() { s.failedOperation(op.ID, failure) }()
	unlock := s.opLock(op.ID)
	defer unlock()
	ctx, finish := s.operationContext(ctx, op.ID)
	defer finish()
	currentOp, e := s.operation(p, op.ID, "export")
	if e != nil {
		return op, e
	}
	if currentOp.Status == "completed" || currentOp.Status == "cancelled" {
		return currentOp, nil
	}
	if e := s.allow(p, op.SpaceID, op.Path, "export"); e != nil {
		return op, e
	}
	_, credentials, e := s.oneDrive(p, op.ExternalConnectionID)
	if e != nil {
		return op, e
	}
	f, v, body, e := s.Download(ctx, p, op.FileID, op.VersionID)
	if e != nil {
		return op, e
	}
	defer body.Close()
	name := op.ID + "-" + strings.ReplaceAll(f.Path, "/", "-")
	if len(name) > 220 {
		name = op.ID + "-output"
	}
	target := "/drives/" + url.PathEscape(credentials.DriveID) + "/root:/" + url.PathEscape(name)
	// Probe the operation-specific destination before opening a new upload session.
	if b, err := s.graph(ctx, p, op.ExternalConnectionID, "GET", target+":?$select=id,eTag,size,@microsoft.graph.downloadUrl", nil); err == nil {
		var item DriveItem
		if json.Unmarshal(b, &item) == nil && item.ID != "" {
			resp, err := s.sensitiveTransfer(ctx, "GET", item.Download, nil, nil)
			if err != nil {
				return op, err
			}
			h := sha256.New()
			n, err := io.Copy(h, io.LimitReader(resp.Body, v.Size+1))
			resp.Body.Close()
			if err != nil || n != v.Size || hex.EncodeToString(h.Sum(nil)) != v.SHA256 {
				return op, code("export_conflict")
			}
			op.Status = "completed"
			op.SourceID = item.ID
			op.SourceVersion = item.ETag
			s.DB.Save(&op)
			return op, nil
		}
	}
	b, e := s.graph(ctx, p, op.ExternalConnectionID, "POST", target+":/createUploadSession", map[string]any{"item": map[string]any{"@microsoft.graph.conflictBehavior": "fail", "name": name}})
	if e != nil {
		return op, e
	}
	var session struct {
		URL string `json:"uploadUrl"`
	}
	if json.Unmarshal(b, &session) != nil || session.URL == "" {
		return op, code("invalid_graph_response")
	}
	const chunk int64 = 10 << 20
	sourceHash := sha256.New()
	for offset := int64(0); offset < v.Size; {
		n := min(chunk, v.Size-offset)
		buf := make([]byte, int(n))
		if _, e = io.ReadFull(body, buf); e != nil {
			// Local managed-body read, not an object-store Get: reporting it
			// as a storage read would misdirect retry decisions.
			return op, code("transfer_failed")
		}
		sourceHash.Write(buf)
		meter := &insights.ObservedReadSeeker{ReadSeeker: bytes.NewReader(buf)}
		resp, e := s.sensitiveTransfer(ctx, "PUT", session.URL, meter, map[string]string{"Content-Length": strconv.FormatInt(n, 10), "Content-Range": fmt.Sprintf("bytes %d-%d/%d", offset, offset+n-1, v.Size)})
		s.observed(p, op, "server_to_storage", "export", meter.Bytes)
		if e != nil {
			return op, e
		}
		last, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		offset += n
		op.Transferred = offset
		s.DB.Model(&op).Update("transferred", offset)
		if offset == v.Size {
			var item DriveItem
			if json.Unmarshal(last, &item) != nil || item.ID == "" {
				return op, code("export_unconfirmed")
			}
			op.SourceID = item.ID
			op.SourceVersion = item.ETag
		}
	}
	if hex.EncodeToString(sourceHash.Sum(nil)) != v.SHA256 {
		return op, code("content_mismatch")
	}
	op.Status = "completed"
	op.Updated = time.Now().UTC()
	if s.DB.Save(&op).Error != nil {
		return op, code("database_unavailable")
	}
	return op, nil
}
