package sdk

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFPShareReceiverSDKTransport(t *testing.T) {
	secret := "local-only-link-secret"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Origin") == "" || r.Header.Get("Authorization") != "" {
			t.Error("receiver transport authentication")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), secret) {
			t.Error("secret sent")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/challenge"):
			io.WriteString(w, `{"data":{"nonce":"challenge"}}`)
		case strings.HasSuffix(r.URL.Path, "/proof-open"):
			http.SetCookie(w, &http.Cookie{Name: "db_share_fixture", Value: "opaque-session", Path: "/api/v1/shared/fixture"})
			io.WriteString(w, `{"data":{"opened":true,"session_kind":"proof","receiver_kind":"anonymous"}}`)
		case strings.HasSuffix(r.URL.Path, "/devices"):
			cookie, err := r.Cookie("db_share_fixture")
			if err != nil || cookie.Value != "opaque-session" {
				t.Error("session not retained")
			}
			io.WriteString(w, `{"data":{"id":"device"}}`)
		case strings.HasSuffix(r.URL.Path, "/device-open"):
			io.WriteString(w, `{"data":{"opened":true,"session_kind":"device","receiver_kind":"device"}}`)
		default:
			t.Error("unexpected route", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewShareReceiverClient(server.URL, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := client.IssueShareChallenge(context.Background(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := client.OpenShareProof(context.Background(), "fixture", challenge.Nonce, secret, "")
	if err != nil || !opened.Opened {
		t.Fatal("proof adapter", err)
	}
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	device, err := client.RegisterShareReceiverDevice(context.Background(), "fixture", public, "Laptop")
	if err != nil {
		t.Fatal(err)
	}
	opened, err = client.OpenShareWithDevice(context.Background(), "fixture", device.ID, "challenge", private)
	if err != nil || opened.ReceiverKind != "device" || requests != 4 {
		t.Fatal("device adapter", err)
	}
}
