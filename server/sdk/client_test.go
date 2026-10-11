package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestReturnsTypedErrorWithMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "user_key_duplicate", "message": "The batch contains a key that already exists; nothing was written."}})
	}))
	defer server.Close()
	client, err := New(server.URL, "dbu_test")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err = client.JSON(t.Context(), "GET", "/api/v1/users", "", nil, &out)
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T %v", err, err)
	}
	if typed.Code != "user_key_duplicate" || err.Error() != "user_key_duplicate" {
		t.Fatalf("code passthrough broken: %+v", typed)
	}
	if !strings.Contains(typed.Message, "already exists") {
		t.Fatalf("message must survive: %q", typed.Message)
	}
}

func TestRequestCollapsesUnsafeCodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "boom: SELECT * FROM users; --"}})
	}))
	defer server.Close()
	client, _ := New(server.URL, "dbu_test")
	var out map[string]any
	err := client.JSON(t.Context(), "GET", "/api/v1/operations", "", nil, &out)
	if err == nil || err.Error() != "service_request_failed" {
		t.Fatalf("unsafe code must collapse, got %v", err)
	}
}

func TestRequestMessageAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "not_found"}})
	}))
	defer server.Close()
	client, _ := New(server.URL, "dbu_test")
	var out map[string]any
	err := client.JSON(t.Context(), "GET", "/api/v1/operations/x", "", nil, &out)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "not_found" || typed.Message != "" {
		t.Fatalf("missing message must stay empty: %+v", typed)
	}
}
