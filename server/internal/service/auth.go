package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type loginProvider struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}
type flowSecret struct {
	Verifier string
	Nonce    string
}

func (s *Service) ConfigureLogin(ctx context.Context) error {
	u, e := url.Parse(s.Config.Origin)
	if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Scheme != "https" && !(s.Config.Development && u.Scheme == "http" && loopback(u.Hostname()))) {
		return code("invalid_public_origin")
	}
	issuer, e := url.Parse(s.Config.Issuer)
	if e != nil || issuer.Host == "" || (issuer.Scheme != "https" && !(s.Config.Development && issuer.Scheme == "http" && loopback(issuer.Hostname()))) || s.Config.ClientID == "" {
		return code("oidc_not_configured")
	}
	provider, e := oidc.NewProvider(ctx, s.Config.Issuer)
	if e != nil {
		return code("oidc_discovery_failed")
	}
	s.oidc = &loginProvider{oauth: oauth2.Config{ClientID: s.Config.ClientID, ClientSecret: s.Config.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: s.Config.Origin + "/auth/callback", Scopes: []string{oidc.ScopeOpenID, "profile"}}, verifier: provider.VerifierContext(context.Background(), &oidc.Config{ClientID: s.Config.ClientID})}
	return nil
}
func (s *Service) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(name), Value: value, Path: "/", HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (s *Service) beginLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled || s.oidc == nil {
		writeError(w, code("oidc_not_configured"))
		return
	}
	state, nonce, verifier := ID(), ID(), oauth2.GenerateVerifier()
	secret, _ := s.seal("login", flowSecret{verifier, nonce})
	f := Flow{Hash: Hash(state), Purpose: "login", Secret: secret, Expires: time.Now().Add(10 * time.Minute)}
	if s.DB.Create(&f).Error != nil {
		writeError(w, code("database_unavailable"))
		return
	}
	s.cookie(w, "db_flow", state, 600)
	http.Redirect(w, r, s.oidc.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "login"), oauth2.SetAuthURLParam("max_age", "0")), http.StatusFound)
}
func (s *Service) consumeFlow(r *http.Request, purpose string) (Flow, error) {
	state := r.URL.Query().Get("state")
	cookie, e := r.Cookie(s.cookieName("db_flow"))
	if e != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		return Flow{}, code("invalid_auth_state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var flow Flow
	if s.DB.First(&flow, "hash = ? AND purpose = ?", Hash(state), purpose).Error != nil || !time.Now().Before(flow.Expires) {
		return flow, code("invalid_auth_state")
	}
	if s.DB.Delete(&flow).Error != nil {
		return flow, code("database_unavailable")
	}
	return flow, nil
}
func (s *Service) finishLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled || s.oidc == nil {
		writeError(w, code("oidc_not_configured"))
		return
	}
	flow, e := s.consumeFlow(r, "login")
	s.cookie(w, "db_flow", "", -1)
	if e != nil {
		writeError(w, e)
		return
	}
	var f flowSecret
	if e = s.unseal("login", flow.Secret, &f); e != nil {
		writeError(w, e)
		return
	}
	token, e := s.oidc.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(f.Verifier))
	if e != nil {
		writeError(w, code("oidc_exchange_failed"))
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		writeError(w, code("oidc_token_invalid"))
		return
	}
	id, e := s.oidc.verifier.Verify(r.Context(), raw)
	if e != nil || id.Subject == "" || id.Nonce != f.Nonce {
		writeError(w, code("oidc_token_invalid"))
		return
	}
	var claims struct {
		Name     string `json:"name"`
		AuthTime int64  `json:"auth_time"`
	}
	_ = id.Claims(&claims)
	if len(claims.Name) > 120 {
		claims.Name = claims.Name[:120]
	}
	user, e := s.ensureUser(id.Issuer, id.Subject, claims.Name)
	if e != nil {
		writeError(w, e)
		return
	}
	session := ID()
	authenticatedAt := time.Time{}
	if claims.AuthTime > 0 && claims.AuthTime <= time.Now().Unix() {
		authenticatedAt = time.Unix(claims.AuthTime, 0)
	}
	v := Session{AuthenticatedAt: authenticatedAt, Hash: Hash(session), Owner: user.ID, CSRF: ID(), Expires: time.Now().Add(12 * time.Hour)}
	if s.DB.Create(&v).Error != nil {
		writeError(w, code("database_unavailable"))
		return
	}
	s.cookie(w, "db_session", session, 12*3600)
	s.CaptureUsage(user.ID)
	http.Redirect(w, r, "/", http.StatusFound)
}
func (s *Service) ensureUser(issuer, sub, name string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var u User
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if tx.First(&u, "issuer = ? AND subject = ?", issuer, sub).Error == nil {
			return nil
		}
		u = User{ID: ID(), Issuer: issuer, Subject: sub, Name: name}
		if e := tx.Create(&u).Error; e != nil {
			return e
		}
		if s.governanceActive() {
			return nil
		}
		connection := ""
		var platform Connection
		if tx.First(&platform, "id = ? AND disabled = ?", "platform", false).Error == nil {
			connection = platform.ID
		}
		return tx.Create(&Space{ID: ID(), Owner: u.ID, Name: "个人空间", ConnectionID: connection, MaxBytes: s.Config.MaxBytes}).Error
	})
	if e != nil {
		return u, code("database_unavailable")
	}
	return u, nil
}
func (s *Service) authenticate(r *http.Request) (Principal, error) {
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		// Hash-first routing: user keys may carry any prefix since external
		// pre-generated keys can be imported; agent tokens never collide
		// because import rejects cross-table hash duplicates.
		var k UserKey
		if s.DB.First(&k, "token_hash = ?", Hash(bearer)).Error == nil {
			if !activeUserKey(k) {
				return Principal{}, code("unauthorized")
			}
			return Principal{UserID: k.Owner, Key: &k, Admin: k.Admin}, nil
		}
		var a Agent
		if s.Config.ServiceIdentityIssuer != "" && strings.Count(bearer, ".") == 2 {
			return s.authenticateServiceIdentity(r.Context(), bearer)
		}
		if s.DB.First(&a, "token_hash = ?", Hash(bearer)).Error != nil {
			return Principal{}, code("unauthorized")
		}
		if a.RestrictionProfile != "" {
			return s.authenticateWorkspaceAgent(r, a)
		}
		if a.Revoked || !time.Now().Before(a.Expires) {
			return Principal{}, code("unauthorized")
		}
		if err := rejectWorkspaceLegacyAccess(s.DB, a); err != nil {
			return Principal{}, code("unauthorized")
		}
		return Principal{UserID: a.Owner, Agent: &a}, nil
	}
	c, e := r.Cookie(s.cookieName("db_session"))
	if e != nil {
		return Principal{}, code("unauthorized")
	}
	var v Session
	if s.DB.First(&v, "hash = ?", Hash(c.Value)).Error != nil || !time.Now().Before(v.Expires) {
		return Principal{}, code("unauthorized")
	}
	if v.OriginKey != "" {
		var origin UserKey
		if s.DB.First(&origin, "id = ?", v.OriginKey).Error != nil || !activeUserKey(origin) {
			return Principal{}, code("unauthorized")
		}
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if r.Header.Get("Origin") != s.Config.Origin || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(v.CSRF)) != 1 {
			return Principal{}, code("csrf_rejected")
		}
	}
	return Principal{UserID: v.Owner, Admin: v.Admin, CSRF: v.CSRF, SessionHash: v.Hash}, nil
}
func (s *Service) createAgent(p Principal, name, space, prefix, scopes, connection string, days int) (Agent, string, error) {
	if p.Agent != nil {
		return Agent{}, "", code("forbidden")
	}
	if sp, e := s.space(p, space); e != nil {
		return Agent{}, "", e
	} else if s.governanceActive() && s.scopeRole(p, sp) != "owner" {
		return Agent{}, "", code("forbidden")
	}
	if prefix != "" {
		if _, e := cleanPath(prefix); e != nil {
			return Agent{}, "", e
		}
	}
	if scopes == "" {
		scopes = "read"
	}
	for _, v := range strings.Split(scopes, ",") {
		switch v {
		case "read", "write", "delete", "import", "export":
		default:
			return Agent{}, "", code("invalid_scope")
		}
	}
	if days == 0 {
		days = 7
	}
	if days < 1 || days > 30 || name == "" || len(name) > 120 {
		return Agent{}, "", code("invalid_agent")
	}
	if connection != "" {
		var c Connection
		if s.DB.First(&c, "id = ? AND owner = ? AND disabled = ?", connection, p.dataOwner(), false).Error != nil {
			return Agent{}, "", code("not_found")
		}
	}
	secret := "db_" + ID() + ID()
	a := Agent{ID: ID(), Owner: p.UserID, Name: s.cookieName(name), SpaceID: space, Prefix: prefix, Scopes: scopes, ConnectionID: connection, Expires: time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour), TokenHash: Hash(secret)}
	if s.DB.Create(&a).Error != nil {
		return a, "", code("database_unavailable")
	}
	return a, secret, nil
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return code("invalid_request")
	}
	return nil
}

func (s *Service) cookieName(name string) string {
	if s.Config.Development {
		return name
	}
	return "__Host-" + name
}

// authMode reports which login paths this deployment honestly supports; it
// carries no tenant-sensitive detail and needs no authentication.
func (s *Service) authMode(w http.ResponseWriter, r *http.Request) {
	respond(w, map[string]any{"oidc": s.Config.Issuer != "", "key": true, "governance": s.governanceActive()})
}

// keyLogin exchanges a user key for the same cookie session OIDC login
// mints. The key itself never persists anywhere client-side; logout or
// expiry drops the session. A cross-origin POST is rejected so an attacker
// cannot silently log a victim into an attacker-controlled account.
func (s *Service) keyLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled {
		writeError(w, code("disabled"))
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != s.Config.Origin {
		writeError(w, code("csrf_rejected"))
		return
	}
	var q struct {
		Key string `json:"key"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&q) != nil || q.Key == "" {
		writeError(w, code("unauthorized"))
		return
	}
	var k UserKey
	if s.DB.First(&k, "token_hash = ?", Hash(q.Key)).Error != nil || !activeUserKey(k) {
		writeError(w, code("unauthorized"))
		return
	}
	var user User
	if s.DB.First(&user, "id = ? AND disabled = ?", k.Owner, false).Error != nil {
		writeError(w, code("unauthorized"))
		return
	}
	if len(k.SealedKey) == 0 {
		if err := s.sealUserKey(&k, q.Key); err != nil {
			writeError(w, err)
			return
		}
		if s.DB.Model(&UserKey{}).Where("id = ? AND revoked = ? AND archived = ? AND disabled = ?", k.ID, false, false, false).Update("sealed_key", k.SealedKey).Error != nil {
			writeError(w, code("database_unavailable"))
			return
		}
	}
	session := ID()
	v := Session{AuthenticatedAt: time.Now().UTC(), Hash: Hash(session), Owner: k.Owner, CSRF: ID(), Admin: k.Admin, OriginKey: k.ID, Expires: time.Now().Add(12 * time.Hour)}
	if s.DB.Create(&v).Error != nil {
		writeError(w, code("database_unavailable"))
		return
	}
	s.cookie(w, "db_session", session, 12*3600)
	s.CaptureUsage(k.Owner)
	respond(w, map[string]any{"admin": k.Admin})
}
