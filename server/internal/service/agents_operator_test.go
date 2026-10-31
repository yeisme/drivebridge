package service_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestCreateAgentOperatorDerivesOwnerFromSpace(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, secret, e := f.Service.CreateAgentOperator(h.space.ID, "ops-agent", "", "read,write", "", 3)
	if e != nil {
		t.Fatal(e)
	}
	if a.Owner != h.user.ID {
		t.Fatalf("owner must be derived from the space row: got %q want %q", a.Owner, h.user.ID)
	}
	if !strings.HasPrefix(secret, "db_") || len(secret) < 32 {
		t.Fatalf("one-shot token malformed: %q", secret[:4])
	}
	if a.SpaceID != h.space.ID || a.Scopes != "read,write" {
		t.Fatalf("agent fields: %+v", a)
	}
	// The token authenticates as the derived owner's agent.
	if _, _, e = f.Service.CreateAgentOperator("missing-space", "x", "", "", "", 1); e == nil || e.Error() != "not_found" {
		t.Fatalf("unknown space must be not_found, got %v", e)
	}
}

// Accounts that join after governance own their data through a separate data owner id, so their spaces do not carry
// the user id. The operator path must issue as that user, and the token must then work under governance.
func TestCreateAgentOperatorResolvesGovernedDataOwner(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	dataOwner := service.ID()
	if e := f.Service.DB.Model(&service.Membership{}).Where("tenant_id = ? AND user_id = ?", f.Service.Instance, alice.user.ID).Update("data_owner", dataOwner).Error; e != nil {
		t.Fatal(e)
	}
	space := service.Space{ID: service.ID(), Owner: dataOwner, TenantID: f.Service.Instance, Name: "notes"}
	if e := f.Service.DB.Create(&space).Error; e != nil {
		t.Fatal(e)
	}
	a, secret, e := f.Service.CreateAgentOperator(space.ID, "control-center", "", "read", "", 30)
	if e != nil {
		t.Fatalf("governed space owned by a data owner must be issuable: %v", e)
	}
	if a.Owner != alice.user.ID || a.SpaceID != space.ID || a.Scopes != "read" {
		t.Fatalf("agent must belong to the member user: %+v", a)
	}
	req, _ := http.NewRequest("GET", f.App.URL+"/api/v1/operations?limit=5", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("issued token must read the space's operations under governance, got %d", res.StatusCode)
	}
	// A data owner no active member holds is never issued an agent.
	orphan := service.Space{ID: service.ID(), Owner: service.ID(), TenantID: f.Service.Instance, Name: "orphan"}
	if e := f.Service.DB.Create(&orphan).Error; e != nil {
		t.Fatal(e)
	}
	if _, _, e := f.Service.CreateAgentOperator(orphan.ID, "x", "", "read", "", 1); e == nil || e.Error() != "forbidden" {
		t.Fatalf("unheld data owner must be forbidden, got %v", e)
	}
}
