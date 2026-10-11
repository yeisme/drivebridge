package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceIdentitySignedMappingScopeAndOutage(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	c := serviceIdentity{Subject: "central-user", TenantID: "central-org", ActorType: "human", MembershipVersion: 1, IssuedAt: now, ExpiresAt: now + 300, Credential: &serviceCredential{KeyID: "key", KeyFamilyID: "family", TenantID: "central-org", PrincipalType: "user", AuthorityVersion: "v1", Audiences: []string{"drivebridge"}, Scopes: []string{"drivebridge:read"}}}
	var status atomic.Int64
	status.Store(200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/jwks.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "kid": "fixture", "use": "sig", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(pub)}}})
			return
		}
		if status.Load() != 200 {
			w.WriteHeader(int(status.Load()))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": c})
	}))
	defer server.Close()
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(Config{State: state, Enabled: true, Development: true, ServiceIdentityIssuer: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.DB.Create(&User{ID: "local-user", Issuer: "old", Subject: "old-user"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&ServiceIdentityBinding{ID: "binding", Issuer: server.URL, Subject: c.Subject, IdentityTenant: c.TenantID, UserID: "local-user", TenantID: "local-org", Active: true, Cutover: true}).Error; err != nil {
		t.Fatal(err)
	}
	sign := func(audience string) string {
		encoded, _ := json.Marshal(c)
		var claims map[string]any
		_ = json.Unmarshal(encoded, &claims)
		claims["iss"] = server.URL
		claims["aud"] = audience
		payload, _ := json.Marshal(claims)
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","kid":"fixture"}`))
		input := header + "." + base64.RawURLEncoding.EncodeToString(payload)
		return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(input)))
	}
	token := sign("drivebridge")
	p, err := s.authenticateServiceIdentity(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != "local-user" || p.TenantID != "local-org" || p.Admin {
		t.Fatal("mapping or role escalation")
	}
	if err := s.serviceIdentityGuard(p, "files.list"); err != nil {
		t.Fatal(err)
	}
	if err := s.serviceIdentityGuard(p, "uploads.create"); err == nil {
		t.Fatal("read-only key wrote data")
	}
	if _, err := s.authenticateServiceIdentity(context.Background(), sign("eikona")); err == nil {
		t.Fatal("wrong audience accepted")
	}
	status.Store(503)
	p, err = s.authenticateServiceIdentity(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if p.ServiceIdentity.online {
		t.Fatal("outage marked online")
	}
	if err := s.serviceIdentityGuard(p, "files.list"); err != nil {
		t.Fatal(err)
	}
	if err := s.serviceIdentityGuard(p, "uploads.create"); err == nil {
		t.Fatal("outage allowed write")
	}
	status.Store(403)
	if _, err := s.authenticateServiceIdentity(context.Background(), token); err == nil {
		t.Fatal("explicit deny treated as outage")
	}
	if err := s.serviceIdentityGuard(Principal{UserID: "local-user", TenantID: "local-org"}, "files.list"); err == nil {
		t.Fatal("old credential bypassed cutover")
	}
}
