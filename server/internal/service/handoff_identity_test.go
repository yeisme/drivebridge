package service

import (
	"testing"
	"time"
)

func TestUnifiedHandoffMetadataKeepsReadScope(t *testing.T) {
	s := &Service{}
	p := Principal{ServiceIdentity: &serviceIdentity{ExpiresAt: time.Now().Add(time.Minute).Unix(), Credential: &serviceCredential{Scopes: []string{"drivebridge:read"}}}}
	if err := s.serviceIdentityGuard(p, "mcp.input.handoff_status"); err != nil {
		t.Fatal("metadata requires mutation admission", err)
	}
	for _, action := range []string{"mcp.input.handoff_renew", "mcp.input.handoff_cancel", "mcp.input.bind_reference", "handoff.read"} {
		if err := s.serviceIdentityGuard(p, action); err == nil {
			t.Fatal("read scope expanded to mutation or byte acquisition", action)
		}
	}
	p.ServiceIdentity.Credential.Scopes = nil
	if err := s.serviceIdentityGuard(p, "mcp.input.handoff_status"); err == nil {
		t.Fatal("missing read scope accepted")
	}
}
