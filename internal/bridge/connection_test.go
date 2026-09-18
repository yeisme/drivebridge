package bridge

import (
	"os"
	"strings"
	"testing"
)

func newConnectionService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	// New requires a private state directory.
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func validConnection() Connection {
	return Connection{ID: "studio-a", Label: "Studio A", BaseURL: "https://db.example.test", InstanceID: "inst-1", CredentialSource: "env"}
}

func TestConnectionCRUD(t *testing.T) {
	s := newConnectionService(t)
	cn := validConnection()
	if err := s.AddConnection(cn); err != nil {
		t.Fatal(err)
	}
	got, err := s.Connection("studio-a")
	if err != nil || got.Label != "Studio A" {
		t.Fatalf("readback: %v %+v", err, got)
	}
	if err := s.SetConnection("studio-a", Connection{Label: "Studio A2", ProjectRef: "proj-9"}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Connection("studio-a")
	if got.Label != "Studio A2" || got.ProjectRef != "proj-9" || got.BaseURL != cn.BaseURL {
		t.Fatalf("partial update: %+v", got)
	}
	if err := s.RemoveConnection("studio-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Connection("studio-a"); err == nil {
		t.Fatal("soft-deleted connection must not resolve")
	}
	if len(s.Connections()) != 0 {
		t.Fatal("soft-deleted connection must not list")
	}
}

func TestConnectionValidation(t *testing.T) {
	s := newConnectionService(t)
	cases := []struct {
		name string
		cn   Connection
		want string
	}{
		{"bad id", Connection{ID: "Bad ID", Label: "x", BaseURL: "https://a.test", InstanceID: "i", CredentialSource: "env"}, "invalid_connection"},
		{"bad url", Connection{ID: "ok", Label: "x", BaseURL: "ftp://a.test", InstanceID: "i", CredentialSource: "env"}, "invalid_connection"},
		{"missing label", Connection{ID: "ok", Label: "", BaseURL: "https://a.test", InstanceID: "i", CredentialSource: "env"}, "invalid_connection"},
		{"keychain unimplemented", Connection{ID: "ok", Label: "x", BaseURL: "https://a.test", InstanceID: "i", CredentialSource: "keychain"}, "credential_source_unimplemented"},
		{"token-file without path", Connection{ID: "ok", Label: "x", BaseURL: "https://a.test", InstanceID: "i", CredentialSource: "token-file"}, "invalid_connection"},
		{"unknown space", Connection{ID: "ok", Label: "x", BaseURL: "https://a.test", InstanceID: "i", CredentialSource: "env", SpaceID: "nope"}, "invalid_connection"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.AddConnection(tc.cn); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
}

func TestConnectionDuplicateRejected(t *testing.T) {
	s := newConnectionService(t)
	if err := s.AddConnection(validConnection()); err != nil {
		t.Fatal(err)
	}
	if err := s.AddConnection(validConnection()); err == nil || !strings.Contains(err.Error(), "connection_conflict") {
		t.Fatalf("want connection_conflict, got %v", err)
	}
}
