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

func TestHandoffCLIOriginalGrant(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-handoff-key" {
			t.Error("wrong caller identity")
		}
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"grant_id": "handoff:original", "cancelled": false}})
	}))
	defer server.Close()
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("fixture-handoff-key"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"status", "renew", "cancel"} {
		out, err := capture(t, func() error {
			return runCLI("handoff", action, "handoff:original", "--url", server.URL, "--token-file", key, "--json")
		})
		if err != nil || !strings.Contains(out, "handoff:original") || strings.Contains(out, "fixture-handoff-key") {
			t.Fatalf("%s: %v %s", action, err, out)
		}
	}
	if len(calls) != 3 || calls[0] != "GET /api/v1/handoffs/handoff:original" || calls[1] != "POST /api/v1/handoffs/handoff:original/renew" || calls[2] != "POST /api/v1/handoffs/handoff:original/cancel" {
		t.Fatal(calls)
	}
}
