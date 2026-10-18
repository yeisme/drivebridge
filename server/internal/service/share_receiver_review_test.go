package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestFPShareReceiverLegacyCookieAdmission(t *testing.T) {
	for _, mode := range []string{"new-unlimited", "new-single-use", "legacy-restricted", "legacy-unlimited"} {
		t.Run(mode, func(t *testing.T) {
			s, v, secret := receiverFixture(t)
			hash, err := bcrypt.GenerateFromPassword([]byte("fixture-passcode"), bcrypt.MinCost)
			if err != nil {
				t.Fatal(err)
			}
			v.Passcode = hash
			v.SessionRequired = mode == "new-unlimited" || mode == "new-single-use"
			if mode == "new-single-use" || mode == "legacy-restricted" {
				one := int64(1)
				v.MaxOpens, v.MaxDownloads, v.SingleUse = &one, &one, true
			}
			if err = s.DB.Save(&v).Error; err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			s.shareRoutes(mux)
			check := func(token string, admitted bool) {
				t.Helper()
				r := httptest.NewRequest(http.MethodGet, "/api/v1/shared/"+v.ID, nil)
				r.AddCookie(&http.Cookie{Name: shareCookie(v.ID), Value: token})
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				if (w.Code == http.StatusOK) != admitted {
					t.Fatalf("cookie admitted=%v status=%d", admitted, w.Code)
				}
			}
			legacy := mode == "legacy-unlimited"
			check(secret, legacy)
			if _, err = s.openShareReceiver(Principal{}, v.ID, "grant", secret, ShareProofRequest{Passcode: "wrong"}, ShareDeviceOpenRequest{}); err == nil {
				t.Fatal("passcode bypass")
			}
			opened, err := s.openShareReceiver(Principal{}, v.ID, "grant", secret, ShareProofRequest{Passcode: "fixture-passcode"}, ShareDeviceOpenRequest{})
			if err != nil {
				t.Fatal(err)
			}
			check(opened.Token, true)
			check(secret, legacy)
			if !legacy {
				if err = s.admitShareDownload(Principal{}, v.ID, secret, File{ID: ID(), VersionID: ID()}); err == nil {
					t.Fatal("raw grant admitted download")
				}
			}
			if err = s.DB.First(&v, "id = ?", v.ID).Error; err != nil {
				t.Fatal(err)
			}
			if v.OpensUsed != 1 || v.DownloadsUsed != 0 {
				t.Fatal("rejected grant mutated quotas")
			}
		})
	}
}

func TestFPShareReceiverBrowserChallengeFlow(t *testing.T) {
	s, v, secret := receiverFixture(t)
	mux := http.NewServeMux()
	s.shareRoutes(mux)
	challenge := func() ShareChallenge {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/shared/"+v.ID+"/challenge", nil)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("Sec-Fetch-Mode", "cors")
		r.Header.Set("Sec-Fetch-Dest", "empty")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatal("browser challenge", w.Code, w.Body.String())
		}
		var envelope struct {
			Data ShareChallenge `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	post := func(path string, input any, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/shared/"+v.ID+path, bytes.NewReader(body))
		r.Header.Set("Origin", s.Config.Origin)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatal("browser mutation", path, w.Code, w.Body.String())
		}
		return w
	}
	nonce := challenge().Nonce
	opened := post("/proof-open", ShareProofRequest{Nonce: nonce, Proof: receiverProof(secret, v.ID, nonce)}, nil)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registered := post("/devices", ShareDeviceRequest{PublicKey: hex.EncodeToString(public)}, opened.Result().Cookies()[0])
	var envelope struct {
		Data ShareDevice `json:"data"`
	}
	if err = json.Unmarshal(registered.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	nonce = challenge().Nonce
	post("/device-open", ShareDeviceOpenRequest{DeviceID: envelope.Data.ID, Nonce: nonce, Signature: hex.EncodeToString(ed25519.Sign(private, []byte(v.ID+":"+nonce)))}, nil)
	for _, test := range []struct{ method, origin, site string }{
		{http.MethodGet, "", "cross-site"},
		{http.MethodGet, "", "same-site"},
		{http.MethodGet, "https://foreign.invalid", "same-origin"},
		{http.MethodGet, s.Config.Origin, "cross-site"},
		{http.MethodPost, "", "same-origin"},
	} {
		path := "/challenge"
		if test.method == http.MethodPost {
			path = "/proof-open"
		}
		r := httptest.NewRequest(test.method, "/api/v1/shared/"+v.ID+path, bytes.NewBufferString(`{}`))
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Sec-Fetch-Site", test.site)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatal("cross-origin receiver admission", test, w.Code)
		}
	}
}
