package bridge

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/sdk"
)

type SharePolicy struct {
	Audience   string   `json:"audience"`
	Recipients []string `json:"recipients"`
	Download   bool     `json:"allow_download"`
	Passcode   string   `json:"passcode"`
}

type ShareCopy struct {
	Policy      []byte    `json:"-"`
	ID          string    `gorm:"primaryKey" json:"id"`
	SpaceID     string    `json:"space_id"`
	Dir         string    `json:"dir"`
	Refs        []string  `gorm:"serializer:json" json:"-"`
	TargetSpace string    `json:"target_space"`
	Mode        string    `json:"mode"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	RemoteID    string    `json:"remote_id,omitempty"`
	Link        []byte    `json:"-"`
	Days        int       `json:"days"`
	Expires     time.Time `json:"expires_at"`
	Updated     time.Time `json:"updated_at"`
}
type ShareReplica struct {
	ID        string `gorm:"primaryKey"`
	ShareID   string `gorm:"index"`
	SourceID  string
	SourceRef string
	FileID    string
	VersionID string
}
type ShareDestination struct {
	Actor string `json:"actor,omitempty"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

func (s *Service) shareCipher(create bool) (cipher.AEAD, error) {
	keyPath := filepath.Join(s.state, "remote-share.key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) && create {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, err = f.Write(key)
		f.Close()
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("share_credentials_unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Service) sealShare(data []byte) ([]byte, error) {
	aead, err := s.shareCipher(true)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, data, []byte("drivebridge-remote-share")), nil
}
func (s *Service) openShare(data []byte) ([]byte, error) {
	aead, err := s.shareCipher(false)
	if err != nil {
		return nil, err
	}
	if len(data) < aead.NonceSize() {
		return nil, errors.New("share_credentials_unavailable")
	}
	out, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte("drivebridge-remote-share"))
	if err != nil {
		return nil, errors.New("share_credentials_unavailable")
	}
	return out, nil
}
func (s *Service) shareClient() (*sdk.Client, error) {
	data, err := os.ReadFile(filepath.Join(s.state, "remote-share.connection"))
	if err != nil {
		return nil, errors.New("share_destination_unconfigured")
	}
	data, err = s.openShare(data)
	if err != nil {
		return nil, err
	}
	var dest ShareDestination
	if json.Unmarshal(data, &dest) != nil {
		return nil, errors.New("share_credentials_unavailable")
	}
	return sdk.New(dest.URL, dest.Token)
}
func (s *Service) remoteShareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /files/share", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fail := func(err error) {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": localFileError(err)}})
		}
		if !s.Enabled {
			fail(errors.New("disabled"))
			return
		}
		var q struct {
			Policy      *SharePolicy     `json:"policy"`
			Action      string           `json:"action"`
			ID          string           `json:"id"`
			Destination ShareDestination `json:"destination"`
			Share       ShareCopy        `json:"share"`
			Refs        []string         `json:"refs"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&q) != nil {
			fail(errors.New("invalid_share"))
			return
		}
		var result any
		switch q.Action {
		case "configure":
			client, err := sdk.New(q.Destination.URL, q.Destination.Token)
			if err != nil {
				fail(errors.New("invalid_share_destination"))
				return
			}
			var me struct {
				Agent any `json:"agent"`
				User  struct {
					ID string `json:"id"`
				} `json:"user"`
			}
			if err = client.JSON(r.Context(), "GET", "/api/v1/me", "", nil, &me); err != nil || me.Agent != nil {
				fail(errors.New("share_user_credential_required"))
				return
			}
			q.Destination.Actor = me.User.ID
			if oldBytes, readErr := os.ReadFile(filepath.Join(s.state, "remote-share.connection")); readErr == nil {
				if plain, openErr := s.openShare(oldBytes); openErr == nil {
					var old ShareDestination
					if json.Unmarshal(plain, &old) == nil && (old.URL != q.Destination.URL || old.Actor != q.Destination.Actor) {
						var active int64
						if s.DB.Model(&ShareCopy{}).Where("status NOT IN ?", []string{"revoked", "expired"}).Count(&active).Error != nil || active > 0 {
							fail(errors.New("active_share_destination_immutable"))
							return
						}
					}
				}
			}
			b, _ := json.Marshal(q.Destination)
			sealed, err := s.sealShare(b)
			if err != nil {
				fail(err)
				return
			}
			if err = os.WriteFile(filepath.Join(s.state, "remote-share.connection"), sealed, 0600); err != nil {
				fail(errors.New("local_write_failed"))
				return
			}
			result = map[string]bool{"configured": true}
		case "spaces":
			client, err := s.shareClient()
			if err != nil {
				fail(err)
				return
			}
			var spaces []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err = client.JSON(r.Context(), "GET", "/api/v1/spaces", "", nil, &spaces); err != nil {
				fail(err)
				return
			}
			result = spaces
		case "list":
			out := []ShareCopy{}
			if s.DB.Order("updated DESC").Limit(200).Find(&out).Error != nil {
				fail(errors.New("state_unavailable"))
				return
			}
			result = out
		case "create":
			if _, err := s.shareClient(); err != nil {
				fail(err)
				return
			}
			v := q.Share
			if _, err := s.Space(v.SpaceID); err != nil {
				fail(err)
				return
			}
			dir, err := remoteDir(v.Dir)
			if err != nil {
				fail(err)
				return
			}
			if v.Mode != "fixed" && v.Mode != "live" || v.Days < 1 || v.Days > 365 || v.TargetSpace == "" || v.Name == "" || len(v.Name) > 120 || len(q.Refs) > 5000 || v.Mode == "live" && len(q.Refs) > 0 {
				fail(errors.New("invalid_share"))
				return
			}
			// Fixed selections are pinned now; a full-directory copy captures its listing before transfer.
			v.ID, v.Dir, v.Refs, v.Status, v.Error, v.RemoteID, v.Link, v.Updated = newID(), dir, q.Refs, "queued", "", "", nil, time.Now().UTC()
			v.Expires = v.Updated.Add(time.Duration(v.Days) * 24 * time.Hour)
			if len(v.Refs) > 0 {
				for _, ref := range v.Refs {
					f, err := s.Stat(r.Context(), ref)
					if err != nil || f.SpaceID != v.SpaceID {
						fail(errors.New("reference_not_found"))
						return
					}
				}
			}
			policy := SharePolicy{Audience: "link", Download: true}
			if q.Policy != nil {
				policy = *q.Policy
			}
			if policy.Audience != "link" && policy.Audience != "users" || policy.Audience == "users" && len(policy.Recipients) == 0 || len(policy.Passcode) > 72 || len(policy.Passcode) > 0 && len(policy.Passcode) < 6 {
				fail(errors.New("invalid_share"))
				return
			}
			policyBytes, _ := json.Marshal(policy)
			v.Policy, err = s.sealShare(policyBytes)
			if err != nil {
				fail(err)
				return
			}
			if s.DB.Create(&v).Error != nil {
				fail(errors.New("state_unavailable"))
				return
			}
			result = v
		case "retry", "revoke", "link":
			var v ShareCopy
			if s.DB.First(&v, "id = ?", q.ID).Error != nil {
				fail(errors.New("share_not_found"))
				return
			}
			if q.Action == "link" {
				if (v.Status == "revoked" || v.Status == "revocation_pending") || time.Now().After(v.Expires) {
					fail(errors.New("share_unavailable"))
					return
				}
				link, err := s.openShare(v.Link)
				if err != nil {
					fail(err)
					return
				}
				result = map[string]string{"link": string(link)}
			} else if q.Action == "retry" {
				s.DB.Model(&v).Updates(map[string]any{"status": "queued", "error": ""})
				result = map[string]bool{"queued": true}
			} else {
				if s.DB.Model(&v).Update("status", "revocation_pending").Error != nil {
					fail(errors.New("state_unavailable"))
					return
				}
				if v.RemoteID != "" {
					client, err := s.shareClient()
					if err != nil {
						fail(err)
						return
					}
					if err = client.JSON(r.Context(), "POST", "/api/v1/shares/"+url.PathEscape(v.RemoteID)+"/revoke", "", map[string]any{}, nil); err != nil {
						fail(err)
						return
					}
				}
				s.DB.Model(&v).Update("status", "revoked")
				result = map[string]bool{"revoked": true}
			}
		default:
			fail(errors.New("invalid_share_action"))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": result})
	})
}
func (s *Service) shareFiles(ctx context.Context, v ShareCopy) ([]File, error) {
	files := []File{}
	if len(v.Refs) > 0 {
		for _, ref := range v.Refs {
			f, err := s.Stat(ctx, ref)
			if err != nil {
				return nil, err
			}
			if f.SpaceID != v.SpaceID || f.Trashed != nil {
				return nil, errors.New("reference_not_found")
			}
			files = append(files, f)
		}
		return files, nil
	}
	pending := []string{v.Dir}
	visited := 0
	for len(pending) > 0 {
		dir := pending[0]
		pending = pending[1:]
		visited++
		if visited > 5000 {
			return nil, errors.New("share_listing_limit")
		}
		entries, err := s.listEntries(ctx, v.SpaceID, dir, true)
		if err != nil {
			return nil, err
		}
		for _, f := range entries {
			if f.Name == trashRoot || strings.HasPrefix(f.Name, ".drivebridge-") {
				continue
			}
			if f.Directory {
				pending = append(pending, path.Join(dir, f.Name))
			} else if f.Trashed == nil && f.Caller != "eikona" && f.Caller != "scaena" {
				files = append(files, f)
				if len(files) > 5000 {
					return nil, errors.New("share_listing_limit")
				}
			}
		}
	}
	return files, nil
}
func (s *Service) syncShare(ctx context.Context, v ShareCopy) error {
	client, err := s.shareClient()
	if err != nil {
		return err
	}
	files, err := s.shareFiles(ctx, v)
	if err != nil {
		return err
	}
	if len(files) == 0 && v.RemoteID == "" {
		return errors.New("share_empty")
	}
	if v.Mode == "fixed" && len(v.Refs) == 0 {
		for _, f := range files {
			v.Refs = append(v.Refs, f.Ref)
		}
		if s.DB.Model(&v).Select("refs").Updates(&v).Error != nil {
			return errors.New("state_unavailable")
		}
	}
	root := "Shared/" + v.ID
	seen := map[string]bool{}
	for _, f := range files {
		sourceID := f.DriveID + ":" + f.ItemID
		seen[sourceID] = true
		var previous ShareReplica
		s.DB.First(&previous, "share_id = ? AND source_id = ?", v.ID, sourceID)
		if previous.SourceRef == f.Ref {
			continue
		}
		p, err := s.Space(f.SpaceID)
		if err != nil {
			return err
		}
		// Reopen pinned bytes for hash and missing parts; no plaintext staging copies.
		source := &localSeeker{size: f.Size, open: func() (io.ReadCloser, error) {
			if _, err := s.Stat(ctx, f.Ref); err != nil {
				return nil, err
			}
			return s.providerFor(p).Open(ctx, p, f)
		}}
		q := sdk.UploadRequest{SpaceID: v.TargetSpace, Path: path.Join(root, f.Dir, f.Name), FileID: previous.FileID, ExpectedVersion: previous.VersionID, Caller: "drivebridge-share"}
		op, err := client.Upload(ctx, q, "share:"+v.ID+":"+f.Ref, source, nil)
		source.close()
		if err != nil {
			return err
		}
		if previous.ID == "" {
			previous = ShareReplica{ID: newID(), ShareID: v.ID, SourceID: sourceID}
		}
		previous.SourceRef, previous.FileID, previous.VersionID = f.Ref, op.FileID, op.VersionID
		if s.DB.Save(&previous).Error != nil {
			return errors.New("state_unavailable")
		}
	}
	var replicas []ShareReplica
	if s.DB.Where("share_id = ?", v.ID).Find(&replicas).Error != nil {
		return errors.New("state_unavailable")
	}
	if v.Mode == "live" {
		for _, replica := range replicas {
			if !seen[replica.SourceID] {
				if err := client.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(replica.FileID)+"/trash", "", map[string]any{}, nil); err != nil {
					return err
				}
				s.DB.Delete(&replica)
			}
		}
	}
	if v.RemoteID == "" {
		var result struct {
			Share struct {
				ID string `json:"id"`
			} `json:"share"`
			Link string `json:"link"`
		}
		policy := SharePolicy{Audience: "link", Download: true}
		if len(v.Policy) > 0 {
			plain, err := s.openShare(v.Policy)
			if err != nil {
				return err
			}
			if json.Unmarshal(plain, &policy) != nil {
				return errors.New("share_credentials_unavailable")
			}
		}
		if err = client.JSON(ctx, "POST", "/api/v1/shares", "local-share:"+v.ID, map[string]any{"name": v.Name, "space_id": v.TargetSpace, "path": root, "mode": v.Mode, "audience": policy.Audience, "recipients": policy.Recipients, "passcode": policy.Passcode, "days": v.Days, "allow_download": policy.Download}, &result); err != nil {
			return err
		}
		if result.Share.ID == "" || result.Link == "" {
			return errors.New("share_invalid_response")
		}
		sealed, err := s.sealShare([]byte(result.Link))
		if err != nil {
			return err
		}
		update := s.DB.Model(&v).Where("status NOT IN ?", []string{"revoked", "revocation_pending"}).Updates(map[string]any{"remote_id": result.Share.ID, "link": sealed})
		if update.RowsAffected == 0 {
			_ = client.JSON(ctx, "POST", "/api/v1/shares/"+url.PathEscape(result.Share.ID)+"/revoke", "", map[string]any{}, nil)
			return errors.New("share_revoked")
		}
		if update.Error != nil {
			return errors.New("state_unavailable")
		}
	}
	return nil
}
func (s *Service) startRemoteShares() {
	s.shareOnce.Do(func() {
		s.shareWG.Add(1)
		go func() {
			defer s.shareWG.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.shareCtx.Done():
					return
				case <-ticker.C:
					var shares []ShareCopy
					if s.DB.Where("status = ? OR (mode = ? AND status = ? AND updated < ?)", "queued", "live", "ready", time.Now().Add(-30*time.Second)).Find(&shares).Error != nil {
						continue
					}
					for _, v := range shares {
						if time.Now().After(v.Expires) {
							s.DB.Model(&v).Update("status", "expired")
							continue
						}
						s.DB.Model(&v).Update("status", "syncing")
						ctx, cancel := context.WithTimeout(s.shareCtx, 30*time.Minute)
						err := s.syncShare(ctx, v)
						cancel()
						status, message := "ready", ""
						if err != nil {
							status, message = "failed", localFileError(err)
						}
						s.DB.Model(&v).Where("status NOT IN ?", []string{"revoked", "revocation_pending"}).Updates(map[string]any{"status": status, "error": message, "updated": time.Now().UTC()})
					}
				}
			}
		}()
	})
}
