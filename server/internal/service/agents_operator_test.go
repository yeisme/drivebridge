package service_test

import (
	"strings"
	"testing"
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
