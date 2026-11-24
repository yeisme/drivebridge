package service_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestOneTimeTokenConsumedOnceAndSessionSurvivesUntilRevoked(t *testing.T) {
	f := rig(t)
	issued, err := f.Service.ImportUsers([]service.UserImport{
		{Name: "Guest Invite", OneTime: true},
		{Name: "Host Owner"},
	})
	if err != nil {
		t.Fatal(err)
	}
	oneTimeKey := issued[0].Key
	if !issued[0].OneTime || !strings.HasPrefix(oneTimeKey, "dbu_") {
		t.Fatal("expected one_time key metadata")
	}

	// Direct Bearer API reuse is rejected for one-time invitation tokens;
	// callers must exchange through POST /auth/key so consumption is atomic.
	if st := bearerCall(t, f, "GET", "/me", oneTimeKey, nil, nil); st != 401 {
		t.Fatalf("one-time bearer should be rejected, got %d", st)
	}

	exchange := func(secret string) (*http.Client, int) {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		body, _ := json.Marshal(map[string]string{"key": secret})
		req, _ := http.NewRequest("POST", f.App.URL+"/auth/key", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", f.Service.Config.Origin)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return client, resp.StatusCode
	}

	// First browser exchange succeeds and establishes a working session.
	firstClient, code := exchange(oneTimeKey)
	if code != 200 {
		t.Fatalf("first one-time login failed: %d", code)
	}
	meResp, err := firstClient.Get(f.App.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	meResp.Body.Close()
	if meResp.StatusCode != 200 {
		t.Fatalf("session from one-time token should stay active, got %d", meResp.StatusCode)
	}

	// Second exchange with the exact same ?token= is immediately rejected (single-use).
	_, secondCode := exchange(oneTimeKey)
	if secondCode != 401 {
		t.Fatalf("second one-time login must be rejected, got %d", secondCode)
	}

	// Verify sealed material was wiped upon consumption.
	verified, err := f.Service.VerifyUserKey(oneTimeKey)
	if err != nil {
		t.Fatal(err)
	}
	if !verified.OneTime || !verified.Consumed || len(verified.SealedKey) != 0 {
		t.Fatal("consumed one-time key must mark consumed=true and wipe sealed key")
	}

	// Self-service temporary one-time invite key under an existing owner can be
	// revoked without tripping last_active_key, and revoking kills the guest session.
	var selfInvite struct {
		ID      string `json:"id"`
		Key     string `json:"key"`
		OneTime bool   `json:"one_time"`
	}
	if st := bearerCall(t, f, "POST", "/key-directory", issued[1].Key, map[string]any{"name": "Friend invite", "days": 7, "one_time": true}, &selfInvite); st != 200 || !selfInvite.OneTime {
		t.Fatalf("self one-time key creation failed: %d", st)
	}
	guestClient, guestCode := exchange(selfInvite.Key)
	if guestCode != 200 {
		t.Fatalf("guest one-time exchange failed: %d", guestCode)
	}
	if st := bearerCall(t, f, "POST", "/key-directory/"+selfInvite.ID+"/revoke", issued[1].Key, map[string]any{"confirm": true}, nil); st != 200 {
		t.Fatalf("revoking consumed one-time key failed: %d", st)
	}
	revokedCheck, err := guestClient.Get(f.App.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	revokedCheck.Body.Close()
	if revokedCheck.StatusCode != 401 {
		t.Fatalf("revoking one-time key must terminate its active session, got %d", revokedCheck.StatusCode)
	}
}
