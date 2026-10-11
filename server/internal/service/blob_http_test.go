package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestBlobReuseHTTPAndBackupCrossReference(t *testing.T) {
	f := rig(t)
	alice, status := login(t, f, "alice")
	if status != 200 {
		t.Fatal(status)
	}
	bob, status := login(t, f, "bob")
	if status != 200 {
		t.Fatal(status)
	}
	_, c := agent(t, alice, "", "read,write")
	body := []byte("cross-owner identical payload for blob reuse")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	first, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "one.txt"}, "blob-1", bytes.NewReader(body), nil)
	if e != nil || first.Status != "completed" || first.BlobReused {
		t.Fatalf("first upload: %v %+v", e, first)
	}
	second, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "two.txt"}, "blob-2", bytes.NewReader(body), nil)
	if e != nil {
		t.Fatal(e)
	}
	if second.Status != "completed" || !second.BlobReused || second.Transferred != 0 {
		t.Fatalf("second upload must be a zero-byte instant hit: %+v", second)
	}
	// Bob's identical bytes are out of scope: no hit, a real transfer.
	_, bc := agent(t, bob, "", "read,write")
	third, e := bc.Upload(context.Background(), sdk.UploadRequest{SpaceID: bob.space.ID, Path: "three.txt"}, "blob-3", bytes.NewReader(body), nil)
	if e != nil || third.BlobReused || third.Transferred != int64(len(body)) {
		t.Fatalf("cross-owner upload must transfer bytes: %v %+v", e, third)
	}
	// Blob metadata endpoint: owner sees the count, others get 404.
	var info struct {
		SHA256 string `json:"sha256"`
		Refs   int64  `json:"ref_count"`
	}
	if status := alice.call(t, "GET", "/blobs/"+sha, nil, &info); status != 200 || info.SHA256 != sha || info.Refs != 2 {
		t.Fatalf("blob stat: status=%d %+v", status, info)
	}
	// A digest Bob never uploaded must not be confirmed to him.
	secret := []byte("alice-only payload")
	secretSum := sha256.Sum256(secret)
	secretSHA := hex.EncodeToString(secretSum[:])
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "secret.txt"}, "blob-4", bytes.NewReader(secret), nil); e != nil {
		t.Fatal(e)
	}
	if status := bob.call(t, "GET", "/blobs/"+secretSHA, nil, nil); status != 404 {
		t.Fatalf("unauthorized blob stat must be 404, got %d", status)
	}
	// Range reads stay on the existing media channel and never expose blob keys.
	req, _ := http.NewRequest("GET", alice.base+"/api/v1/files/"+second.FileID+"/media?version="+second.VersionID, nil)
	req.Header.Set("Range", "bytes=0-4")
	resp, e := alice.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	chunk, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 206 || !bytes.Equal(chunk, body[:5]) {
		t.Fatalf("range read: status=%d body=%q", resp.StatusCode, chunk)
	}
	// Backup manifest cross-references version→blob→object key.
	dir := stageBackup(t, f, "blob-backup")
	m, e := service.ReadBackupManifest(dir)
	if e != nil {
		t.Fatal(e)
	}
	// Three blobs exist in total: alice's shared pair, bob's own copy and
	// alice's secret file.
	if m.Counts["blobs"] != 3 {
		t.Fatalf("expected one shared blob in manifest counts: %+v", m.Counts)
	}
	blobEntries := 0
	for _, entry := range m.Entries {
		if entry.BlobSHA256 == sha {
			blobEntries++
		}
	}
	// Alice's two versions plus bob's own copy all reference the same
	// plaintext digest; each row keeps its own blob object key.
	if blobEntries != 3 {
		t.Fatalf("expected two blob-referencing entries, got %d", blobEntries)
	}
	if failures := service.CrossCheckManifest(dir, m); len(failures) != 0 {
		t.Fatalf("cross check: %v", failures)
	}
}
