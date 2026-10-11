package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"gorm.io/gorm"
)

// ServiceIdentityBinding is an exact migration mapping, never an email match.
type ServiceIdentityBinding struct {
	ID             string `gorm:"primaryKey"`
	Issuer         string `gorm:"uniqueIndex:service_identity_source"`
	Subject        string `gorm:"uniqueIndex:service_identity_source"`
	IdentityTenant string `gorm:"uniqueIndex:service_identity_source"`
	UserID         string `gorm:"index"`
	TenantID       string `gorm:"index"`
	AgentID        string
	Cutover        bool
	Active         bool
	CreatedAt      time.Time
}

// BindServiceIdentity is deployment-side only. An existing mapping cannot be
// reassigned and a cutover cannot silently revive legacy credentials.
func (s *Service) BindServiceIdentity(b ServiceIdentityBinding) error {
	u, err := url.Parse(b.Issuer)
	if !s.governanceActive() || err != nil || u.Host == "" || u.Scheme != "https" || b.Subject == "" || b.IdentityTenant == "" || b.UserID == "" || b.TenantID == "" {
		return code("invalid_identity_mapping")
	}
	s.governanceMu.Lock()
	defer s.governanceMu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var user User
		var member Membership
		if tx.First(&user, "id = ? AND disabled = ?", b.UserID, false).Error != nil || tx.First(&member, "user_id = ? AND tenant_id = ? AND status = ?", b.UserID, b.TenantID, "active").Error != nil {
			return code("invalid_identity_mapping")
		}
		if b.AgentID != "" {
			var agent Agent
			if tx.First(&agent, "id = ? AND owner = ? AND revoked = ?", b.AgentID, b.UserID, false).Error != nil {
				return code("invalid_identity_mapping")
			}
		}
		b.ID = Hash(b.Issuer + "\x00" + b.Subject + "\x00" + b.IdentityTenant)
		b.Active = true
		var old ServiceIdentityBinding
		if err := tx.First(&old, "id = ?", b.ID).Error; err == nil {
			if old.UserID != b.UserID || old.TenantID != b.TenantID || old.AgentID != b.AgentID {
				return code("identity_mapping_conflict")
			}
			b.Cutover = b.Cutover || old.Cutover
			b.CreatedAt = old.CreatedAt
		} else if err != gorm.ErrRecordNotFound {
			return code("database_unavailable")
		}
		if err := tx.Save(&b).Error; err != nil {
			return code("database_unavailable")
		}
		if b.Cutover {
			var members []Membership
			if tx.Where("tenant_id = ? AND status = ?", b.TenantID, "active").Find(&members).Error != nil {
				return code("database_unavailable")
			}
			for _, m := range members {
				var count int64
				if tx.Model(&ServiceIdentityBinding{}).Where("tenant_id = ? AND user_id = ? AND active = ?", b.TenantID, m.UserID, true).Count(&count).Error != nil || count == 0 {
					return code("identity_mapping_incomplete")
				}
			}
		}
		return s.event(tx, Principal{UserID: "deployment-operator", TenantID: b.TenantID}, Space{}, "identity.link", b.ID, "", "success", b.UserID)
	})
}

type serviceCredential struct {
	KeyID            string   `json:"key_id"`
	KeyFamilyID      string   `json:"key_family_id"`
	TenantID         string   `json:"tenant_id"`
	PrincipalType    string   `json:"principal_type"`
	AuthorityVersion string   `json:"authority_version"`
	Audiences        []string `json:"audiences"`
	Scopes           []string `json:"scopes"`
}
type serviceIdentity struct {
	Subject           string             `json:"sub"`
	TenantID          string             `json:"tenant_id"`
	ActorType         string             `json:"actor_type"`
	MembershipVersion int64              `json:"membership_version"`
	IssuedAt          int64              `json:"iat"`
	ExpiresAt         int64              `json:"exp"`
	Credential        *serviceCredential `json:"credential_context"`
	token             string
	online            bool
	localTenant       string
}

func (s *Service) ConfigureServiceIdentity() error {
	if s.Config.ServiceIdentityIssuer == "" {
		return nil
	}
	for _, value := range []string{s.Config.ServiceIdentityIssuer, s.serviceIdentityEndpoint()} {
		u, err := url.Parse(value)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(s.Config.Development && u.Scheme == "http" && loopback(u.Hostname()))) {
			return code("invalid_service_identity_config")
		}
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx := oidc.ClientContext(context.Background(), client)
	s.serviceVerifier = oidc.NewVerifier(s.Config.ServiceIdentityIssuer, oidc.NewRemoteKeySet(ctx, strings.TrimRight(s.serviceIdentityEndpoint(), "/")+"/.well-known/jwks.json"), &oidc.Config{ClientID: "drivebridge", SupportedSigningAlgs: []string{"EdDSA"}})
	return nil
}
func (s *Service) serviceIdentityEndpoint() string {
	if s.Config.ServiceIdentityEndpoint != "" {
		return s.Config.ServiceIdentityEndpoint
	}
	return s.Config.ServiceIdentityIssuer
}

func (s *Service) authenticateServiceIdentity(ctx context.Context, token string) (Principal, error) {
	if s.serviceVerifier == nil {
		return Principal{}, code("unauthorized")
	}
	id, err := s.serviceVerifier.Verify(ctx, token)
	if err != nil {
		return Principal{}, code("unauthorized")
	}
	var c serviceIdentity
	if id.Claims(&c) != nil || c.Subject == "" || c.TenantID == "" || c.MembershipVersion < 1 || c.Credential == nil || c.IssuedAt > time.Now().Unix() || c.ExpiresAt <= c.IssuedAt || c.ExpiresAt-c.IssuedAt > 300 {
		return Principal{}, code("unauthorized")
	}
	origin := c.Credential
	if origin.KeyID == "" || origin.KeyFamilyID == "" || origin.AuthorityVersion == "" || origin.TenantID != c.TenantID || !slices.Contains(origin.Audiences, "drivebridge") || (origin.PrincipalType != "user" && origin.PrincipalType != "service_account") {
		return Principal{}, code("unauthorized")
	}
	if origin.PrincipalType == "service_account" && c.ActorType == "human" {
		return Principal{}, code("unauthorized")
	}
	var binding ServiceIdentityBinding
	if s.DB.Where(&ServiceIdentityBinding{Issuer: id.Issuer, Subject: c.Subject, IdentityTenant: c.TenantID, Active: true}).First(&binding).Error != nil {
		return Principal{}, code("identity_mapping_required")
	}
	var user User
	if s.DB.First(&user, "id = ?", binding.UserID).Error != nil || user.Disabled {
		return Principal{}, code("unauthorized")
	}
	c.token = token
	c.localTenant = binding.TenantID
	// An explicit deny is never treated as an outage. Only network/5xx failure
	// permits bounded read-only use of this already signature-verified context.
	online, denied := s.checkServiceIdentity(ctx, c)
	if denied {
		return Principal{}, code("unauthorized")
	}
	c.online = online
	p := Principal{UserID: binding.UserID, TenantID: binding.TenantID, ServiceIdentity: &c}
	if origin.PrincipalType == "service_account" {
		var agent Agent
		if binding.AgentID == "" || s.DB.First(&agent, "id = ? AND owner = ?", binding.AgentID, binding.UserID).Error != nil || agent.Revoked || !time.Now().Before(agent.Expires) {
			return Principal{}, code("forbidden")
		}
		p.Agent = &agent
	} else if binding.AgentID != "" {
		return Principal{}, code("forbidden")
	}
	return p, nil
}

func (s *Service) checkServiceIdentity(ctx context.Context, identity serviceIdentity) (online, denied bool) {
	data, _ := json.Marshal(map[string]string{"principal_context": identity.token, "audience": "drivebridge"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.serviceIdentityEndpoint(), "/")+"/api/v1/service-access/authorize", bytes.NewReader(data))
	if err != nil {
		return false, true
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return false, false
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return false, false
	}
	if res.StatusCode != 200 {
		return false, true
	}
	var response struct {
		Data serviceIdentity `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&response) != nil || response.Data.Subject != identity.Subject || response.Data.TenantID != identity.TenantID || !reflect.DeepEqual(response.Data.Credential, identity.Credential) {
		return false, true
	}
	return true, false
}

// serviceIdentityCodes are the stable error codes a client (Web or the MCP
// adapter) must handle on the unified-identity path.
var serviceIdentityCodes = []string{"unified_login_required", "identity_mapping_required", "identity_unavailable", "service_admission_required", "insufficient_funds", "cross_tenant_admin_context_required"}

// serviceIdentityCapability reports unified-identity discovery state for the
// HTTP capabilities endpoint and the MCP capabilities tool. The MCP adapter
// (the root CLI/MCP module) may advertise `aigora_url` support only when this reports
// "supported"; otherwise it must surface block_reason instead of pretending
// the path works. No credentials or token material are included.
func (s *Service) serviceIdentityCapability(p Principal) map[string]any {
	configured := s.Config.ServiceIdentityIssuer != ""
	cutover := false
	if p.TenantID != "" {
		var count int64
		if s.DB.Model(&ServiceIdentityBinding{}).Where("tenant_id = ? AND cutover = ? AND active = ?", p.TenantID, true, true).Count(&count).Error == nil {
			cutover = count > 0
		}
	}
	out := map[string]any{
		"configured":   configured,
		"cutover":      cutover,
		"aigora_url":   "blocked",
		"block_reason": "service_identity_not_configured",
		"error_codes":  serviceIdentityCodes,
	}
	if configured {
		out["aigora_url"] = "supported"
		out["block_reason"] = ""
	}
	return out
}

func (s *Service) serviceIdentityGuard(p Principal, action string) error {
	if p.ServiceIdentity == nil {
		var count int64
		if p.TenantID != "" {
			if s.DB.Model(&ServiceIdentityBinding{}).Where("tenant_id = ? AND cutover = ?", p.TenantID, true).Count(&count).Error != nil {
				return code("database_unavailable")
			}
		}
		if count > 0 {
			return code("unified_login_required")
		}
		return nil
	}
	c := p.ServiceIdentity
	if time.Now().Unix() >= c.ExpiresAt {
		return code("unauthorized")
	}
	read := false
	switch action {
	case "me", "tenant", "capabilities", "files.list", "files.stat", "files.versions", "files.usage", "spaces.list", "browser.list", "operations.get", "operations.list", "operations.inbox", "previews.list", "governance.overview", "agents.list", "connections.list", "creative.connections", "creative.receipts", "creative.status", "audit.list", "shares.list", "usage.files", "blobs.stat":
		read = true
	case "mcp.input.handoff_status", "mcp.files", "mcp.stat", "mcp.spaces", "mcp.operation_get", "mcp.capabilities", "mcp.search", "mcp.operations", "mcp.inbox", "mcp.usage_summary", "mcp.usage_series", "mcp.usage_breakdown", "mcp.usage_scans", "mcp.audit_events":
		read = true
	}
	if read {
		if !slices.Contains(c.Credential.Scopes, "drivebridge:read") {
			return code("forbidden")
		}
		return nil
	}
	if !c.online {
		return code("identity_unavailable")
	}
	if !slices.Contains(c.Credential.Scopes, "drivebridge:write") {
		return code("forbidden")
	}
	// Business admission is implemented at operation boundaries, never by
	// charging each HTTP request. Admission-bound actions pass the guard only
	// with a write scope; the boundary then either reserves usage with the
	// authority or rejects the write. Continuation actions only move bytes of
	// an already-admitted operation. Every other hosted write stays closed.
	if admissionBound(action) || admissionContinuation(action) {
		return nil
	}
	return code("service_admission_required")
}
