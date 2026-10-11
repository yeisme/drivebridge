package service

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Share struct {
	CreateKey   *string   `json:"-" gorm:"uniqueIndex:share_create_key"`
	Fingerprint string    `json:"-"`
	ActorID     string    `json:"actor_id,omitempty"`
	ID          string    `json:"id" gorm:"primaryKey"`
	Owner       string    `json:"-" gorm:"index;uniqueIndex:share_create_key"`
	Name        string    `json:"name"`
	SpaceID     string    `json:"space_id"`
	Path        string    `json:"path"`
	Mode        string    `json:"mode"`
	Audience    string    `json:"audience" gorm:"index"`
	Recipients  string    `json:"-"`
	Download    bool      `json:"allow_download"`
	Expires     time.Time `json:"expires_at" gorm:"index"`
	Revoked     bool      `json:"revoked" gorm:"index"`
	// Revision is the share's monotonic optimistic counter: creation sets
	// 1 and every member/permission change (revocation, membership edits)
	// advances it so expected_share_revision guards detect stale views.
	Revision         uint64            `json:"revision"`
	Created          time.Time         `json:"created_at" gorm:"index"`
	TokenHash        string            `json:"-"`
	Secret           []byte            `json:"-"`
	Passcode         []byte            `json:"-"`
	PresentationJSON string            `json:"-"`
	Presentation     SharePresentation `json:"presentation,omitempty" gorm:"-"`
	IsPermanent      bool              `json:"is_permanent" gorm:"-"`
	Failures         int               `json:"-"`
	LockedUntil      time.Time         `json:"-"`
	MaxOpens         *int64            `json:"max_opens,omitempty"`
	MaxDownloads     *int64            `json:"max_downloads,omitempty"`
	SingleUse        bool              `json:"single_use" gorm:"not null;default:false"`
	OpensUsed        int64             `json:"opens_used" gorm:"not null;default:0"`
	DownloadsUsed    int64             `json:"downloads_used" gorm:"not null;default:0"`
	ProofFailures    int               `json:"-" gorm:"not null;default:0"`
	ProofLockedUntil time.Time         `json:"-"`
	// Existing rows default false; every newly created share requires an
	// opaque session so its link secret cannot bypass complete opening.
	SessionRequired bool `json:"-" gorm:"not null;default:false"`
}
type SharePresentation struct {
	Preset         string `json:"preset,omitempty"`
	Font           string `json:"font,omitempty"`
	AccentColor    string `json:"accent_color,omitempty"`
	LogoRef        string `json:"logo_ref,omitempty"`
	WelcomeMessage string `json:"welcome_message,omitempty"`
}
type ShareItem struct {
	ShareID   string `gorm:"primaryKey" json:"-"`
	FileID    string `gorm:"primaryKey" json:"file_id"`
	VersionID string `json:"version_id"`
	Path      string `json:"path"`
}
type ShareRequest struct {
	Name         string            `json:"name"`
	SpaceID      string            `json:"space_id"`
	Path         string            `json:"path"`
	Mode         string            `json:"mode"`
	Audience     string            `json:"audience"`
	Recipients   []string          `json:"recipients"`
	Download     bool              `json:"allow_download"`
	Days         int               `json:"days"`
	Passcode     string            `json:"passcode"`
	Presentation SharePresentation `json:"presentation"`
	Items        []ShareItem       `json:"items"`
	MaxOpens     *int64            `json:"max_opens,omitempty"`
	MaxDownloads *int64            `json:"max_downloads,omitempty"`
	SingleUse    bool              `json:"single_use,omitempty"`
}

func (s *Share) hydratePresentation() {
	s.IsPermanent = s.Expires.IsZero()
	if s.PresentationJSON == "" {
		s.Presentation = SharePresentation{}
		return
	}
	_ = json.Unmarshal([]byte(s.PresentationJSON), &s.Presentation)
}

func (s *Share) AfterFind(_ *gorm.DB) error { s.hydratePresentation(); return nil }

func validPresentation(p SharePresentation) bool {
	if p.Preset != "" && p.Preset != "default" && p.Preset != "dark" && p.Preset != "brand" {
		return false
	}
	if p.Font != "" && p.Font != "system" && p.Font != "serif" && p.Font != "rounded" {
		return false
	}
	if p.AccentColor != "" {
		if len(p.AccentColor) != 7 || p.AccentColor[0] != '#' {
			return false
		}
		if _, err := hex.DecodeString(p.AccentColor[1:]); err != nil {
			return false
		}
	}
	return len(p.LogoRef) <= 240 && len(p.WelcomeMessage) <= 240
}

func shareActive(v Share) bool {
	return !v.Revoked && (v.Expires.IsZero() || v.Expires.After(time.Now()))
}

func (s *Service) shareRoutes(mux *http.ServeMux) {
	s.shareReceiverRoutes(mux)
	mux.HandleFunc("GET /api/v1/shares", s.wrap("shares.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		out := []Share{}
		if s.DB.Where("owner = ?", p.dataOwner()).Order("created DESC").Find(&out).Error != nil {
			return code("database_unavailable")
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/share-recipients", s.wrap("shares.recipients", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		if s.governanceActive() {
			if _, e := s.membership(r.URL.Query().Get("id"), p.TenantID); e != nil {
				return code("not_found")
			}
		}
		// Resolve only explicitly supplied identities; never expose the user directory.
		id := r.URL.Query().Get("id")
		var u User
		if id == "" || s.DB.First(&u, "id = ?", id).Error != nil {
			return code("not_found")
		}
		respond(w, map[string]string{"id": u.ID, "name": u.Name})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shares", s.wrap("shares.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ShareRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		out, secret, err := s.createShare(p, q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, map[string]any{"share": out, "link": s.Config.Origin + "/#share/" + out.ID + "/" + secret})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shares/{id}/link", s.wrap("shares.link", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var v Share
		if s.DB.First(&v, "id = ? AND owner = ? AND revoked = ?", r.PathValue("id"), p.dataOwner(), false).Error != nil {
			return code("not_found")
		}
		var secret string
		if err := s.unseal(p.dataOwner(), v.Secret, &secret); err != nil {
			return err
		}
		respond(w, map[string]string{"link": s.Config.Origin + "/#share/" + v.ID + "/" + secret})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shares/{id}/revoke", s.wrap("shares.revoke", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		rst := s.DB.Model(&Share{}).Where("id = ? AND owner = ?", r.PathValue("id"), p.dataOwner()).Updates(map[string]any{"revoked": true, "revision": gorm.Expr("revision + 1")})
		if rst.Error != nil {
			return code("database_unavailable")
		}
		if rst.RowsAffected == 0 {
			return code("not_found")
		}
		respond(w, map[string]bool{"revoked": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shared/{id}/open", s.openShare)
	mux.HandleFunc("GET /api/v1/shared/{id}", func(w http.ResponseWriter, r *http.Request) {
		share, err := s.authorizeShare(r)
		if err != nil {
			writeError(w, err)
			return
		}
		items, err := s.shareFiles(share)
		if err != nil {
			writeError(w, err)
			return
		}
		share.hydratePresentation()
		respond(w, map[string]any{"name": share.Name, "mode": share.Mode, "allow_download": share.Download, "expires_at": share.Expires, "is_permanent": share.IsPermanent, "presentation": share.Presentation, "files": items})
	})
	mux.HandleFunc("GET /api/v1/shared/{id}/files/{file}/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		share, err := s.authorizeShare(r)
		if err != nil {
			writeError(w, err)
			return
		}
		files, err := s.shareFiles(share)
		if err != nil {
			writeError(w, err)
			return
		}
		for _, file := range files {
			if file.ID == r.PathValue("file") {
				var preview Preview
				if s.DB.First(&preview, "owner = ? AND file_id = ? AND version_id = ? AND kind = ? AND status = ?", share.Owner, file.ID, file.VersionID, "thumbnail", "ready").Error != nil {
					writeError(w, code("preview_not_ready"))
					return
				}
				if err = s.servePreview(w, r, preview); err != nil {
					writeError(w, err)
				}
				return
			}
		}
		writeError(w, code("not_found"))
	})
	mux.HandleFunc("GET /api/v1/shared/{id}/files/{file}/content", func(w http.ResponseWriter, r *http.Request) {
		share, err := s.authorizeShare(r)
		if err != nil {
			writeError(w, err)
			return
		}
		items, err := s.shareFiles(share)
		if err != nil {
			writeError(w, err)
			return
		}
		for _, f := range items {
			if f.ID == r.PathValue("file") {
				attachment := r.URL.Query().Get("download") == "true"
				if attachment && !share.Download {
					writeError(w, code("forbidden"))
					return
				}
				if err = s.serveShareReceiverContent(w, r, share, f, attachment); err != nil {
					writeError(w, err)
				}
				return
			}
		}
		writeError(w, code("not_found"))
	})
}

func (s *Service) createShare(p Principal, q ShareRequest, keys ...string) (Share, string, error) {
	if s.governanceActive() {
		if err := s.allow(p, q.SpaceID, q.Path, "share"); err != nil {
			return Share{}, "", err
		}
		for _, u := range q.Recipients {
			if _, err := s.membership(u, p.TenantID); err != nil {
				return Share{}, "", err
			}
		}
	}
	if p.Agent != nil {
		return Share{}, "", code("forbidden")
	}
	if q.Mode != "fixed" && q.Mode != "live" || q.Audience != "link" && q.Audience != "users" || q.Days < 0 || q.Days > 365 || len(q.Name) > 120 || q.Name == "" || len(q.Passcode) > 72 || len(q.Passcode) > 0 && len(q.Passcode) < 6 || !validPresentation(q.Presentation) {
		return Share{}, "", code("invalid_share")
	}
	if (q.MaxOpens != nil || q.MaxDownloads != nil || q.SingleUse) && !s.Config.ShareReceivers {
		return Share{}, "", code("unsupported_action")
	}
	if q.MaxOpens != nil && *q.MaxOpens < 1 || q.MaxDownloads != nil && *q.MaxDownloads < 1 {
		return Share{}, "", code("invalid_share")
	}
	if q.SingleUse {
		one := int64(1)
		q.MaxOpens, q.MaxDownloads = &one, &one
	}
	if _, err := s.space(p, q.SpaceID); err != nil {
		return Share{}, "", err
	}
	if q.Path != "" {
		if _, err := cleanPath(q.Path); err != nil {
			return Share{}, "", err
		}
	}
	if q.Audience == "users" {
		if len(q.Recipients) == 0 || len(q.Recipients) > 100 {
			return Share{}, "", code("invalid_share")
		}
		for _, id := range q.Recipients {
			var n int64
			if s.DB.Model(&User{}).Where("id = ?", id).Count(&n).Error != nil || n != 1 {
				return Share{}, "", code("invalid_recipient")
			}
		}
	}
	secret := ID()
	expires := time.Time{}
	if q.Days > 0 {
		expires = time.Now().UTC().Add(time.Duration(q.Days) * 24 * time.Hour)
	}
	presentation, _ := json.Marshal(q.Presentation)
	v := Share{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), Name: q.Name, SpaceID: q.SpaceID, Path: q.Path, Mode: q.Mode, Audience: q.Audience, Download: q.Download, Recipients: strings.Join(q.Recipients, ","), Revision: 1, Created: time.Now().UTC(), Expires: expires, PresentationJSON: string(presentation), TokenHash: Hash(secret)}
	v.MaxOpens, v.MaxDownloads, v.SingleUse = q.MaxOpens, q.MaxDownloads, q.SingleUse
	v.SessionRequired = true
	v.hydratePresentation()
	var err error
	v.Secret, err = s.seal(p.dataOwner(), secret)
	if err != nil {
		return v, "", err
	}
	if q.Passcode != "" {
		v.Passcode, err = bcrypt.GenerateFromPassword([]byte(q.Passcode), bcrypt.DefaultCost)
		if err != nil {
			return v, "", err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(keys) > 0 && keys[0] != "" {
		if len(keys[0]) > 200 {
			return v, "", code("invalid_share")
		}
		body, _ := json.Marshal(q)
		v.Fingerprint = Hash(string(body))
		key := keys[0]
		v.CreateKey = &key
		var previous Share
		if s.DB.First(&previous, "owner = ? AND create_key = ?", p.dataOwner(), key).Error == nil {
			if previous.Fingerprint != v.Fingerprint {
				return v, "", code("idempotency_conflict")
			}
			var previousSecret string
			if err = s.unseal(p.dataOwner(), previous.Secret, &previousSecret); err != nil {
				return v, "", err
			}
			return previous, previousSecret, nil
		}
	}
	items := []ShareItem{}
	if q.Mode == "fixed" {
		if len(q.Items) == 0 {
			var files []File
			if s.DB.Where("owner = ? AND space_id = ? AND trashed = ? AND review_pending = ?", p.dataOwner(), q.SpaceID, false, false).Find(&files).Error != nil {
				return v, "", code("database_unavailable")
			}
			for _, f := range files {
				if contains(q.Path, f.Path) {
					q.Items = append(q.Items, ShareItem{FileID: f.ID, VersionID: f.VersionID})
				}
			}
		}
		if len(q.Items) == 0 || len(q.Items) > 5000 {
			return v, "", code("invalid_share_selection")
		}
		seen := map[string]bool{}
		for _, item := range q.Items {
			f, err := s.file(p, item.FileID, "read")
			if err != nil {
				return v, "", err
			}
			if f.SpaceID != q.SpaceID || f.Trashed || f.ReviewPending || !contains(q.Path, f.Path) || seen[f.ID] {
				return v, "", code("invalid_share_selection")
			}
			seen[f.ID] = true
			var version Version
			if s.DB.First(&version, "id = ? AND file_id = ? AND owner = ?", item.VersionID, f.ID, p.dataOwner()).Error != nil {
				return v, "", code("not_found")
			}
			items = append(items, ShareItem{ShareID: v.ID, FileID: f.ID, VersionID: version.ID, Path: f.Path})
		}
	}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, q.SpaceID); e != nil {
			return e
		}
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
	return v, secret, err
}

func (s *Service) shareFiles(v Share) ([]File, error) {
	out := []File{}
	if v.Mode == "live" {
		var files []File
		if s.DB.Where("owner = ? AND space_id = ? AND trashed = ? AND review_pending = ?", v.Owner, v.SpaceID, false, false).Order("path").Find(&files).Error != nil {
			return nil, code("database_unavailable")
		}
		for _, f := range files {
			if contains(v.Path, f.Path) {
				out = append(out, f)
			}
		}
	} else {
		var items []ShareItem
		if s.DB.Where("share_id = ?", v.ID).Order("path").Find(&items).Error != nil {
			return nil, code("database_unavailable")
		}
		for _, item := range items {
			var f File
			var version Version
			if s.DB.First(&f, "id = ? AND owner = ? AND trashed = ? AND review_pending = ?", item.FileID, v.Owner, false, false).Error != nil {
				continue
			}
			if s.DB.First(&version, "id = ? AND file_id = ? AND owner = ?", item.VersionID, f.ID, v.Owner).Error != nil {
				continue
			}
			f.Path, f.VersionID, f.Size, f.MIME = item.Path, version.ID, version.Size, version.MIME
			out = append(out, f)
		}
	}
	return out, nil
}
func shareCookie(id string) string { return "db_share_" + id }
func (s *Service) openShare(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled {
		writeError(w, code("disabled"))
		return
	}
	if r.Header.Get("Origin") != s.Config.Origin {
		writeError(w, code("csrf_rejected"))
		return
	}
	var q struct {
		Grant    string `json:"grant"`
		Passcode string `json:"passcode"`
	}
	if decodeBody := http.MaxBytesReader(w, r.Body, 4096); decodeBody != nil {
		r.Body = decodeBody
	}
	if decode(r, &q) != nil {
		writeError(w, code("invalid_share"))
		return
	}
	p, err := s.optionalSharePrincipal(r)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := s.openShareReceiver(p, r.PathValue("id"), "grant", q.Grant, ShareProofRequest{Passcode: q.Passcode}, ShareDeviceOpenRequest{})
	if err != nil {
		writeError(w, err)
		return
	}
	s.respondShareOpen(w, r.PathValue("id"), out)
}
func containsRecipient(v Share, user string) bool {
	return user == v.Owner || strings.Contains(","+v.Recipients+",", ","+user+",")
}
func (s *Service) authorizeShare(r *http.Request) (Share, error) {
	var v Share
	if !s.Config.Enabled {
		return v, code("disabled")
	}
	if s.DB.First(&v, "id = ? AND revoked = ?", r.PathValue("id"), false).Error != nil || !shareActive(v) {
		return v, code("share_unavailable")
	}
	if s.governanceActive() {
		var sp Space
		if s.DB.First(&sp, "id = ?", v.SpaceID).Error != nil || s.scopeRole(s.sharePrincipal(v), sp) == "" {
			return v, code("share_unavailable")
		}
	}
	c, err := r.Cookie(shareCookie(v.ID))
	if err != nil {
		return v, code("unauthorized")
	}
	session, _, err := s.shareReceiverSession(s.DB, v, c.Value)
	if err != nil {
		return v, err
	}
	if session.Kind == "device" || v.Audience == "users" {
		p, err := s.optionalSharePrincipal(r)
		if err != nil || p.Agent != nil || p.ServiceIdentity != nil {
			return v, code("forbidden")
		}
		if v.Audience == "users" && (p.UserID == "" || !containsRecipient(v, p.UserID) || session.UserID != "" && session.UserID != p.UserID) {
			return v, code("forbidden")
		}
	}
	return v, nil
}

func (s *Service) fileUse(p Principal, f File) (map[string]any, error) {
	shares := []Share{}
	if s.DB.Where("owner = ? AND revoked = ?", p.dataOwner(), false).Find(&shares).Error != nil {
		return nil, code("database_unavailable")
	}
	activeShares := shares[:0]
	for _, share := range shares {
		if shareActive(share) {
			activeShares = append(activeShares, share)
		}
	}
	shares = activeShares
	ids := []string{}
	for _, v := range shares {
		if v.Mode == "live" && v.SpaceID == f.SpaceID && contains(v.Path, f.Path) {
			ids = append(ids, v.ID)
		} else {
			var n int64
			if s.DB.Model(&ShareItem{}).Where("share_id = ? AND file_id = ?", v.ID, f.ID).Count(&n).Error != nil {
				return nil, code("database_unavailable")
			}
			if n > 0 {
				ids = append(ids, v.ID)
			}
		}
	}
	var active int64
	if s.DB.Model(&HandoffGrant{}).Where("owner = ? AND file_id = ? AND expires > ?", p.dataOwner(), f.ID, time.Now()).Count(&active).Error != nil {
		return nil, code("database_unavailable")
	}
	var operations int64
	if s.DB.Model(&Operation{}).Where("owner = ? AND (file_id = ? OR archive_file_id = ?) AND status NOT IN ?", p.dataOwner(), f.ID, f.ID, []string{"completed", "cancelled", "failed"}).Count(&operations).Error != nil {
		return nil, code("database_unavailable")
	}
	var receipts []CreativeReceipt
	if s.DB.Where("owner = ? AND status NOT IN ?", p.UserID, []string{"archived", "owner_terminal"}).Find(&receipts).Error != nil {
		return nil, code("database_unavailable")
	}
	for _, receipt := range receipts {
		var inputs []CreativeInput
		if json.Unmarshal(receipt.Inputs, &inputs) != nil {
			return nil, code("database_unavailable")
		}
		for _, input := range inputs {
			if input.FileID == f.ID {
				active++
			}
		}
	}
	return map[string]any{"active_shares": ids, "active_references": active + operations, "permanent_delete_allowed": len(ids) == 0 && active+operations == 0, "history_will_be_unavailable": true}, nil
}

func (s *Service) sharePrincipal(v Share) Principal {
	p := Principal{ShareID: v.ID, ExternalRead: true, UserID: defaultString(v.ActorID, v.Owner), DataOwner: v.Owner, ScopeSpace: v.SpaceID}
	var sp Space
	if s.DB.First(&sp, "id = ?", v.SpaceID).Error == nil {
		p.TenantID = sp.TenantID
	}
	return p
}
