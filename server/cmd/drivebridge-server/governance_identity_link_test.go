package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func setupIdentityLinkFixture(t *testing.T) (dir, tenantID string, userIDs []string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	feedStdin(t, `[{"name":"identity link user"},{"name":"second user"}]`)
	out, err := capture(t, func() error { return runCLI("users", "import", "--state", dir, "--json") })
	if err != nil {
		t.Fatalf("users import: %v\n%s", err, out)
	}
	var envelope struct {
		Data struct {
			Issued []struct {
				UserID string `json:"user_id"`
			} `json:"issued"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || len(envelope.Data.Issued) != 2 {
		t.Fatalf("users import output: %v %q", err, out)
	}
	for _, issued := range envelope.Data.Issued {
		userIDs = append(userIDs, issued.UserID)
	}
	if out, err = capture(t, func() error { return runCLI("governance", "bootstrap", "--state", dir, "--user", userIDs[0], "--json") }); err != nil {
		t.Fatalf("governance bootstrap: %v\n%s", err, out)
	}
	var boot struct {
		Data struct {
			TenantID string `json:"tenant_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &boot); err != nil || boot.Data.TenantID == "" {
		t.Fatalf("governance bootstrap output: %v %q", err, out)
	}
	return dir, boot.Data.TenantID, userIDs
}

func TestGovernanceIdentityLinkCLI(t *testing.T) {
	dir, tenantID, userIDs := setupIdentityLinkFixture(t)
	userID, other := userIDs[0], userIDs[1]

	link := func(user, subject string, extra ...string) (string, error) {
		args := append([]string{"governance", "identity-link", "--state", dir, "--issuer", "https://idp.fixture.example", "--subject", subject, "--identity-tenant", "central-org", "--user", user, "--tenant", tenantID}, extra...)
		return capture(t, func() error { return runCLI(args...) })
	}

	// identity-link must fail for a tenant the user is not an active member of.
	out, err := capture(t, func() error {
		return runCLI("governance", "identity-link", "--state", dir, "--issuer", "https://idp.fixture.example", "--subject", "central-user", "--identity-tenant", "central-org", "--user", userID, "--tenant", "pending", "--json")
	})
	if err == nil {
		t.Fatalf("identity-link with a tenant the user is not a member of must fail: %s", out)
	}

	out, err = link(userID, "central-user", "--json")
	if err != nil {
		t.Fatalf("identity-link: %v\n%s", err, out)
	}
	if strings.Contains(out, "dbu_") {
		t.Fatalf("identity-link output must not contain key material: %s", out)
	}
	var linkEnvelope struct {
		Status string `json:"status"`
		Data   struct {
			UserID  string `json:"user_id"`
			Cutover bool   `json:"cutover"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &linkEnvelope); err != nil || linkEnvelope.Data.UserID != userID || linkEnvelope.Data.Cutover {
		t.Fatalf("identity-link output: %v %q", err, out)
	}

	// A second mapping for the same triple pointing at another user is rejected.
	if out, err = link(other, "central-user", "--json"); err == nil {
		t.Fatalf("reassigning an existing mapping must fail: %s", out)
	}

	// Cutover with an unmapped active member is rejected; once every active
	// member is mapped, cutover succeeds and is sticky.
	if out, err = link(userID, "central-user", "--cutover", "--json"); err == nil {
		t.Fatalf("cutover with an unmapped active member must fail: %s", out)
	}
	if out, err = link(other, "central-user-2", "--json"); err != nil {
		t.Fatalf("mapping the second member: %v\n%s", err, out)
	}
	out, err = link(userID, "central-user", "--cutover", "--json")
	if err != nil {
		t.Fatalf("cutover after full mapping: %v\n%s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &linkEnvelope); err != nil || !linkEnvelope.Data.Cutover {
		t.Fatalf("cutover output: %v %q", err, out)
	}
}
