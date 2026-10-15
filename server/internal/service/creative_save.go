package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type creativeStatus struct {
	State        string   `json:"state"`
	Artifacts    []string `json:"artifacts"`
	ReviewStatus string   `json:"review_status"`
}

func (s *Service) readCreativeStatus(ctx context.Context, receipt CreativeReceipt, conn CreativeConnection, secret creativeSecret) (creativeStatus, error) {
	target := "/api/v1/runs/" + url.PathEscape(receipt.TaskRef)
	if conn.Kind == "scaena" {
		target = "/api/v1/projects/" + url.PathEscape(conn.Project) + "/video-tasks/" + url.PathEscape(receipt.TaskRef)
	}
	var result struct {
		Status       string   `json:"status"`
		State        string   `json:"state"`
		ResourceURIs []string `json:"resource_uris"`
		ArtifactRef  string   `json:"artifact_ref"`
		AssetRef     string   `json:"asset_ref"`
		ReviewStatus string   `json:"review_status"`
	}
	if err := s.creativeJSON(ctx, secret, "GET", target, nil, "", &result); err != nil {
		return creativeStatus{}, err
	}
	state := result.State
	if state == "" {
		state = result.Status
	}
	refs := []string{}
	for _, ref := range append(result.ResourceURIs, result.ArtifactRef, result.AssetRef) {
		if len(ref) <= 512 && !strings.ContainsAny(ref, "?#\\\r\n") && (strings.HasPrefix(ref, "eikona://artifact/") || strings.HasPrefix(ref, "asset:video-provider-task:")) {
			refs = append(refs, ref)
		}
	}
	return creativeStatus{state, refs, result.ReviewStatus}, nil
}
func (s *Service) creativeSaveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/creative/receipts/{id}/save", s.wrap("creative.save", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Ref string `json:"artifact_ref"`
		}
		if err := decode(r, &q); err != nil {
			return err
		}
		receipt, conn, secret, err := s.creativeReceipt(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		status, err := s.readCreativeStatus(r.Context(), receipt, conn, secret)
		if err != nil {
			return err
		}
		allowed := false
		for _, ref := range status.Artifacts {
			if ref == q.Ref {
				allowed = true
			}
		}
		if !allowed {
			return code("forbidden")
		}
		op, err := s.saveCreativeArtifact(r.Context(), p, receipt, conn, secret, q.Ref)
		if err != nil {
			return err
		}
		respond(w, op)
		return nil
	}))
}
func (s *Service) saveCreativeArtifact(ctx context.Context, p Principal, receipt CreativeReceipt, conn CreativeConnection, secret creativeSecret, ref string) (Operation, error) {
	key := "creative-save:" + receipt.ID + ":" + Hash(ref)
	// Reuse completed or recoverable transfers, independently of generation.
	var existing Operation
	if s.DB.First(&existing, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
		if existing.Status == "completed" {
			return existing, nil
		}
		if existing.Transferred == existing.Size {
			return s.Complete(ctx, p, existing.ID)
		}
	}
	var grant struct {
		URL          string    `json:"download_url"`
		ResourceLink string    `json:"resource_link"`
		GrantRef     string    `json:"grant_ref"`
		Expires      time.Time `json:"expires_at"`
		MIME         string    `json:"media_type"`
		Digest       string    `json:"digest"`
	}
	target := "/api/v1/artifacts/" + url.PathEscape(strings.TrimPrefix(ref, "eikona://artifact/")) + "/download-grants"
	request := map[string]any{"consumer": "drivebridge", "confirm_rights": true}
	if conn.Kind == "scaena" {
		target = "/api/v1/projects/" + url.PathEscape(conn.Project) + "/artifacts/" + url.PathEscape(ref) + "/access-grants"
		request = map[string]any{"ttl_seconds": 300}
	}
	if err := s.creativeJSON(ctx, secret, "POST", target, request, key, &grant); err != nil {
		return Operation{}, err
	}
	raw := grant.URL
	if raw == "" {
		raw = grant.ResourceLink
	}
	if conn.Kind == "scaena" && raw == "" && grant.GrantRef != "" {
		raw = target + "/" + url.PathEscape(grant.GrantRef)
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || raw == "" {
		return Operation{}, code("creative_invalid_response")
	}
	owner, _ := url.Parse(secret.URL)
	if u.IsAbs() {
		if u.Scheme != owner.Scheme || u.Host != owner.Host {
			return Operation{}, code("creative_download_transport_unsupported")
		}
	} else if !strings.HasPrefix(raw, "/api/v1/") {
		return Operation{}, code("creative_invalid_response")
	}
	if !strings.HasPrefix(u.Path, "/api/v1/") {
		return Operation{}, code("creative_invalid_response")
	}
	response, err := s.creativeHTTP(ctx, secret, "GET", u.RequestURI(), nil, -1, "")
	if err != nil {
		return Operation{}, err
	}
	defer response.Body.Close()
	sp, err := s.space(p, receipt.SpaceID)
	if err != nil {
		return Operation{}, err
	}
	dir, err := os.MkdirTemp(s.Config.State, "creative-save-")
	if err != nil {
		return Operation{}, code("disk_write_failed")
	}
	defer os.RemoveAll(dir)
	// Encrypt every staged part, including downloads for encrypted spaces.
	hash := sha256.New()
	total := int64(0)
	parts := 0
	for {
		buf := make([]byte, PartSize)
		n, readErr := io.ReadFull(response.Body, buf)
		if n > 0 {
			total += int64(n)
			if total > sp.MaxBytes {
				return Operation{}, code("quota_exceeded")
			}
			hash.Write(buf[:n])
			nonce, nonceErr := secureRandomBytes(s.crypt.NonceSize())
			if nonceErr != nil {
				return Operation{}, code("encryption_unavailable")
			}
			sealed := s.crypt.Seal(nonce, nonce, buf[:n], []byte(p.dataOwner()))
			if os.WriteFile(filepath.Join(dir, strconv.Itoa(parts)), sealed, 0600) != nil {
				return Operation{}, code("disk_write_failed")
			}
			parts++
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return Operation{}, code("transfer_failed")
		}
	}
	if total <= 0 {
		return Operation{}, code("transfer_failed")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if grant.Digest != "" && strings.TrimPrefix(grant.Digest, "sha256:") != digest {
		return Operation{}, code("content_mismatch")
	}
	m := response.Header.Get("Content-Type")
	ext := ".bin"
	if strings.HasPrefix(m, "image/png") {
		ext = ".png"
	} else if strings.HasPrefix(m, "image/jpeg") {
		ext = ".jpg"
	} else if strings.HasPrefix(m, "video/") {
		ext = ".mp4"
	} else if strings.HasPrefix(m, "image/webp") {
		ext = ".webp"
	}
	destination := strings.TrimSuffix(receipt.Path, "/") + "/result-" + receipt.ID[:8] + "-" + Hash(ref)[:8] + ext
	destination = strings.TrimPrefix(destination, "/")
	op, err := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: destination, Size: total, SHA256: digest, Caller: conn.Kind, TaskRef: "creative:" + receipt.ID, ReviewPending: true}, key)
	if err != nil {
		return op, err
	}
	if op.Status == "completed" {
		// Instant blob hit: no parts to stage.
		return op, nil
	}
	for i := 0; i < parts; i++ {
		sealed, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(i)))
		if err != nil {
			return op, code("disk_write_failed")
		}
		n := s.crypt.NonceSize()
		plain, err := s.crypt.Open(nil, sealed[:n], sealed[n:], []byte(p.dataOwner()))
		if err != nil {
			return op, code("encryption_unavailable")
		}
		op, err = s.PutPart(p, op.ID, i, strings.NewReader(string(plain)))
		if err != nil {
			return op, err
		}
	}
	return s.Complete(ctx, p, op.ID)
}
func (s *Service) startCreativeSaveback() {
	s.creativeOnce.Do(func() {
		s.previewWG.Add(1)
		go func() {
			defer s.previewWG.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.previewCtx.Done():
					return
				case <-ticker.C:
					var receipts []CreativeReceipt
					if s.DB.Where("status IN ? AND task_ref <> ''", []string{"submitted", "saveback_retry"}).Order("updated").Limit(20).Find(&receipts).Error != nil {
						continue
					}
					for _, receipt := range receipts {
						p := s.personalOwnerPrincipal(receipt.Owner)
						conn, secret, err := s.creativeConnection(p, receipt.ConnectionID)
						if err != nil {
							continue
						}
						ctx, cancel := context.WithTimeout(s.previewCtx, 5*time.Minute)
						status, err := s.readCreativeStatus(ctx, receipt, conn, secret)
						if err == nil && len(status.Artifacts) > 0 {
							saved := true
							for _, ref := range status.Artifacts {
								if _, err = s.saveCreativeArtifact(ctx, p, receipt, conn, secret, ref); err != nil {
									saved = false
									break
								}
							}
							next := "archived"
							attempts := receipt.ArchiveAttempts
							if !saved {
								attempts++
								next = "saveback_retry"
								if attempts >= 3 {
									next = "saveback_failed"
								}
							}
							s.DB.Model(&receipt).Updates(map[string]any{"status": next, "archive_attempts": attempts, "updated": time.Now().UTC()})
						} else if err == nil && (status.State == "failed" || status.State == "cancelled") {
							s.DB.Model(&receipt).Update("status", "owner_terminal")
						}
						cancel()
					}
				}
			}
		}()
	})
}
