package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestFPShareReceiverThrottleIsolation(t *testing.T) {
	s, v, secret := receiverFixture(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-passcode"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Model(&v).Updates(map[string]any{"passcode": hash, "failures": 2}).Error; err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		challenge, err := s.IssueShareChallenge(v.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: "00", Passcode: "correct-passcode"})
		want := "share_unavailable"
		if i == 9 {
			want = "throttled"
		}
		if err == nil || err.Error() != want {
			t.Fatal("proof failure", i, err)
		}
	}
	s.DB.First(&v, "id = ?", v.ID)
	if v.Failures != 2 || v.OpensUsed != 0 {
		t.Fatal("proof touched passcode or quota")
	}
	out, err := s.openShareReceiver(Principal{}, v.ID, "grant", secret, ShareProofRequest{Passcode: "correct-passcode"}, ShareDeviceOpenRequest{})
	if err != nil || !out.Opened {
		t.Fatal("proof lock blocked grant", err)
	}
	if err = s.DB.Model(&v).Update("proof_locked_until", time.Time{}).Error; err != nil {
		t.Fatal(err)
	}
	challenge, _ := s.IssueShareChallenge(v.ID)
	_, err = s.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce), Passcode: "wrong"})
	if err == nil || err.Error() != "share_passcode_required" {
		t.Fatal("passcode failure", err)
	}
	s.DB.First(&v, "id = ?", v.ID)
	if v.ProofFailures != 0 || v.Failures != 1 || v.OpensUsed != 1 {
		t.Fatal("isolated counters drifted")
	}
}

func TestFPShareReceiverUserAudienceAndAgentDenial(t *testing.T) {
	s, v, secret := receiverFixture(t)
	if err := s.DB.Model(&v).Updates(map[string]any{"audience": "users", "recipients": "receiver-user"}).Error; err != nil {
		t.Fatal(err)
	}
	v.Audience, v.Recipients = "users", "receiver-user"
	challenge, _ := s.IssueShareChallenge(v.ID)
	q := ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce), ReceiverKind: "device"}
	if _, err := s.OpenShareProof(Principal{}, v.ID, q); err == nil || err.Error() != "forbidden" {
		t.Fatal("anonymous users share", err)
	}
	challenge, _ = s.IssueShareChallenge(v.ID)
	q.Nonce, q.Proof = challenge.Nonce, receiverProof(secret, v.ID, challenge.Nonce)
	out, err := s.OpenShareProof(Principal{UserID: "receiver-user"}, v.ID, q)
	if err != nil || out.ReceiverKind != "user" {
		t.Fatal("user projection", err)
	}
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err = s.RegisterShareReceiverDevice(Principal{UserID: "receiver-user"}, v.ID, out.Token, ShareDeviceRequest{PublicKey: hex.EncodeToString(public)}); err == nil || err.Error() != "invalid_share" {
		t.Fatal("user device binding", err)
	}
	if _, err = s.OpenShareWithDevice(Principal{Agent: &Agent{}}, v.ID, ShareDeviceOpenRequest{}); err == nil || err.Error() != "forbidden" {
		t.Fatal("agent device open", err)
	}
	if err = s.RevokeShareReceiver(Principal{UserID: v.Owner, Agent: &Agent{}, SessionHash: "session"}, v.ID, "unknown"); err == nil || err.Error() != "forbidden" {
		t.Fatal("agent management", err)
	}
}

func TestFPShareReceiverRollbackAndUnavailable(t *testing.T) {
	s, v, secret := receiverFixture(t)
	if err := s.DB.Callback().Create().Before("gorm:create").Register("receiver_session_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "share_sessions" {
			tx.AddError(errors.New("injected session write failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	challenge, _ := s.IssueShareChallenge(v.ID)
	q := ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)}
	if _, err := s.OpenShareProof(Principal{}, v.ID, q); err == nil {
		t.Fatal("failed insert succeeded")
	}
	s.DB.Callback().Create().Remove("receiver_session_failure")
	s.DB.First(&v, "id = ?", v.ID)
	if v.OpensUsed != 0 {
		t.Fatal("failed issuance debited quota")
	}
	if _, err := s.OpenShareProof(Principal{}, v.ID, q); err == nil {
		t.Fatal("failed transaction revived nonce")
	}
	for _, id := range []string{v.ID, ID()} {
		challenge, _ := s.IssueShareChallenge(id)
		_, err := s.OpenShareWithDevice(Principal{}, id, ShareDeviceOpenRequest{DeviceID: ID(), Nonce: challenge.Nonce, Signature: "00"})
		if err == nil || err.Error() != "share_unavailable" {
			t.Fatal("unknown identity distinguishable", err)
		}
	}
	opened := receiverOpen(t, s, v, secret)
	if err := s.DB.Model(&v).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	s.DB.First(&v, "id = ?", v.ID)
	if _, _, err := s.shareReceiverSession(s.DB, v, opened.Token); err == nil {
		t.Fatal("share revocation lost")
	}
	if _, err := s.IssueShareChallenge(v.ID); err == nil || err.Error() != "share_unavailable" {
		t.Fatal("revoked challenge", err)
	}
}

func TestFPShareReceiverDeliveryProjection(t *testing.T) {
	s, v, secret := receiverFixture(t)
	opened := receiverOpen(t, s, v, secret)
	file := File{ID: ID(), VersionID: ID()}
	if err := s.observeShareReceiverDownload(Principal{}, v, opened.Token, file); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB.Model(&DeliveryReceipt{}).Count(&count)
	if count != 0 {
		t.Fatal("fabricated missing-manifest receipt")
	}
	manifest := DeliveryManifest{ID: ID(), Owner: v.Owner, ShareID: v.ID, Revision: 1, Phase: "published", Key: ID()}
	if err := s.DB.Create(&manifest).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&DeliveryManifestItem{ID: ID(), ManifestID: manifest.ID, FileID: file.ID, VersionID: file.VersionID}).Error; err != nil {
		t.Fatal(err)
	}
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	device, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, ShareDeviceRequest{PublicKey: hex.EncodeToString(public)})
	if err != nil {
		t.Fatal(err)
	}
	challenge, _ := s.IssueShareChallenge(v.ID)
	visit, err := s.OpenShareWithDevice(Principal{}, v.ID, ShareDeviceOpenRequest{DeviceID: device.ID, Nonce: challenge.Nonce, Signature: hex.EncodeToString(ed25519.Sign(private, []byte(v.ID+":"+challenge.Nonce)))})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.observeShareReceiverDownload(Principal{}, v, visit.Token, file); err != nil {
			t.Fatal(err)
		}
	}
	var receipts []DeliveryReceipt
	if err = s.DB.Find(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Kind != "receiver_verified" || receipts[0].ReceiverKind != "device" || receipts[0].ReceiverRef != device.Fingerprint {
		t.Fatalf("wrong receiver observation %+v", receipts)
	}
}

func TestFPShareReceiverHTTPProofSecrecyAndGates(t *testing.T) {
	s, v, secret := receiverFixture(t)
	mux := http.NewServeMux()
	s.shareRoutes(mux)
	challenge, _ := s.IssueShareChallenge(v.ID)
	q := ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce), ReceiverKind: "user"}
	body, _ := json.Marshal(q)
	if bytes.Contains(body, []byte(secret)) {
		t.Fatal("proof sends link secret")
	}
	r := httptest.NewRequest("POST", "/api/v1/shared/"+v.ID+"/proof-open", bytes.NewReader(body))
	r.Header.Set("Origin", s.Config.Origin)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"receiver_kind":"anonymous"`)) || bytes.Contains(w.Body.Bytes(), []byte(secret)) || bytes.Contains([]byte(w.Header().Get("Set-Cookie")), []byte(secret)) {
		t.Fatal("proof contract", w.Code, w.Body.String())
	}
	for _, origin := range []string{"", "https://foreign.invalid"} {
		r = httptest.NewRequest("GET", "/api/v1/shared/"+v.ID+"/challenge", nil)
		r.Header.Set("Origin", origin)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal("origin bypass")
		}
	}
	s.Config.ShareReceivers = false
	r = httptest.NewRequest("GET", "/api/v1/shared/"+v.ID+"/challenge", nil)
	r.Header.Set("Origin", s.Config.Origin)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"unsupported_action"`)) {
		t.Fatal("feature not gated", w.Body.String())
	}
}

func TestFPShareReceiverStreamAdmission(t *testing.T) {
	for _, status := range []int{http.StatusRequestedRangeNotSatisfiable, http.StatusNotModified} {
		called := false
		w := &shareDownloadWriter{ResponseWriter: httptest.NewRecorder(), admit: func() error { called = true; return nil }}
		w.WriteHeader(status)
		if called {
			t.Fatal("non-stream charged", status)
		}
	}
	base := httptest.NewRecorder()
	w := &shareDownloadWriter{ResponseWriter: base, admit: func() error { return code("share_unavailable") }}
	if n, err := w.Write([]byte("private bytes")); n != 0 || err == nil || base.Body.Len() != 0 {
		t.Fatal("quota rejection transmitted bytes")
	}
}
