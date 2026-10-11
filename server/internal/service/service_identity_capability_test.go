package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

func openFixtureService(t *testing.T, cfg Config) *Service {
	t.Helper()
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.State = state
	cfg.Enabled = true
	cfg.Development = true
	s, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestServiceIdentityCapabilityDiscovery(t *testing.T) {
	plain := openFixtureService(t, Config{})
	p := Principal{UserID: "local-user", TenantID: "local-org"}
	cap := plain.serviceIdentityCapability(p)
	if cap["configured"] != false || cap["aigora_url"] != "blocked" || cap["block_reason"] != "service_identity_not_configured" {
		t.Fatalf("unconfigured deployment must report a blocked aigora_url: %+v", cap)
	}
	codes, ok := cap["error_codes"].([]string)
	if !ok || len(codes) != 6 {
		t.Fatalf("error code contract must list six codes: %+v", cap)
	}

	configured := openFixtureService(t, Config{ServiceIdentityIssuer: "https://idp.fixture.example"})
	cap = configured.serviceIdentityCapability(p)
	if cap["configured"] != true || cap["aigora_url"] != "supported" || cap["block_reason"] != "" {
		t.Fatalf("configured deployment must advertise aigora_url support: %+v", cap)
	}
	if cap["cutover"] != false {
		t.Fatalf("no binding must not report cutover: %+v", cap)
	}
	if err := configured.DB.Create(&ServiceIdentityBinding{ID: "binding", Issuer: "https://idp.fixture.example", Subject: "sub", IdentityTenant: "org", UserID: "local-user", TenantID: "local-org", Active: true, Cutover: true}).Error; err != nil {
		t.Fatal(err)
	}
	if cap = configured.serviceIdentityCapability(p); cap["cutover"] != true {
		t.Fatalf("cutover binding must be reported: %+v", cap)
	}
	if cap = configured.serviceIdentityCapability(Principal{UserID: "other", TenantID: "other-tenant"}); cap["cutover"] != false {
		t.Fatalf("cutover must be scoped to the caller tenant: %+v", cap)
	}
}

func TestMCPCapabilitiesReportsServiceIdentity(t *testing.T) {
	s := openFixtureService(t, Config{})
	out, err := s.CallTool(context.Background(), Principal{UserID: "local-user"}, "capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("capabilities payload shape: %T", out)
	}
	cap, ok := payload["service_identity"].(map[string]any)
	if !ok || cap["aigora_url"] != "blocked" {
		t.Fatalf("MCP capabilities must expose the service_identity discovery contract: %+v", payload)
	}
}

func TestUnifiedIdentityErrorEnvelopeShape(t *testing.T) {
	expected := map[string]int{
		"unified_login_required":              403,
		"identity_mapping_required":           403,
		"identity_unavailable":                503,
		"service_admission_required":          503,
		"insufficient_funds":                  409,
		"cross_tenant_admin_context_required": 403,
	}
	for c, status := range expected {
		rec := httptest.NewRecorder()
		writeError(rec, code(c))
		if rec.Code != status {
			t.Fatalf("%s: status %d, want %d", c, rec.Code, status)
		}
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: envelope not JSON: %v", c, err)
		}
		if body.Error.Code != c || body.Error.Message == "" {
			t.Fatalf("%s: envelope must carry the code and a static message: %+v", c, body)
		}
	}
}
