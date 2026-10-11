package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusLocalProjection(t *testing.T) {
	dir := seedAgentState(t)
	c := operatorCLI(t, dir)
	cmd := newStatusCommand(c)
	out := captureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("local status: %v", err)
		}
	})
	var envelope struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
		Data    struct {
			Mode  string `json:"mode"`
			State struct {
				Database bool `json:"database"`
			} `json:"state"`
			Spaces   int `json:"spaces"`
			UserKeys struct {
				Total int `json:"total"`
				Live  int `json:"live"`
			} `json:"user_keys"`
			Quota struct {
				Max  int64 `json:"max_bytes"`
				Used int64 `json:"used_bytes"`
			} `json:"quota"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(out), &envelope) != nil || envelope.Status != "success" || envelope.Data.Mode != "local" {
		t.Fatalf("local status envelope: %s", out)
	}
	if !envelope.Data.State.Database || envelope.Data.Spaces != 1 {
		t.Fatalf("state projection: %s", out)
	}
	if envelope.Data.UserKeys.Live != 0 || !strings.Contains(envelope.Summary, "Recommended next step:") {
		t.Fatalf("empty deployment must point at the next step: %s", out)
	}
	if envelope.Data.Quota.Max != 1<<20 {
		t.Fatalf("quota sums: %+v", envelope.Data.Quota)
	}
}

func TestStatusRemoteProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "ok", "enabled": true}})
		case "/api/v1/me":
			if r.Header.Get("Authorization") != "Bearer dbu_live" {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{"name": "ops"}, "admin": true}})
		case "/api/v1/spaces":
			if r.Header.Get("Authorization") != "Bearer dbu_live" {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "sp-1"}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	token := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(token, []byte("dbu_live"), 0600); e != nil {
		t.Fatal(e)
	}
	c := operatorCLI(t, t.TempDir())
	cmd := newStatusCommand(c)
	if e := cmd.Flags().Set("url", srv.URL); e != nil {
		t.Fatal(e)
	}
	if e := cmd.Flags().Set("token-file", token); e != nil {
		t.Fatal(e)
	}
	out := captureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("remote status: %v", err)
		}
	})
	var envelope struct {
		Data struct {
			Mode      string `json:"mode"`
			Reachable bool   `json:"reachable"`
			Identity  struct {
				Name  string `json:"name"`
				Admin bool   `json:"admin"`
			} `json:"identity"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(out), &envelope) != nil || !envelope.Data.Reachable || envelope.Data.Identity.Name != "ops" || !envelope.Data.Identity.Admin {
		t.Fatalf("remote status envelope: %s", out)
	}
}

func TestStatusRemoteRejectsLeakyTokenFile(t *testing.T) {
	token := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(token, []byte("dbu_live"), 0644); e != nil {
		t.Fatal(e)
	}
	c := operatorCLI(t, t.TempDir())
	cmd := newStatusCommand(c)
	_ = cmd.Flags().Set("url", "https://example.invalid")
	_ = cmd.Flags().Set("token-file", token)
	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "credential_unavailable") {
		t.Fatalf("group-readable token file must be rejected, got %v", err)
	}
}
