package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// CreativeConnection owns only the user's consumer connection, never provider credentials.
type CreativeConnection struct {
	ID       string `json:"id" gorm:"primaryKey"`
	Owner    string `json:"-" gorm:"index"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Project  string `json:"project_ref"`
	Secret   []byte `json:"-"`
	Disabled bool   `json:"disabled"`
}
type creativeSecret struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type CreativeInput struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Role      string `json:"role"`
}
type CreativeReceipt struct {
	ArchiveAttempts int       `json:"archive_attempts"`
	ID              string    `json:"id" gorm:"primaryKey"`
	Owner           string    `json:"-" gorm:"uniqueIndex:creative_key"`
	Key             string    `json:"-" gorm:"uniqueIndex:creative_key"`
	ConnectionID    string    `json:"connection_id"`
	Fingerprint     string    `json:"-"`
	TaskRef         string    `json:"task_ref,omitempty"`
	Status          string    `json:"status"`
	SpaceID         string    `json:"space_id"`
	Path            string    `json:"path"`
	Inputs          []byte    `json:"-"`
	Updated         time.Time `json:"updated_at"`
}
type CreativeAsset struct {
	ID           string `gorm:"primaryKey"`
	ConnectionID string `gorm:"uniqueIndex:creative_asset"`
	VersionID    string `gorm:"uniqueIndex:creative_asset"`
	Ref          string
	Session      string
}
type CreativeSubmit struct {
	MaxCostMicroUSD    int64           `json:"max_cost_micro_usd"`
	ComplianceGrantRef string          `json:"compliance_grant_ref,omitempty"`
	ConnectionID       string          `json:"connection_id"`
	SpaceID            string          `json:"space_id"`
	Path               string          `json:"path"`
	Model              string          `json:"model_ref"`
	Prompt             string          `json:"prompt"`
	Inputs             []CreativeInput `json:"inputs"`
	ConfirmRights      bool            `json:"confirm_rights"`
	ConfirmCost        bool            `json:"confirm_cost"`
	Duration           int             `json:"duration_seconds"`
	SubjectMode        string          `json:"subject_mode"`
}

func (s *Service) creativeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/creative/connections", s.wrap("creative.connections", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		out := []CreativeConnection{}
		if s.DB.Where("owner = ? AND disabled = ?", p.dataOwner(), false).Find(&out).Error != nil {
			return code("database_unavailable")
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/creative/connections", s.wrap("creative.connect", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var q struct {
			Kind    string `json:"kind"`
			Name    string `json:"name"`
			Project string `json:"project_ref"`
			URL     string `json:"url"`
			Token   string `json:"token"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		u, err := url.Parse(q.URL)
		if err != nil || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || u.Scheme != "https" && !(s.Config.Development && u.Scheme == "http" && loopback(u.Hostname())) || q.Kind != "eikona" && q.Kind != "scaena" && q.Kind != "manju" || !validProvenance(q.Project, "") || q.Project == "" || len(q.Token) < 8 || len(q.Token) > 4096 || q.Name == "" || len(q.Name) > 120 {
			return code("invalid_creative_connection")
		}
		v := CreativeConnection{ID: ID(), Owner: p.dataOwner(), Kind: q.Kind, Name: q.Name, Project: q.Project}
		v.Secret, err = s.seal(p.dataOwner(), creativeSecret{URL: strings.TrimRight(q.URL, "/"), Token: q.Token})
		if err != nil {
			return err
		}
		if s.DB.Create(&v).Error != nil {
			return code("database_unavailable")
		}
		respond(w, v)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/creative/connections/{id}/disconnect", s.wrap("creative.disconnect", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		result := s.DB.Model(&CreativeConnection{}).Where("owner = ? AND id = ?", p.dataOwner(), r.PathValue("id")).Update("disabled", true)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected == 0 {
			return code("not_found")
		}
		respond(w, map[string]bool{"disconnected": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/creative/submit", s.wrap("creative.submit", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CreativeSubmit
		if err := decode(r, &q); err != nil {
			return err
		}
		out, err := s.submitCreative(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/creative/receipts", s.wrap("creative.receipts", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		out := []CreativeReceipt{}
		if s.DB.Where("owner = ?", p.dataOwner()).Order("updated DESC").Limit(100).Find(&out).Error != nil {
			return code("database_unavailable")
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/creative/receipts/{id}", s.wrap("creative.status", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		receipt, conn, secret, err := s.creativeReceipt(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		if receipt.TaskRef == "" {
			respond(w, map[string]any{"receipt": receipt, "state": receipt.Status, "artifacts": []string{}})
			return nil
		}
		result, err := s.readCreativeStatus(r.Context(), receipt, conn, secret)
		if err != nil {
			return err
		}
		respond(w, map[string]any{"receipt": receipt, "state": result.State, "artifacts": result.Artifacts, "review_status": result.ReviewStatus})
		return nil
	}))
}
func (s *Service) creativeConnection(p Principal, id string) (CreativeConnection, creativeSecret, error) {
	var conn CreativeConnection
	var secret creativeSecret
	if p.Agent != nil {
		return conn, secret, code("forbidden")
	}
	if s.DB.First(&conn, "id = ? AND owner = ? AND disabled = ?", id, p.dataOwner(), false).Error != nil {
		return conn, secret, code("creative_unavailable")
	}
	err := s.unseal(p.dataOwner(), conn.Secret, &secret)
	return conn, secret, err
}
func (s *Service) creativeReceipt(p Principal, id string) (CreativeReceipt, CreativeConnection, creativeSecret, error) {
	var receipt CreativeReceipt
	if s.DB.First(&receipt, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return receipt, CreativeConnection{}, creativeSecret{}, code("not_found")
	}
	conn, secret, err := s.creativeConnection(p, receipt.ConnectionID)
	return receipt, conn, secret, err
}
func (s *Service) creativeHTTP(ctx context.Context, secret creativeSecret, method, target string, body io.Reader, length int64, key string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, secret.URL+target, body)
	if err != nil {
		return nil, code("creative_unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+secret.Token)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Content-Type", "application/json")
	if method == "PUT" {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if length >= 0 {
		req.ContentLength = length
	}
	tr := &http.Transport{Proxy: nil, DialContext: safeDial(s.Config.Development)}
	client := &http.Client{Transport: tr, Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		tr.CloseIdleConnections()
		return nil, code("creative_transport_unknown")
	}
	// Responses are always closed by callers; idle connections need not persist.
	tr.CloseIdleConnections()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, code("creative_owner_rejected")
	}
	return resp, nil
}
func (s *Service) creativeJSON(ctx context.Context, secret creativeSecret, method, target string, input any, key string, out any) error {
	var reader io.Reader
	if input != nil {
		body, err := json.Marshal(input)
		if err != nil {
			return code("invalid_creative_request")
		}
		reader = bytes.NewReader(body)
	}
	resp, err := s.creativeHTTP(ctx, secret, method, target, reader, -1, key)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&envelope) != nil || len(envelope.Data) == 0 {
		return code("creative_invalid_response")
	}
	if out == nil {
		return nil
	}
	if json.Unmarshal(envelope.Data, out) != nil {
		return code("creative_invalid_response")
	}
	return nil
}
func (s *Service) submitCreative(ctx context.Context, p Principal, q CreativeSubmit, key string) (CreativeReceipt, error) {
	if !q.ConfirmRights || !q.ConfirmCost {
		return CreativeReceipt{}, code("creative_confirmation_required")
	}
	if len(q.Prompt) == 0 || len(q.Prompt) > 10000 || len(q.Inputs) > 8 || key == "" || len(key) > 200 {
		return CreativeReceipt{}, code("invalid_creative_request")
	}
	if q.Path != "" {
		if _, err := cleanPath(q.Path); err != nil {
			return CreativeReceipt{}, err
		}
	}
	if _, err := s.space(p, q.SpaceID); err != nil {
		return CreativeReceipt{}, err
	}
	conn, secret, err := s.creativeConnection(p, q.ConnectionID)
	if err != nil {
		return CreativeReceipt{}, err
	}
	if conn.Kind == "scaena" && (len(q.Inputs) > 1 || q.Model == "" || q.Duration < 1 || q.Duration > 120 || q.MaxCostMicroUSD <= 0) {
		return CreativeReceipt{}, code("creative_reference_limit")
	}
	if conn.Kind == "scaena" && len(q.Inputs) > 0 && q.SubjectMode != "synthetic_virtual_person" && q.SubjectMode != "authorized_real_person" {
		return CreativeReceipt{}, code("creative_subject_confirmation_required")
	}
	if conn.Kind == "eikona" && q.Model == "" {
		q.Model = "openai/gpt-5.4-image-2"
	}
	for _, input := range q.Inputs {
		f, err := s.file(p, input.FileID, "read")
		if err != nil {
			return CreativeReceipt{}, err
		}
		if f.Trashed {
			return CreativeReceipt{}, code("file_trashed")
		}
		var v Version
		if s.DB.First(&v, "id = ? AND file_id = ? AND owner = ?", input.VersionID, f.ID, p.dataOwner()).Error != nil {
			return CreativeReceipt{}, code("not_found")
		}
		if conn.Kind == "eikona" && !strings.HasPrefix(v.MIME, "image/") {
			return CreativeReceipt{}, code("creative_media_unsupported")
		}
	}
	encoded, _ := json.Marshal(q)
	fingerprint := Hash(string(encoded))
	s.creativeMu.Lock()
	defer s.creativeMu.Unlock()
	var receipt CreativeReceipt
	if s.DB.First(&receipt, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
		if receipt.Fingerprint != fingerprint {
			return receipt, code("idempotency_conflict")
		}
		if receipt.Status != "input_transfer_failed" && receipt.Status != "preparing_inputs" {
			return receipt, nil
		}
	}
	inputBytes, _ := json.Marshal(q.Inputs)
	if receipt.ID == "" {
		receipt = CreativeReceipt{ID: ID(), Owner: p.dataOwner(), Key: key, ConnectionID: conn.ID, Fingerprint: fingerprint, SpaceID: q.SpaceID, Path: q.Path, Inputs: inputBytes, Status: "preparing_inputs", Updated: time.Now().UTC()}
		if s.DB.Create(&receipt).Error != nil {
			return receipt, code("database_unavailable")
		}
	}
	refs := []string{}
	for _, input := range q.Inputs {
		ref, err := s.transferCreativeInput(ctx, p, conn, secret, input)
		if err != nil {
			s.DB.Model(&receipt).Update("status", "input_transfer_failed")
			receipt.Status = "input_transfer_failed"
			return receipt, err
		}
		refs = append(refs, ref)
	}
	request := map[string]any{"project_id": conn.Project, "model_ref": q.Model, "prompt": q.Prompt, "reference_images": refs, "idempotency_key": receipt.ID, "execution_mode": "background", "run_id": "run_db_" + receipt.ID}
	target := "/api/v1/generation"
	if conn.Kind == "scaena" {
		target = "/api/v1/projects/" + url.PathEscape(conn.Project) + "/video-tasks"
		request = map[string]any{"binding_id": q.Model, "max_cost_micro_usd": q.MaxCostMicroUSD, "compliance_grant_ref": q.ComplianceGrantRef, "duration_seconds": q.Duration, "resolution": "720p", "ratio": "16:9", "prompt": q.Prompt, "production_scope": "experiment", "confirmed": true, "reference_rights_confirmed": true, "acknowledge_unknown_cost": true, "subject_mode": q.SubjectMode}
		if len(refs) > 0 {
			request["reference_upload_ref"] = refs[0]
		}
	}
	if conn.Kind == "eikona" {
		receipt.TaskRef = "run_db_" + receipt.ID
	}
	if s.DB.Model(&receipt).Updates(map[string]any{"task_ref": receipt.TaskRef, "status": "submission_unknown", "updated": time.Now().UTC()}).Error != nil {
		return receipt, code("database_unavailable")
	}
	var result struct {
		RunID         string `json:"run_id"`
		SubmissionRef string `json:"submission_ref"`
	}
	err = s.creativeJSON(ctx, secret, "POST", target, request, receipt.ID, &result)
	if err != nil {
		receipt.Status = "submission_unknown"
		return receipt, nil
	}
	receipt.TaskRef = result.RunID
	if conn.Kind == "scaena" {
		receipt.TaskRef = result.SubmissionRef
	}
	if receipt.TaskRef == "" {
		receipt.Status = "submission_unknown"
		return receipt, nil
	}
	receipt.Status = "submitted"
	receipt.Updated = time.Now().UTC()
	if s.DB.Save(&receipt).Error != nil {
		return receipt, code("database_unavailable")
	}
	return receipt, nil
}
func (s *Service) transferCreativeInput(ctx context.Context, p Principal, conn CreativeConnection, secret creativeSecret, input CreativeInput) (string, error) {
	s.creativeInputsMu.Lock()
	defer s.creativeInputsMu.Unlock()
	var asset CreativeAsset
	if s.DB.First(&asset, "connection_id = ? AND version_id = ?", conn.ID, input.VersionID).Error == nil && asset.Ref != "" {
		return asset.Ref, nil
	}
	f, v, body, err := s.Download(ctx, p, input.FileID, input.VersionID)
	if err != nil {
		return "", err
	}
	defer body.Close()
	if asset.ID == "" {
		asset = CreativeAsset{ID: ID(), ConnectionID: conn.ID, VersionID: v.ID}
		if s.DB.Create(&asset).Error != nil {
			return "", code("database_unavailable")
		}
	}
	if conn.Kind == "eikona" {
		var session struct {
			SessionID     string `json:"session_id"`
			URL           string `json:"url"`
			Method        string `json:"method"`
			Authorization string `json:"authorization"`
		}
		q := map[string]any{"scope": conn.Project, "mime": v.MIME, "size": v.Size, "sha256": v.SHA256, "idempotency_key": asset.ID}
		if err = s.creativeJSON(ctx, secret, "POST", "/api/v1/asset-uploads", q, asset.ID, &session); err != nil {
			return "", err
		}
		target := "/api/v1/asset-uploads/" + url.PathEscape(session.SessionID)
		if session.Method != "PUT" || session.Authorization != "bearer" || session.URL != secret.URL+target {
			return "", code("creative_upload_transport_unsupported")
		}
		resp, err := s.creativeHTTP(ctx, secret, "PUT", target, body, v.Size, "")
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		var complete struct {
			URI string `json:"uri"`
		}
		if err = s.creativeJSON(ctx, secret, "POST", target+"/complete", q, asset.ID+"-complete", &complete); err != nil {
			return "", err
		}
		if !strings.HasPrefix(complete.URI, "eikona://asset/") {
			return "", code("creative_invalid_response")
		}
		asset.Ref = complete.URI
	} else {
		base := "/api/v1/projects/" + url.PathEscape(conn.Project) + "/upload-sessions"
		usage := "image"
		if strings.HasPrefix(v.MIME, "video/") {
			usage = "video"
		}
		var session struct {
			Ref      string `json:"session_ref"`
			PartSize int64  `json:"part_size"`
			Total    int64  `json:"total_size"`
		}
		if err = s.creativeJSON(ctx, secret, "POST", base, map[string]any{"file_name": path.Base(f.Path), "mime_type": v.MIME, "total_size": v.Size, "part_size": min(v.Size, 8<<20), "digest_algorithm": "sha256", "intended_usage": usage}, asset.ID, &session); err != nil {
			return "", err
		}
		if session.PartSize <= 0 || session.PartSize > 8<<20 || session.Total != v.Size || session.Ref == "" {
			return "", code("creative_invalid_response")
		}
		base += "/" + url.PathEscape(session.Ref)
		for remaining, part := v.Size, 1; remaining > 0; part++ {
			size := min(remaining, session.PartSize)
			buf := make([]byte, size)
			if _, err = io.ReadFull(body, buf); err != nil {
				return "", code("transfer_failed")
			}
			sum := sha256.Sum256(buf)
			req, err := http.NewRequestWithContext(ctx, "PUT", secret.URL+base+"/parts/"+strconv.Itoa(part), bytes.NewReader(buf))
			if err != nil {
				return "", code("transfer_failed")
			}
			req.Header.Set("Authorization", "Bearer "+secret.Token)
			req.Header.Set("Content-Digest", "sha256="+hex.EncodeToString(sum[:]))
			req.Header.Set("Idempotency-Key", asset.ID+"-"+strconv.Itoa(part))
			tr := &http.Transport{Proxy: nil, DialContext: safeDial(s.Config.Development)}
			resp, err := (&http.Client{Transport: tr, Timeout: time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
			tr.CloseIdleConnections()
			if err != nil {
				return "", code("creative_transport_unknown")
			}
			resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return "", code("creative_owner_rejected")
			}
			remaining -= size
		}
		var complete struct {
			Ref string `json:"upload_ref"`
		}
		if err = s.creativeJSON(ctx, secret, "POST", base+":complete", map[string]string{"session_ref": session.Ref, "full_digest": v.SHA256}, asset.ID+"-complete", &complete); err != nil {
			return "", err
		}
		if !strings.HasPrefix(complete.Ref, "upload:") {
			return "", code("creative_invalid_response")
		}
		asset.Ref = complete.Ref
	}
	if s.DB.Save(&asset).Error != nil {
		return "", code("database_unavailable")
	}
	return asset.Ref, nil
}
