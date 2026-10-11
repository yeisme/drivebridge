package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func receiverFixture(t *testing.T) (*Service, Share, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(Config{State: dir, Enabled: true, Development: true, ShareReceivers: true, Origin: "http://localhost"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	secret := ID()
	sealed, err := s.seal("receiver-owner", secret)
	if err != nil {
		t.Fatal(err)
	}
	v := Share{ID: ID(), Owner: "receiver-owner", Name: "Receiver fixture", Mode: "fixed", Audience: "link", Download: true, TokenHash: Hash(secret), Secret: sealed, Revision: 1}
	if err = s.DB.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	return s, v, secret
}

func receiverProof(secret, id, nonce string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(id + ":" + nonce))
	return hex.EncodeToString(m.Sum(nil))
}

func receiverOpen(t *testing.T, s *Service, v Share, secret string) ShareOpenResult {
	t.Helper()
	challenge, err := s.IssueShareChallenge(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFPShareReceiverProofReplayAndRestart(t *testing.T) {
	s, v, secret := receiverFixture(t)
	challenge, err := s.IssueShareChallenge(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	q := ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)}
	out, err := s.OpenShareProof(Principal{}, v.ID, q)
	if err != nil || out.ReceiverKind != "anonymous" || out.Token == secret {
		t.Fatalf("open: %+v %v", out, err)
	}
	var session ShareSession
	if err = s.DB.First(&session, "token_hash = ?", Hash(out.Token)).Error; err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(session)
	if bytes.Contains(encoded, []byte(out.Token)) || session.TokenHash == out.Token {
		t.Fatal("session secret exposed")
	}
	if _, err = s.OpenShareProof(Principal{}, v.ID, q); err == nil || err.Error() != "share_unavailable" {
		t.Fatal("nonce replay", err)
	}
	pending, _ := s.IssueShareChallenge(v.ID)
	// A fresh application instance has no challenges, but uses persisted sessions.
	restarted := &Service{DB: s.DB, Config: s.Config, crypt: s.crypt}
	if _, err = restarted.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: pending.Nonce, Proof: receiverProof(secret, v.ID, pending.Nonce)}); err == nil {
		t.Fatal("nonce revived")
	}
	if _, _, err = restarted.shareReceiverSession(s.DB, v, out.Token); err != nil {
		t.Fatal("persisted session lost", err)
	}
	s.DB.First(&v, "id = ?", v.ID)
	if v.OpensUsed != 1 {
		t.Fatal("replay counted", v.OpensUsed)
	}
}

func TestFPShareReceiverDevicesAndRevocation(t *testing.T) {
	s, v, secret := receiverFixture(t)
	opened := receiverOpen(t, s, v, secret)
	pub, private, _ := ed25519.GenerateKey(rand.Reader)
	q := ShareDeviceRequest{PublicKey: hex.EncodeToString(pub), Label: "Laptop"}
	device, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, q)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, q)
	if err != nil || again.ID != device.ID {
		t.Fatal("registration not idempotent", err)
	}
	challenge, _ := s.IssueShareChallenge(v.ID)
	visit, err := s.OpenShareWithDevice(Principal{}, v.ID, ShareDeviceOpenRequest{DeviceID: device.ID, Nonce: challenge.Nonce, Signature: hex.EncodeToString(ed25519.Sign(private, []byte(v.ID+":"+challenge.Nonce)))})
	if err != nil || visit.ReceiverKind != "device" {
		t.Fatal("device open", err)
	}
	if _, err = s.RegisterShareReceiverDevice(Principal{}, v.ID, visit.Token, q); err == nil {
		t.Fatal("device chained registration")
	}
	owner := Principal{UserID: v.Owner, SessionHash: "fixture-session"}
	if err = s.RevokeShareReceiver(owner, v.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.shareReceiverSession(s.DB, v, visit.Token); err == nil {
		t.Fatal("revoked device session survived")
	}
	again, err = s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, q)
	if err != nil || !again.Revoked || again.ID != device.ID {
		t.Fatal("registration resurrected device", err)
	}
	if _, _, err = s.shareReceiverSession(s.DB, v, opened.Token); err != nil {
		t.Fatal("unrelated session revoked", err)
	}
	if _, err = s.RegisterShareReceiverDevice(Principal{Agent: &Agent{}}, v.ID, opened.Token, q); err == nil {
		t.Fatal("agent registration")
	}
	if _, err = s.ListShareReceivers(Principal{UserID: v.Owner, Key: &UserKey{}}, v.ID); err == nil {
		t.Fatal("key managed receiver")
	}
}

func TestFPShareReceiverQuotaConcurrencyAndResume(t *testing.T) {
	s, v, secret := receiverFixture(t)
	limit := int64(3)
	if err := s.DB.Model(&v).Updates(map[string]any{"max_opens": limit, "max_downloads": 1}).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan ShareOpenResult, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			challenge, err := s.IssueShareChallenge(v.ID)
			if err != nil {
				return
			}
			out, err := s.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)})
			if err == nil {
				results <- out
			}
		}()
	}
	wg.Wait()
	close(results)
	if len(results) != 3 {
		t.Fatal("open quota", len(results))
	}
	var token string
	for out := range results {
		token = out.Token
	}
	file := File{ID: ID(), VersionID: ID()}
	for range 3 {
		if err := s.admitShareDownload(Principal{}, v.ID, token, file); err != nil {
			t.Fatal("resume", err)
		}
	}
	restarted := &Service{DB: s.DB, Config: s.Config, crypt: s.crypt}
	if err := restarted.admitShareDownload(Principal{}, v.ID, token, file); err != nil {
		t.Fatal("restart resume", err)
	}
	file.VersionID = ID()
	if err := s.admitShareDownload(Principal{}, v.ID, token, file); err == nil {
		t.Fatal("download quota bypass")
	}
	s.DB.First(&v, "id = ?", v.ID)
	if v.OpensUsed != 3 || v.DownloadsUsed != 1 || v.Revoked {
		t.Fatalf("quota drift: %+v", v)
	}
	if _, _, err := s.shareReceiverSession(s.DB, v, token); err != nil {
		t.Fatal("quota blocked list", err)
	}
}

func TestFPShareReceiverLegacyCookieAndHTTP(t *testing.T) {
	s, v, secret := receiverFixture(t)
	mux := http.NewServeMux()
	s.shareRoutes(mux)
	body := bytes.NewBufferString(`{"grant":"` + secret + `"}`)
	r := httptest.NewRequest("POST", "/api/v1/shared/"+v.ID+"/open", body)
	r.Header.Set("Origin", s.Config.Origin)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatal("legacy open", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if cookie.Name != shareCookie(v.ID) || cookie.Value == secret || bytes.Contains(w.Body.Bytes(), []byte(secret)) {
		t.Fatal("grant leaked")
	}
	for _, value := range []string{secret, cookie.Value} {
		r = httptest.NewRequest("GET", "/api/v1/shared/"+v.ID, nil)
		r.AddCookie(&http.Cookie{Name: shareCookie(v.ID), Value: value})
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal("cookie transition", w.Code, w.Body.String())
		}
	}
	challenge, _ := s.IssueShareChallenge(v.ID)
	s.mu.Lock()
	state := s.shareChallenges[Hash(challenge.Nonce)]
	state.Expires = time.Now().Add(-time.Second)
	s.shareChallenges[Hash(challenge.Nonce)] = state
	s.mu.Unlock()
	if _, err := s.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)}); err == nil {
		t.Fatal("expired nonce")
	}
}

func TestFPShareReceiverScale(t *testing.T) {
	if os.Getenv("DRIVEBRIDGE_SHARE_RECEIVER_SCALE") != "1" {
		t.Skip("set DRIVEBRIDGE_SHARE_RECEIVER_SCALE=1 for 1000 opens / 20 devices")
	}
	s, v, secret := receiverFixture(t)
	opened := receiverOpen(t, s, v, secret)
	for i := range 21 {
		public, _, _ := ed25519.GenerateKey(rand.Reader)
		_, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, ShareDeviceRequest{PublicKey: hex.EncodeToString(public)})
		if (err == nil) != (i < 20) {
			t.Fatal("device bound", i, err)
		}
	}
	for i := 1; i < 1000; i++ {
		receiverOpen(t, s, v, secret)
	}
	s.DB.First(&v, "id = ?", v.ID)
	if v.OpensUsed != 1000 {
		t.Fatal("scale count", v.OpensUsed)
	}
}
