package service

import (
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

// gdriveOAuth builds the Google Drive OAuth2 configuration. The flow is
// always user-initiated with PKCE; the callback binds the same internal
// user that started it (consumeFlow enforces the owner match).
func (s *Service) gdriveOAuth() oauth2.Config {
	return oauth2.Config{ClientID: s.Config.GoogleClientID, ClientSecret: s.Config.GoogleSecret, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}, Scopes: []string{"https://www.googleapis.com/auth/drive.readonly"}, RedirectURL: s.Config.Origin + "/auth/gdrive/callback"}
}

// finishGoogleDrive completes the OAuth callback: PKCE exchange, sealed
// credential and a verified connection row bound to the starting user.
// Agents can never land here (browser-only flow with session auth).
func (s *Service) finishGoogleDrive(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled {
		writeError(w, code("disabled"))
		return
	}
	p, e := s.authenticate(r)
	if e != nil || p.Agent != nil {
		writeError(w, code("unauthorized"))
		return
	}
	flow, e := s.consumeFlow(r, "gdrive")
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
	cfg := s.gdriveOAuth()
	token, e := cfg.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(secret.Verifier))
	if e != nil {
		writeError(w, code("gdrive_authorization_failed"))
		return
	}
	cipher, e := s.seal(p.dataOwner(), gdriveCredential{Token: *token})
	if e != nil {
		writeError(w, e)
		return
	}
	conn := Connection{ID: ID(), Owner: p.dataOwner(), Kind: "gdrive", Name: "Google Drive", Secret: cipher, Status: "connected"}
	if s.DB.Create(&conn).Error != nil {
		writeError(w, code("database_unavailable"))
		return
	}
	// Verify the credential before advertising the connection: the drive
	// scope must be readable with the granted token.
	q := url.Values{}
	q.Set("fields", "id")
	if _, e = s.gdriveCall(r.Context(), p, conn, "GET", "/drive/v3/files/root", q); e != nil {
		s.DB.Delete(&conn)
		writeError(w, e)
		return
	}
	s.DB.Model(&conn).Update("status", "verified")
	http.Redirect(w, r, "/#connections", 302)
}

// cloudRoutes registers the unified cloud-connector surface. Every route is
// behind the deployment capability switch; management actions (binding,
// disconnecting, starting OAuth) never register for agents.
func (s *Service) cloudRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/connections/gdrive", s.wrap("connections.gdrive", s.BeginGoogleDriveConnection))
	mux.HandleFunc("GET /auth/gdrive/callback", s.finishGoogleDrive)
	mux.HandleFunc("POST /api/v1/connections/{id}/roots", s.wrap("cloud.roots_bind", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			RootRef string `json:"root_ref"`
			Name    string `json:"name"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.BindCloudRoot(p, r.PathValue("id"), q.RootRef, q.Name)
		if e != nil {
			return e
		}
		markAuditTarget(r, view.RootID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/connections/{id}/roots", s.wrap("cloud.roots_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		views, e := s.ListCloudRoots(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, views)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/{id}/roots/{root}/disconnect", s.wrap("cloud.roots_disconnect", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.DisconnectCloudRoot(p, r.PathValue("id"), r.PathValue("root"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/connections/{id}/cloud-capabilities", s.wrap("cloud.capabilities", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.CloudCapabilities(r.Context(), p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/connections/{id}/entries", s.wrap("cloud.entries", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit, e := parseCloudLimit(r.URL.Query().Get("limit"))
		if e != nil {
			return e
		}
		view, e := s.ListCloudEntries(r.Context(), p, r.PathValue("id"), r.URL.Query().Get("root_id"), r.URL.Query().Get("parent_ref"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/{id}/entries/stat", s.wrap("cloud.entry_stat", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			EntryRef         string `json:"entry_ref"`
			ExpectedRevision string `json:"expected_revision"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.StatCloudEntry(r.Context(), p, r.PathValue("id"), q.EntryRef, q.ExpectedRevision)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/connections/{id}/changes", s.wrap("cloud.changes", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit, e := parseCloudLimit(r.URL.Query().Get("limit"))
		if e != nil {
			return e
		}
		view, e := s.ReadCloudChanges(r.Context(), p, r.PathValue("id"), r.URL.Query().Get("root_id"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/{id}/scan", s.wrap("cloud.scan", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CloudScanRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.ScanCloudConnection(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.RootID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/{id}/entries/import", s.wrap("cloud.import", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ImportCloudEntryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		op, e := s.ImportCloudEntry(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, op.ID)
		respond(w, op)
		return nil
	}))
}

// parseCloudLimit accepts only plain decimal digits so limits never smuggle
// other syntax into the query.
func parseCloudLimit(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	v := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, code("invalid_request")
		}
		v = v*10 + int(c-'0')
		if v > 1_000_000 {
			return 0, code("invalid_request")
		}
	}
	return v, nil
}
