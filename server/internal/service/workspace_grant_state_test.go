package service

import (
	"testing"
	"time"
)

func TestWorkspaceGrantStateBoundaries(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Second)
	base := WorkspaceGrant{RestrictionProfile: "personal_workspace_v1", ExpiryMode: "never"}
	cases := []struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{
		{"never", base, "active"},
		{"unknown-profile", WorkspaceGrant{ExpiryMode: "never"}, "invalid"},
		{"unknown-mode", WorkspaceGrant{RestrictionProfile: "personal_workspace_v1"}, "invalid"},
	}
	duration := base
	duration.ExpiryMode = "duration"
	duration.ExpiresAt = &future
	cases = append(cases, struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{"future", duration, "active"})
	duration.ExpiresAt = &now
	duration.Disabled = true
	cases = append(cases, struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{"expiry-equality-over-disabled", duration, "expired"})
	duration.Revoked = true
	cases = append(cases, struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{"revocation-over-expiry", duration, "revoked"})
	disabled := base
	disabled.Disabled = true
	cases = append(cases, struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{"disabled", disabled, "disabled"})
	invalid := base
	invalid.ExpiresAt = &future
	cases = append(cases, struct {
		name  string
		grant WorkspaceGrant
		want  string
	}{"never-with-date", invalid, "invalid"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := workspaceGrantState(tc.grant, now); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
