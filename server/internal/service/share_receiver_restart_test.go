package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"
)

func TestFPShareReceiverPersistentRestart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := Config{State: dir, Enabled: true, Development: true, ShareReceivers: true}
	s, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			s.Close()
		}
	})
	secret := ID()
	sealed, err := s.seal("owner", secret)
	if err != nil {
		t.Fatal(err)
	}
	v := Share{ID: ID(), Owner: "owner", Audience: "link", TokenHash: Hash(secret), Secret: sealed}
	if err = s.DB.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	opened := receiverOpen(t, s, v, secret)
	file := File{ID: ID(), VersionID: ID()}
	if err = s.admitShareDownload(Principal{}, v.ID, opened.Token, file); err != nil {
		t.Fatal(err)
	}
	challenge, err := s.IssueShareChallenge(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	closed = true
	restarted, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, _, err = restarted.shareReceiverSession(restarted.DB, v, opened.Token); err != nil {
		t.Fatal("session did not persist", err)
	}
	if err = restarted.admitShareDownload(Principal{}, v.ID, opened.Token, file); err != nil {
		t.Fatal("resume did not persist", err)
	}
	if _, err = restarted.OpenShareProof(Principal{}, v.ID, ShareProofRequest{Nonce: challenge.Nonce, Proof: receiverProof(secret, v.ID, challenge.Nonce)}); err == nil {
		t.Fatal("challenge revived on boot")
	}
	if err = restarted.DB.First(&v, "id = ?", v.ID).Error; err != nil {
		t.Fatal(err)
	}
	if v.OpensUsed != 1 || v.DownloadsUsed != 1 {
		t.Fatal("restart count drift")
	}
}

func TestFPShareReceiverDeviceBoundAndIndependentSessions(t *testing.T) {
	s, v, secret := receiverFixture(t)
	opened := receiverOpen(t, s, v, secret)
	var devices []ShareDevice
	var visits []ShareOpenResult
	var wg sync.WaitGroup
	challenges := make(chan ShareChallenge, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			challenge, err := s.IssueShareChallenge(v.ID)
			if err == nil {
				challenges <- challenge
			}
		}()
	}
	wg.Wait()
	close(challenges)
	if len(challenges) != 20 {
		t.Fatal("concurrent challenges lost")
	}
	for challenge := range challenges {
		public, private, _ := ed25519.GenerateKey(rand.Reader)
		device, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, ShareDeviceRequest{PublicKey: hex.EncodeToString(public)})
		if err != nil {
			t.Fatal(err)
		}
		visit, err := s.OpenShareWithDevice(Principal{}, v.ID, ShareDeviceOpenRequest{DeviceID: device.ID, Nonce: challenge.Nonce, Signature: hex.EncodeToString(ed25519.Sign(private, []byte(v.ID+":"+challenge.Nonce)))})
		if err != nil {
			t.Fatal(err)
		}
		devices = append(devices, device)
		visits = append(visits, visit)
	}
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := s.RegisterShareReceiverDevice(Principal{}, v.ID, opened.Token, ShareDeviceRequest{PublicKey: hex.EncodeToString(public)}); err == nil || err.Error() != "invalid_share" {
		t.Fatal("device cap", err)
	}
	if err := s.RevokeShareReceiver(Principal{UserID: v.Owner, SessionHash: "session"}, v.ID, devices[0].ID); err != nil {
		t.Fatal(err)
	}
	for i, visit := range visits {
		_, _, err := s.shareReceiverSession(s.DB, v, visit.Token)
		if (err == nil) != (i != 0) {
			t.Fatal("device revocation scope", i, err)
		}
	}
}

func TestFPShareReceiverConcurrentDownloadAdmission(t *testing.T) {
	s, v, secret := receiverFixture(t)
	opened := receiverOpen(t, s, v, secret)
	if err := s.DB.Model(&v).Update("max_downloads", 1).Error; err != nil {
		t.Fatal(err)
	}
	file := File{ID: ID(), VersionID: ID()}
	results := make(chan error, 20)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.admitShareDownload(Principal{}, v.ID, opened.Token, file) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("concurrent resume", err)
		}
	}
	if err := s.DB.First(&v, "id = ?", v.ID).Error; err != nil {
		t.Fatal(err)
	}
	if v.DownloadsUsed != 1 {
		t.Fatal("concurrent resume overcharged", v.DownloadsUsed)
	}
}
