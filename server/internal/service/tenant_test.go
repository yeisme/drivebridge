package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func uploadPlain(t *testing.T, c *sdk.Client, h *human, path, content string) sdk.Operation {
	t.Helper()
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: path}, "tenant-"+path, bytes.NewReader([]byte(content)), nil)
	if e != nil {
		t.Fatal(e)
	}
	return op
}

func containsText(haystack []byte, needle string) bool {
	return bytes.Contains(haystack, []byte(needle))
}

func TestTenantProfileReadOnlySurface(t *testing.T) {
	f := rig(t)
	// No profile configured: the endpoint answers with a null tenant.
	anonymous, e := f.App.Client().Get(f.App.URL + "/api/v1/tenant")
	if e != nil {
		t.Fatal(e)
	}
	anonymous.Body.Close()
	if anonymous.StatusCode != 401 {
		t.Fatalf("anonymous tenant read returned %d", anonymous.StatusCode)
	}
	h, _ := login(t, f, "alice")
	var empty struct {
		Tenant         *service.TenantProfile `json:"tenant"`
		InstanceID     string                 `json:"instance_id"`
		InstanceRefPrf string                 `json:"instance_ref_prefix"`
	}
	if status := h.call(t, "GET", "/tenant", nil, &empty); status != 200 {
		t.Fatal(status)
	}
	if empty.Tenant != nil || empty.InstanceID != f.Service.Instance || empty.InstanceRefPrf != "drivebridge://"+f.Service.Instance+"/file/" {
		t.Fatalf("empty tenant response unexpected: %+v", empty)
	}
	// Profile appears once written (the deployment-as-tenant metadata).
	profile := service.TenantProfile{ID: "tenant", Name: "Family Vault", Origin: "https://vault.example.com", Contact: "ops@vault.example.com", Updated: time.Now().UTC()}
	if e = f.Service.DB.Create(&profile).Error; e != nil {
		t.Fatal(e)
	}
	var current struct {
		Tenant     *service.TenantProfile `json:"tenant"`
		InstanceID string                 `json:"instance_id"`
	}
	if status := h.call(t, "GET", "/tenant", nil, &current); status != 200 {
		t.Fatal(status)
	}
	if current.Tenant == nil || current.Tenant.Name != "Family Vault" || current.Tenant.Origin != "https://vault.example.com" {
		t.Fatalf("tenant profile not served: %+v", current)
	}
}

func TestTenantStartupParametersPersist(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := service.Open(service.Config{State: dir, TenantName: "Home", TenantOrigin: "https://home.example.com", TenantContact: "admin@home.example.com"})
	if e != nil {
		t.Fatal(e)
	}
	profile := s.Tenant()
	if profile == nil || profile.Name != "Home" || profile.Origin != "https://home.example.com" || profile.Contact != "admin@home.example.com" {
		t.Fatalf("startup parameters not persisted: %+v", profile)
	}
	key := s.Config.State
	s.Close()
	// Restarting with different parameters is the documented change path.
	reopened, e := service.Open(service.Config{State: key, TenantName: "Cottage"})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if p := reopened.Tenant(); p == nil || p.Name != "Cottage" || p.Origin != "" {
		t.Fatalf("downtime parameter change not applied: %+v", p)
	}
	// Invalid profiles refuse to start rather than half-apply.
	bad := t.TempDir()
	if e := os.Chmod(bad, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Open(service.Config{State: bad, TenantName: "x", TenantOrigin: "not a url"}); e == nil || e.Error() != "invalid_tenant_config" {
		t.Fatalf("invalid tenant config accepted: %v", e)
	}
}

func TestCrossInstanceReferenceRejected(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	op := uploadPlain(t, c, h, "shared.txt", "payload")
	p := service.Principal{UserID: h.user.ID}
	foreign := "drivebridge://" + service.ID() + "/file/" + op.FileID + "/version/" + op.VersionID
	if _, e := f.Service.CallTool(context.Background(), p, "stat", map[string]any{"ref": foreign}); e == nil || e.Error() != "cross_instance_rejected" {
		t.Fatalf("foreign reference accepted: %v", e)
	}
	if _, e := f.Service.CallTool(context.Background(), p, "prepare_reference", map[string]any{"ref": foreign}); e == nil || e.Error() != "cross_instance_rejected" {
		t.Fatalf("foreign reference handoff accepted: %v", e)
	}
	local := "drivebridge://" + f.Service.Instance + "/file/" + op.FileID + "/version/" + op.VersionID
	out, e := f.Service.CallTool(context.Background(), p, "stat", map[string]any{"ref": local})
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(out)
	if string(encoded) == "" || !containsText(encoded, op.FileID) {
		t.Fatal("local reference not resolved")
	}
	// Malformed refs never leak existence; a foreign-looking host is simply
	// a cross-instance rejection, an unparsable one is invalid_ref.
	for broken, want := range map[string]string{
		"": "invalid_ref",
		"drivebridge://" + f.Service.Instance + "/file/x/version/" + op.VersionID: "invalid_ref",
		"https://" + f.Service.Instance + "/file/" + op.FileID:                    "invalid_ref",
	} {
		if _, e := f.Service.CallTool(context.Background(), p, "stat", map[string]any{"ref": broken}); e == nil || e.Error() != want {
			t.Fatalf("malformed ref %q outcome: %v", broken, e)
		}
	}
}
