package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// stageBackup copies the two shipped pieces into a fresh 0700 directory and
// writes the manifest from the shipped database.
func stageBackup(t *testing.T, f *fixtures.Runtime, name string) string {
	t.Helper()
	if e := f.Service.CheckpointMetadata(); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(t.TempDir(), name)
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, file := range []string{"metadata.sqlite", "instance.id"} {
		b, e := os.ReadFile(filepath.Join(f.Service.Config.State, file))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, file), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := service.WriteBackupManifest(dir); e != nil {
		t.Fatal(e)
	}
	return dir
}

func TestBackupManifestVerifyAndFailureModes(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	one, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "plain/one.txt"}, "plain-1", bytes.NewReader([]byte("first")), nil)
	if e != nil {
		t.Fatal(e)
	}
	vault := encryptedSpace(t, h, "Vault")
	vc, _ := vaultAgent(t, h, vault.ID)
	two, e := vc.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/two.bin"}, "vault-2", bytes.NewReader([]byte("second")), nil)
	if e != nil {
		t.Fatal(e)
	}
	dir := stageBackup(t, f, "backup")
	manifest, e := service.ReadBackupManifest(dir)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.SchemaVersion != 1 || manifest.InstanceID != f.Service.Instance || len(manifest.Entries) != 2 || manifest.Counts["encrypted_spaces"] != 1 {
		t.Fatalf("manifest unexpected: %+v", manifest)
	}
	if mode := os.FileMode(0600); fileMode(t, filepath.Join(dir, "manifest.json")) != mode {
		t.Fatal("manifest not private")
	}
	result, e := f.Service.VerifyBackup(context.Background(), dir, 50)
	if e != nil {
		t.Fatal(e)
	}
	if !result.Ready || result.ManifestEntries != 2 || result.Sampled != 2 || result.DBVersions != 2 {
		t.Fatalf("clean verification failed: %+v", result)
	}
	// Stage 1: tampered manifest digest.
	manifest.Entries[0].SHA256 = service.Hash("tampered")
	body, _ := json.Marshal(manifest)
	if e = os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0600); e != nil {
		t.Fatal(e)
	}
	result, e = f.Service.VerifyBackup(context.Background(), dir, 50)
	if e != nil {
		t.Fatal(e)
	}
	if result.Ready || !containsFailure(result.Failures, "manifest_mismatch:sha256:") {
		t.Fatalf("tampered manifest accepted: %+v", result)
	}
	// Stage 2: sampled object missing at the storage layer.
	dir2 := stageBackup(t, f, "backup2")
	manifest2, _ := service.ReadBackupManifest(dir2)
	var version service.Version
	f.Service.DB.First(&version, "id = ?", two.VersionID)
	for key := range f.Objects() {
		if len(key) > len(version.ObjectKey) && key[len(key)-len(version.ObjectKey):] == version.ObjectKey {
			f.DropObject(key)
		}
	}
	result, e = f.Service.VerifyBackup(context.Background(), dir2, 50)
	if e != nil {
		t.Fatal(e)
	}
	if result.Ready || !containsFailure(result.Failures, "object_missing:") {
		t.Fatalf("missing object accepted: %+v", result)
	}
	// Stage 3: a backup from another instance is refused.
	manifest2.InstanceID = service.ID()
	body, _ = json.Marshal(manifest2)
	os.WriteFile(filepath.Join(dir2, "manifest.json"), body, 0600)
	result, e = f.Service.VerifyBackup(context.Background(), dir2, 50)
	if e != nil {
		t.Fatal(e)
	}
	if result.Ready || !containsFailure(result.Failures, "manifest_mismatch:instance_id") {
		t.Fatalf("foreign instance accepted: %+v", result)
	}
	_ = manifest
	_ = one
}

func containsFailure(failures []string, prefix string) bool {
	for _, failure := range failures {
		if len(failure) >= len(prefix) && failure[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, e := os.Lstat(path)
	if e != nil {
		t.Fatal(e)
	}
	return info.Mode().Perm()
}

func TestBackupBundleEncryptDecryptRoundtrip(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "bundle.txt"}, "bundle", bytes.NewReader([]byte("sealed content")), nil); e != nil {
		t.Fatal(e)
	}
	dir := stageBackup(t, f, "bundle")
	files := map[string][]byte{}
	for _, name := range []string{"metadata.sqlite", "instance.id", "manifest.json"} {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		files[name] = b
	}
	blob, e := f.Service.SealBundle(files)
	if e != nil {
		t.Fatal(e)
	}
	if string(blob[:4]) != "DBB1" {
		t.Fatal("bundle magic missing")
	}
	for name, body := range files {
		if bytes.Contains(blob, body) {
			t.Fatalf("bundle leaks plaintext for %s", name)
		}
	}
	opened, e := f.Service.OpenBundle(blob)
	if e != nil {
		t.Fatal(e)
	}
	if len(opened) != len(files) {
		t.Fatal("bundle lost files")
	}
	for name, body := range files {
		if string(opened[name]) != string(body) {
			t.Fatalf("bundle file %s differs", name)
		}
	}
	// A different instance key cannot open the bundle.
	other := t.TempDir()
	os.Chmod(other, 0700)
	foreign, e := service.Open(service.Config{State: other})
	if e != nil {
		t.Fatal(e)
	}
	defer foreign.Close()
	if _, e = foreign.OpenBundle(blob); e == nil || e.Error() != "content_mismatch" {
		t.Fatalf("foreign key opened the bundle: %v", e)
	}
	blob[20] ^= 0x20
	if _, e = f.Service.OpenBundle(blob); e == nil {
		t.Fatal("tampered bundle accepted")
	}
}

// TestRestoreDrillScriptedSmoke is the scripted restore drill from the
// deployment runbook: backup, restore into an isolated state directory with
// the escrowed key, verify read-only, then boot without --enabled.
func TestRestoreDrillScriptedSmoke(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	vc, _ := vaultAgent(t, h, vault.ID)
	op, e := vc.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/drill.bin"}, "drill", bytes.NewReader([]byte("drill payload")), nil)
	if e != nil {
		t.Fatal(e)
	}
	backup := stageBackup(t, f, "drill")
	// Key escrow retrieval: the drill copies the key from the live state as
	// the stand-in for the separate secret channel.
	key, e := os.ReadFile(filepath.Join(f.Service.Config.State, "instance.key"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(backup, "instance.key"), key, 0600); e != nil {
		t.Fatal(e)
	}
	result, e := f.Service.VerifyBackup(context.Background(), backup, 50)
	if e != nil || !result.Ready {
		t.Fatalf("drill verification failed: %+v %v", result, e)
	}
	restored, e := service.Open(service.Config{State: backup, Development: true})
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	p := service.Principal{UserID: h.user.ID}
	_, _, body, e := restored.Download(context.Background(), p, op.FileID, "")
	if e != nil {
		t.Fatal(e)
	}
	content, e := io.ReadAll(body)
	body.Close()
	if e != nil || string(content) != "drill payload" {
		t.Fatalf("restored drill object unreadable: %q %v", content, e)
	}
}
