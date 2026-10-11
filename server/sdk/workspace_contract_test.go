package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestWorkspaceGeneratedFixturesThroughSDK(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "personal-workspace", "fixtures", "responses.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/agent-grants/lookup":
			w.Write(fixtures["GrantReplayEnvelope"])
		default:
			w.WriteHeader(422)
			w.Write(fixtures["ErrorEnvelope"])
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "fixture-only")
	if err != nil {
		t.Fatal(err)
	}
	var replay contract.GrantReplayView
	if err = client.JSON(t.Context(), "POST", "/api/v1/agent-grants/lookup", "", map[string]string{"idempotency_key": "fixture-request"}, &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Credential.State != "not_replayable" || replay.Grant.ExpiresAt != nil {
		t.Fatal("SDK changed generated recovery response")
	}
	err = client.JSON(t.Context(), "POST", "/api/v1/agent-grants", "fixture-request", nil, nil)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "invalid_expiry" {
		t.Fatalf("unexpected error: %v", err)
	}
}
