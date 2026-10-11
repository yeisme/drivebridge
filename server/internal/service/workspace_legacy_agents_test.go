package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceLegacyAgentDefaultsAndArrayContract(t *testing.T) {
	f := rig(t)
	h, status := login(t, f, "alice")
	if status != 200 {
		t.Fatalf("login status %d", status)
	}
	before := time.Now().UTC()
	var created struct {
		Agent service.Agent `json:"agent"`
		Token string        `json:"token"`
	}
	if status = h.call(t, "POST", "/agents", map[string]any{"name": "legacy", "space_id": h.space.ID, "scopes": "read"}, &created); status != 200 {
		t.Fatalf("create status %d", status)
	}
	after := time.Now().UTC()
	if created.Agent.Expires.Before(before.Add(7*24*time.Hour)) || created.Agent.Expires.After(after.Add(7*24*time.Hour)) {
		t.Fatal("legacy default expiry changed")
	}
	if created.Token == "" || created.Agent.Scopes != "read" {
		t.Fatal("legacy response shape changed")
	}
	var raw json.RawMessage
	if status = h.call(t, "GET", "/agents", nil, &raw); status != 200 {
		t.Fatalf("list status %d", status)
	}
	var agents []service.Agent
	if err := json.Unmarshal(raw, &agents); err != nil {
		t.Fatal("legacy agent list must remain an array")
	}
	found := false
	for _, a := range agents {
		if a.ID == created.Agent.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("created legacy agent missing")
	}
	if status = h.call(t, "POST", "/agents", map[string]any{"name": "too-long", "space_id": h.space.ID, "scopes": "read", "days": 31}, nil); status < 400 {
		t.Fatal("legacy duration limit relaxed")
	}
}
