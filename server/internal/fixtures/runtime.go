// Package fixtures provides disposable local OIDC and S3 services. It is used
// only by tests and the explicitly named fixture command, never by serve.
package fixtures

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/yeisme/drivebridge-server/internal/service"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Runtime struct {
	listCalls atomic.Int64
	App       *httptest.Server
	OIDC      *httptest.Server
	S3        *httptest.Server
	Service   *service.Service
	mu        sync.Mutex
	objects   map[string][]byte
	claims    map[string]url.Values
	badClaim  string
	Failure   bool
	// diskDir enables the disk-backed streaming object store for scale
	// fixtures. Objects stream to files instead of resident memory.
	diskDir    string
	diskSizes  map[string]int64
	diskCounts map[string]int
}

type Options struct {
	// DiskObjects switches the fixture object store to disk-backed
	// streaming in this directory (scale fixtures only; no size cap).
	DiskObjects string
	// FailArchivePublishOnce enables one real transaction failure for local recovery tests only.
	FailArchivePublishOnce   bool
	Ready                    *atomic.Bool
	RequireExplicitAdminRead bool
	MaxBytes                 int64
	// KeepStorage retains the already running fixture store during metadata handoff.
	KeepStorage bool
	// ResumeDemo reuses an explicitly checkpointed fixture without file objects.
	ResumeDemo   bool
	TeamDemo     bool
	Demo         bool
	PublicOrigin string
	// CloudConnectors enables the unified cloud directory surface on the
	// fixture service so connector tests run against the real routes.
	CloudConnectors bool
	// TransferPlans enables the cross-storage transfer surface likewise.
	TransferPlans bool
	// PropertyEditing opts into sidecar writes only in disposable fixtures.
	PropertyEditing      bool
	AutomationManagement bool
	ShareReceivers       bool
	HoldManagement       bool
	LabelManagement      bool
}

func existingDemoUsers(s *service.Service, names []string) ([]service.UserKeyIssued, error) {
	out := []service.UserKeyIssued{}
	for _, name := range names {
		var users []service.User
		if err := s.DB.Where("name = ?", name).Find(&users).Error; err != nil || len(users) != 1 {
			return nil, fmt.Errorf("fixture persona identity unavailable")
		}
		out = append(out, service.UserKeyIssued{UserID: users[0].ID, Name: name})
	}
	return out, nil
}

func Start(state, web, listen string) (*Runtime, error) {
	return StartWithOptions(state, web, listen, Options{})
}

func StartWithOptions(state, web, listen string, options Options) (*Runtime, error) {
	if options.KeepStorage && !options.ResumeDemo {
		return nil, fmt.Errorf("keep storage requires resume mode")
	}
	if options.ResumeDemo && !options.Demo {
		return nil, fmt.Errorf("resume requires explicit demo mode")
	}
	if options.TeamDemo && !options.Demo {
		return nil, fmt.Errorf("team demo requires demo mode")
	}
	if options.Demo || options.PublicOrigin != "" || options.FailArchivePublishOnce {
		host, _, err := net.SplitHostPort(listen)
		if ip := net.ParseIP(host); err != nil || ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) {
			return nil, fmt.Errorf("demo requires an explicit numeric local listener")
		}
	}
	if options.PublicOrigin != "" {
		u, err := url.Parse(options.PublicOrigin)
		if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Hostname() != "localhost" && !func() bool {
			ip := net.ParseIP(u.Hostname())
			return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
		}()) {
			return nil, fmt.Errorf("fixture public origin must be an exact HTTP local origin")
		}
	}
	os.MkdirAll(state, 0700)
	os.Chmod(state, 0700)
	f := &Runtime{objects: map[string][]byte{}, claims: map[string]url.Values{}, diskSizes: map[string]int64{}, diskCounts: map[string]int{}}
	if options.DiskObjects != "" {
		if e := os.MkdirAll(options.DiskObjects, 0700); e != nil {
			return nil, e
		}
		f.diskDir = options.DiskObjects
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return nil, e
	}
	signer, e := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "fixture-key"))
	if e != nil {
		return nil, e
	}
	f.OIDC = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": f.OIDC.URL, "authorization_endpoint": f.OIDC.URL + "/authorize", "token_endpoint": f.OIDC.URL + "/token", "jwks_uri": f.OIDC.URL + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "fixture-key", Algorithm: "RS256", Use: "sig"}}})
		case "/authorize":
			id := service.ID()
			f.mu.Lock()
			f.claims[id] = r.URL.Query()
			f.mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html lang="zh-CN"><title>Fixture login</title><body><h1>本地测试身份</h1><a href="/approve?id=%s&amp;user=alice">以 Alice 登录</a><br><a href="/approve?id=%s&amp;user=bob">以 Bob 登录</a></body></html>`, html.EscapeString(id), html.EscapeString(id))
		case "/approve":
			f.mu.Lock()
			q, ok := f.claims[r.URL.Query().Get("id")]
			if ok {
				delete(f.claims, r.URL.Query().Get("id"))
				q.Set("user", r.URL.Query().Get("user"))
				code := service.ID()
				f.claims[code] = q
				f.mu.Unlock()
				redirect := q.Get("redirect_uri") + "?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(q.Get("state"))
				http.Redirect(w, r, redirect, 302)
			} else {
				f.mu.Unlock()
				http.Error(w, "unknown fixture transaction", 400)
			}
		case "/token":
			r.ParseForm()
			f.mu.Lock()
			q, ok := f.claims[r.Form.Get("code")]
			delete(f.claims, r.Form.Get("code"))
			bad := f.badClaim
			f.mu.Unlock()
			if !ok {
				http.Error(w, "invalid fixture code", 400)
				return
			}
			challenge := r.Form.Get("code_verifier")
			sum := sha256.Sum256([]byte(challenge))
			if challenge == "" || q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) {
				http.Error(w, "PKCE required", 400)
				return
			}
			now := time.Now()
			claims := map[string]any{"iss": f.OIDC.URL, "sub": q.Get("user"), "aud": "fixture-client", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": q.Get("nonce"), "name": strings.ToUpper(q.Get("user")[:1]) + q.Get("user")[1:], "email": "same-address@example.invalid"}
			switch bad {
			case "issuer":
				claims["iss"] = "https://wrong.invalid"
			case "audience":
				claims["aud"] = "other-client"
			case "nonce":
				claims["nonce"] = "wrong"
			case "expired":
				claims["exp"] = now.Add(-time.Hour).Unix()
			}
			b, _ := json.Marshal(claims)
			signed, _ := signer.Sign(b)
			jwt, _ := signed.CompactSerialize()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture-only-access", "token_type": "Bearer", "expires_in": 3600, "id_token": jwt})
		default:
			http.NotFound(w, r)
		}
	}))
	f.S3 = httptest.NewServer(f.S3Handler())
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 32 << 20
	}
	s, e := service.Open(service.Config{RequireExplicitAdminRead: options.RequireExplicitAdminRead, State: state, Development: true, Enabled: true, DocumentEditing: true, PropertyEditing: options.PropertyEditing, AutomationManagement: options.AutomationManagement, Issuer: f.OIDC.URL, ClientID: "fixture-client", MaxBytes: maxBytes, WebDir: web, CloudConnectors: options.CloudConnectors, TransferPlans: options.TransferPlans})
	if e != nil {
		f.OIDC.Close()
		f.S3.Close()
		return nil, e
	}
	s.Config.ShareReceivers = options.ShareReceivers
	s.Config.HoldManagement = options.HoldManagement
	s.Config.LabelManagement = options.LabelManagement
	f.Service = s
	if options.FailArchivePublishOnce {
		if e = installArchivePublishFailure(s.DB); e != nil {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, e
		}
	}
	var oldPlatform service.Connection
	if options.ResumeDemo {
		var files, pending int64
		if s.DB.Model(&service.File{}).Count(&files).Error != nil || s.DB.Model(&service.Operation{}).Where("status NOT IN ?", []string{"completed", "cancelled", "failed"}).Count(&pending).Error != nil || (files != 0 && !options.KeepStorage) || pending != 0 {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, fmt.Errorf("fixture resume requires no file objects or pending operations")
		}
		// No objects exist, so only the fixture's loopback storage endpoint
		// changes. Accounts, sessions, directory IDs and bindings stay intact.
		if e = s.DB.First(&oldPlatform, "id = ?", "platform").Error; e != nil {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, e
		}
		if !options.KeepStorage {
			if e = s.DB.Delete(&oldPlatform).Error; e != nil {
				s.Close()
				f.OIDC.Close()
				f.S3.Close()
				return nil, e
			}
		}
	}
	if !options.KeepStorage {
		if e = s.SetPlatform(context.Background(), service.S3Config{Endpoint: f.S3.URL, Region: "us-east-1", Bucket: "fixture", Prefix: "managed", AccessKey: "fixture-key", SecretKey: "fixture-secret"}); e != nil {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, e
		}
	}
	if options.ResumeDemo {
		if e = s.DB.Model(&service.Connection{}).Where("id = ?", "platform").Updates(map[string]any{"name": oldPlatform.Name, "disabled": oldPlatform.Disabled}).Error; e != nil {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, e
		}
	}
	handler := s.Handler()
	if options.Demo {
		var users []service.UserKeyIssued
		var err error
		if options.ResumeDemo {
			users, err = existingDemoUsers(s, []string{"本地体验"})
		} else {
			users, err = s.ImportUsers([]service.UserImport{{Name: "本地体验", Admin: true}})
		}
		if err != nil {
			s.Close()
			f.OIDC.Close()
			f.S3.Close()
			return nil, err
		}
		if !options.ResumeDemo {
			if err = s.DB.Create(&service.Space{ID: service.ID(), Owner: users[0].UserID, Name: "体验空间", ConnectionID: "platform", MaxBytes: 32 << 20}).Error; err != nil {
				s.Close()
				f.OIDC.Close()
				f.S3.Close()
				return nil, err
			}
		}
		demoUsers := map[string]string{"admin": users[0].UserID}
		personas := []map[string]string{}
		if options.TeamDemo {
			var more []service.UserKeyIssued
			var seedErr error
			if options.ResumeDemo {
				more, seedErr = existingDemoUsers(s, []string{"素材负责人", "团队成员"})
			} else {
				more, seedErr = s.ImportUsers([]service.UserImport{{Name: "素材负责人"}, {Name: "团队成员"}})
			}
			if seedErr != nil {
				f.Service.Close()
				f.OIDC.Close()
				f.S3.Close()
				return nil, seedErr
			}
			demoUsers["custodian"], demoUsers["member"] = more[0].UserID, more[1].UserID
			if !options.ResumeDemo {
				if seedErr = s.BootstrapGovernance(users[0].UserID); seedErr != nil {
					return nil, seedErr
				}
				team := service.Team{ID: service.ID(), TenantID: s.Instance, Name: "设计协作团队", Description: "隔离的本地团队体验，使用真实接口和临时数据。", Revision: 1, UpdatedAt: time.Now().UTC()}
				if seedErr = s.DB.Create(&team).Error; seedErr != nil {
					return nil, seedErr
				}
				for i, u := range []string{users[0].UserID, more[0].UserID, more[1].UserID} {
					role := "member"
					if i == 0 {
						role = "admin"
					}
					if seedErr = s.DB.Create(&service.TeamMember{TeamID: team.ID, UserID: u, Role: role}).Error; seedErr != nil {
						return nil, seedErr
					}
				}
				if seedErr = s.DB.Create(&service.TeamStorage{TeamID: team.ID, TenantID: s.Instance, ConnectionID: "platform", AssignedBy: users[0].UserID, Created: time.Now().UTC()}).Error; seedErr != nil {
					return nil, seedErr
				}
				sp := service.Space{ID: service.ID(), TeamID: team.ID, TenantID: s.Instance, Owner: service.ID(), Name: "团队素材", ConnectionID: "platform", MaxBytes: 32 << 20}
				if seedErr = s.DB.Create(&sp).Error; seedErr != nil {
					return nil, seedErr
				}
				for i, u := range more {
					role := "viewer"
					if i == 0 {
						role = "custodian"
					}
					if seedErr = s.DB.Create(&service.SpacePermission{SpaceID: sp.ID, UserID: u.UserID, Role: role}).Error; seedErr != nil {
						return nil, seedErr
					}
				}
			}
			personas = []map[string]string{{"id": "admin", "name": "团队管理员"}, {"id": "custodian", "name": "数据负责人"}, {"id": "member", "name": "普通成员"}}
		}
		owner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			// Explicit disposable demo: browser/proxy origins do not gate development.
			// The owner still requires the session and its CSRF token for mutations.
			r = r.Clone(r.Context())
			r.Header.Set("Origin", s.Config.Origin)
			if r.URL.Path == "/auth/mode" && r.Method == http.MethodGet {
				currentPersona := ""
				if cookie, e := r.Cookie("db_session"); e == nil {
					var session service.Session
					if s.DB.First(&session, "hash = ?", service.Hash(cookie.Value)).Error == nil {
						for id, uid := range demoUsers {
							if session.Owner == uid {
								currentPersona = id
							}
						}
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"oidc": false, "key": true, "demo": true, "team_demo": options.TeamDemo, "governance": func() bool {
					var state service.GovernanceState
					return s.DB.First(&state, 1).Error == nil && state.Active
				}(), "current_persona": currentPersona, "personas": personas}})
				return
			}
			if r.URL.Path == "/auth/demo" {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				var choice struct {
					Persona string `json:"persona"`
				}
				if r.Body != nil {
					_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&choice)
				}
				if choice.Persona == "" {
					choice.Persona = "admin"
				}
				uid, ok := demoUsers[choice.Persona]
				if !ok {
					http.Error(w, "Unknown demo persona", http.StatusBadRequest)
					return
				}
				// Resolve the active successor after self-service key rotation.
				var active service.UserKey
				if s.DB.Where("owner = ? AND revoked = ? AND archived = ?", uid, false, false).Order("created_at DESC").First(&active).Error != nil {
					http.Error(w, "Demo credential unavailable", http.StatusUnauthorized)
					return
				}
				key, err := s.RevealUserKey(service.Principal{UserID: uid}, active.ID)
				if err != nil {
					http.Error(w, "Demo credential unavailable", http.StatusUnauthorized)
					return
				}
				// Retire the previous browser session before a persona switch.
				if c, err := r.Cookie("db_session"); err == nil {
					s.DB.Where("hash = ?", service.Hash(c.Value)).Delete(&service.Session{})
				}
				body, _ := json.Marshal(map[string]string{"key": key})
				request := r.Clone(r.Context())
				request.URL.Path = "/auth/key"
				request.Body = io.NopCloser(bytes.NewReader(body))
				request.ContentLength = int64(len(body))
				owner.ServeHTTP(w, request)
				return
			}
			owner.ServeHTTP(w, r)
		})
	}
	if options.Ready != nil {
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !options.Ready.Load() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Development checkpoint recovery in progress", http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	f.App = httptest.NewUnstartedServer(handler)
	if listen != "" {
		f.App.Listener.Close()
		listener, e := net.Listen("tcp", listen)
		if e != nil {
			return nil, e
		}
		f.App.Listener = listener
	}
	s.Config.Origin = "http://" + f.App.Listener.Addr().String()
	if options.PublicOrigin != "" {
		s.Config.Origin = options.PublicOrigin
	}
	if e = s.ConfigureLogin(context.Background()); e != nil {
		return nil, e
	}
	f.App.Start()
	return f, nil
}

// S3Handler exposes the in-memory fixture object store so the fixture command
// can publish it on an explicit loopback listener for offline rehearsals.
func (f *Runtime) S3Handler() http.Handler {
	if f.diskDir != "" {
		return f.diskS3()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.Failure {
			http.Error(w, "fixture storage failure", 503)
			return
		}
		switch r.Method {
		case "HEAD":
			// Bucket probes stay reachable; deeper paths report object
			// existence so verification can observe missing objects. The
			// ETag mirrors the listing value so HEAD and list agree.
			w.Header().Set("ETag", `"fixture-etag"`)
			if strings.Count(r.URL.Path, "/") > 1 {
				if object, ok := f.objects[r.URL.Path]; ok {
					w.Header().Set("Content-Length", strconv.Itoa(len(object)))
				} else {
					w.WriteHeader(404)
					return
				}
			}
			w.WriteHeader(200)
		case "PUT":
			b, e := io.ReadAll(io.LimitReader(r.Body, 64<<20))
			if e != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			f.objects[r.URL.Path] = b
			w.Header().Set("ETag", `"fixture-etag"`)
			w.WriteHeader(200)
		case "GET":
			if r.URL.Query().Get("list-type") == "2" {
				f.listCalls.Add(1)
				type item struct {
					Key          string    `xml:"Key"`
					Size         int       `xml:"Size"`
					ETag         string    `xml:"ETag"`
					LastModified time.Time `xml:"LastModified"`
				}
				type commonPrefix struct {
					Prefix string `xml:"Prefix"`
				}
				out := struct {
					XMLName        xml.Name       `xml:"ListBucketResult"`
					Contents       []item         `xml:"Contents"`
					CommonPrefixes []commonPrefix `xml:"CommonPrefixes"`
					Truncated      bool           `xml:"IsTruncated"`
					NextToken      string         `xml:"NextContinuationToken"`
					KeyCount       int            `xml:"KeyCount"`
				}{Contents: []item{}, CommonPrefixes: []commonPrefix{}}
				prefix := r.URL.Query().Get("prefix")
				delimiter := r.URL.Query().Get("delimiter")
				startAfter := r.URL.Query().Get("start-after")
				if token := r.URL.Query().Get("continuation-token"); token != "" && startAfter == "" {
					startAfter = token
				}
				maxKeys := 1000
				if raw := r.URL.Query().Get("max-keys"); raw != "" {
					if v, e := strconv.Atoi(raw); e == nil && v > 0 {
						maxKeys = v
					}
				}
				base := "/" + strings.Trim(r.URL.Path, "/") + "/"
				seenPrefix := map[string]bool{}
				for key, value := range f.objects {
					relative := strings.TrimPrefix(key, base)
					if !strings.HasPrefix(relative, prefix) || relative == prefix {
						continue
					}
					if startAfter != "" && relative <= startAfter {
						continue
					}
					rest := strings.TrimPrefix(relative, prefix)
					if delimiter != "" {
						if idx := strings.Index(rest, delimiter); idx >= 0 {
							rolled := prefix + rest[:idx+len(delimiter)]
							if !seenPrefix[rolled] {
								seenPrefix[rolled] = true
								out.CommonPrefixes = append(out.CommonPrefixes, commonPrefix{rolled})
							}
							continue
						}
					}
					out.Contents = append(out.Contents, item{Key: relative, Size: len(value), ETag: "fixture-etag", LastModified: time.Now().UTC().Round(time.Second)})
				}
				sort.Slice(out.CommonPrefixes, func(i, j int) bool { return out.CommonPrefixes[i].Prefix < out.CommonPrefixes[j].Prefix })
				// Interleave directories and files in key order like real S3:
				// merge both sorted lists respecting the shared key space.
				type entry struct {
					key       string
					directory bool
				}
				merged := make([]entry, 0, len(out.Contents)+len(out.CommonPrefixes))
				for _, p := range out.CommonPrefixes {
					merged = append(merged, entry{p.Prefix, true})
				}
				for _, c := range out.Contents {
					merged = append(merged, entry{c.Key, false})
				}
				sort.Slice(merged, func(i, j int) bool { return merged[i].key < merged[j].key })
				var keptContents []item
				var keptPrefixes []commonPrefix
				var lastKey string
				for _, m := range merged {
					if len(keptContents)+len(keptPrefixes) >= maxKeys {
						out.Truncated = true
						break
					}
					lastKey = m.key
					if m.directory {
						keptPrefixes = append(keptPrefixes, commonPrefix{m.key})
					} else {
						for _, c := range out.Contents {
							if c.Key == m.key {
								keptContents = append(keptContents, c)
								break
							}
						}
					}
				}
				out.Contents = keptContents
				out.CommonPrefixes = keptPrefixes
				out.KeyCount = len(out.Contents) + len(out.CommonPrefixes)
				if out.Truncated {
					out.NextToken = lastKey
				}
				w.Header().Set("Content-Type", "application/xml")
				xml.NewEncoder(w).Encode(out)
				return
			}
			b, ok := f.objects[r.URL.Path]
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			http.ServeContent(w, r, "object", time.Time{}, bytes.NewReader(b))
		case "DELETE":
			delete(f.objects, r.URL.Path)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	})
}

// SeedObjects bulk-fills the fixture store for scale fixtures. It exists
// only for tests: one HTTP PUT per object would dominate the run.
func (f *Runtime) SeedObjects(key string, size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = bytes.Repeat([]byte("s"), size)
}

func (f *Runtime) Close()           { f.App.Close(); f.Service.Close(); f.OIDC.Close(); f.S3.Close() }
func (f *Runtime) ObjectCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.objects) }

// ListCalls reports how many ListObjectsV2 requests the fixture store has
// served; scans assert bounded provider paging against it.
func (f *Runtime) ListCalls() int64     { return f.listCalls.Load() }
func (f *Runtime) SetBadClaim(v string) { f.mu.Lock(); defer f.mu.Unlock(); f.badClaim = v }

// Objects returns a copy of the raw stored bytes so tests can assert
// ciphertext at rest without touching live credential material.
func (f *Runtime) Objects() map[string][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string][]byte, len(f.objects))
	for k, v := range f.objects {
		out[k] = v
	}
	return out
}

// DropObject removes one stored object so tests can produce object-missing
// verification failures.
func (f *Runtime) DropObject(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	delete(f.objects, key)
	return ok
}
