package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func encryptedSpace(t *testing.T, h *human, name string) service.Space {
	t.Helper()
	var sp service.Space
	if status := h.call(t, "POST", "/spaces", map[string]any{"name": name, "connection_id": "platform", "encrypted": true}, &sp); status != 200 {
		t.Fatal(status)
	}
	if !sp.Encrypted {
		t.Fatal("space not flagged encrypted")
	}
	return sp
}

func storedObject(t *testing.T, f *fixtures.Runtime, objectKey string) []byte {
	t.Helper()
	for k, v := range f.Objects() {
		if strings.HasSuffix(k, "/"+objectKey) {
			return v
		}
	}
	t.Fatalf("object %s not stored", objectKey)
	return nil
}

func versionRow(t *testing.T, f *fixtures.Runtime, id string) service.Version {
	t.Helper()
	var v service.Version
	if f.Service.DB.First(&v, "id = ?", id).Error != nil {
		t.Fatalf("version %s missing", id)
	}
	return v
}

func TestEncryptedSpaceRoundtripAndCiphertextAtRest(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	c, _ := vaultAgent(t, h, vault.ID)
	// Key material never reaches API output.
	encoded, _ := json.Marshal(vault)
	if bytes.Contains(encoded, []byte("wrapped")) || bytes.Contains(encoded, []byte("dek")) {
		t.Fatal("space response leaked key material")
	}
	payload := bytes.Repeat([]byte("secret-payload-"), (9<<20)/15)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/ledger.bin"}, "vault-1", bytes.NewReader(payload), nil)
	if e != nil {
		t.Fatal(e)
	}
	if op.Status != "completed" {
		t.Fatal(op.Status)
	}
	v := versionRow(t, f, op.VersionID)
	if v.SHA256 != service.Hash(string(payload)) || v.Size != int64(len(payload)) {
		t.Fatal("plaintext digest or logical size not preserved in metadata")
	}
	body := storedObject(t, f, v.ObjectKey)
	if !bytes.HasPrefix(body, []byte("DBE1")) {
		t.Fatal("stored object missing envelope header")
	}
	if bytes.Contains(body, payload[:4096]) || int64(len(body)) <= v.Size {
		t.Fatal("object stored as plaintext")
	}
	var out bytes.Buffer
	if e = c.Download(context.Background(), op.FileID, "", &out); e != nil || !bytes.Equal(payload, out.Bytes()) {
		t.Fatal("decrypted download differs from plaintext")
	}
	// Direct link is narrowed to a short-lived server decryption stream.
	access, e := c.Acquire(context.Background(), op.FileID, op.VersionID)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(access.URL, h.base+"/api/v1/access/") || strings.Contains(access.URL, "X-Amz-") {
		t.Fatalf("encrypted space returned a storage direct link: %s", access.URL)
	}
	response, e := http.Get(access.URL)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Equal(data, payload) {
		t.Fatal("decryption stream returned wrong bytes")
	}
	if response.Header.Get("X-Content-SHA256") != v.SHA256 {
		t.Fatal("decryption stream missing digest header")
	}
	// Unknown or expired capabilities are refused.
	bad, _ := http.Get(h.base + "/api/v1/access/" + service.ID())
	bad.Body.Close()
	if bad.StatusCode != 401 {
		t.Fatalf("foreign capability accepted: %d", bad.StatusCode)
	}
	f.Service.DB.Model(&service.AccessGrant{}).Where("1 = 1").Update("expires", time.Now().UTC().Add(-time.Minute))
	expired, _ := http.Get(access.URL)
	expired.Body.Close()
	if expired.StatusCode != 401 {
		t.Fatalf("expired capability accepted: %d", expired.StatusCode)
	}
	// Reference handoff is rejected for encrypted spaces with a fixed code.
	_, e = f.Service.CallTool(context.Background(), service.Principal{UserID: h.user.ID}, "prepare_reference", map[string]any{"file_id": op.FileID, "version_id": op.VersionID})
	if e == nil || e.Error() != "encrypted_space_reference_rejected" {
		t.Fatalf("encrypted reference handoff not rejected: %v", e)
	}
}

func vaultAgent(t *testing.T, h *human, space string) (*sdk.Client, service.Principal) {
	t.Helper()
	var out struct {
		Agent service.Agent `json:"agent"`
		Token string        `json:"token"`
	}
	if status := h.call(t, "POST", "/agents", map[string]any{"name": "vault-agent", "space_id": space, "scopes": "read,write", "days": 1}, &out); status != 200 {
		t.Fatal(status)
	}
	client, e := sdk.New(h.base, out.Token)
	if e != nil {
		t.Fatal(e)
	}
	return client, service.Principal{UserID: h.user.ID}
}

func TestEncryptedUploadRestartRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	c, p := vaultAgent(t, h, vault.ID)
	payload := bytes.Repeat([]byte("recoverable "), (9<<20)/12)
	q := sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/restart.bin", Size: int64(len(payload)), SHA256: service.Hash(string(payload))}
	var op sdk.Operation
	if e := c.JSON(context.Background(), "POST", "/api/v1/uploads", "restart", q, &op); e != nil {
		t.Fatal(e)
	}
	for i := 0; int64(i)*8<<20 < int64(len(payload)); i++ {
		lo := int64(i) * 8 << 20
		hi := min(lo+8<<20, int64(len(payload)))
		if _, e := f.Service.PutPart(p, op.ID, i, bytes.NewReader(payload[lo:hi])); e != nil {
			t.Fatal(e)
		}
	}
	original := f.Service.Config
	f.Service.Close()
	reopened, e := service.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	f.Service = reopened
	done, e := reopened.Complete(context.Background(), p, op.ID)
	if e != nil || done.Status != "completed" {
		t.Fatalf("restart recovery failed: %v %s", e, done.Status)
	}
	// The fixture HTTP handler still points at the pre-restart service, so
	// the recovered object is read through the reopened service directly.
	_, _, body, e := reopened.Download(context.Background(), p, done.FileID, "")
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := io.ReadAll(body)
	body.Close()
	if e != nil || !bytes.Equal(payload, recovered) {
		t.Fatal("recovered object differs")
	}
}

func TestKEKRotationKeepsObjectsReadable(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	c, _ := vaultAgent(t, h, vault.ID)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/a.bin"}, "rotate-1", bytes.NewReader([]byte("before-rotation")), nil)
	if e != nil {
		t.Fatal(e)
	}
	v := versionRow(t, f, op.VersionID)
	before := storedObject(t, f, v.ObjectKey)
	var beforeSpace service.Space
	f.Service.DB.First(&beforeSpace, "id = ?", vault.ID)
	report, e := f.Service.RotateKEK()
	if e != nil {
		t.Fatal(e)
	}
	if report["key_version"] != 2 || report["spaces_rewrapped"] != 1 || report["objects_rewritten"] != 0 {
		t.Fatalf("rotation report unexpected: %v", report)
	}
	var afterSpace service.Space
	f.Service.DB.First(&afterSpace, "id = ?", vault.ID)
	if bytes.Equal(beforeSpace.WrappedDEK, afterSpace.WrappedDEK) {
		t.Fatal("space DEK not re-wrapped")
	}
	if after := storedObject(t, f, v.ObjectKey); !bytes.Equal(before, after) {
		t.Fatal("rotation rewrote object bytes")
	}
	var out bytes.Buffer
	if e = c.Download(context.Background(), op.FileID, "", &out); e != nil || out.String() != "before-rotation" {
		t.Fatal("old version unreadable after rotation")
	}
	next, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/b.bin"}, "rotate-2", bytes.NewReader([]byte("after-rotation")), nil)
	if e != nil || next.Status != "completed" {
		t.Fatalf("post-rotation upload failed: %v", e)
	}
	var fresh bytes.Buffer
	if e = c.Download(context.Background(), next.FileID, "", &fresh); e != nil || fresh.String() != "after-rotation" {
		t.Fatal("post-rotation version unreadable")
	}
}

func TestEncryptedInstanceKeyLossRefusesService(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	c, _ := vaultAgent(t, h, vault.ID)
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "vault/lost.bin"}, "lost", bytes.NewReader([]byte("irreplaceable")), nil); e != nil {
		t.Fatal(e)
	}
	restored := t.TempDir()
	os.Chmod(restored, 0700)
	for _, name := range []string{"metadata.sqlite", "instance.id"} {
		b, e := os.ReadFile(filepath.Join(f.Service.Config.State, name))
		if e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(restored, name), b, 0600)
	}
	if _, e := service.Open(service.Config{State: restored, Enabled: true, Development: true}); e == nil || e.Error() != "instance_key_missing_restore_required" {
		t.Fatalf("missing key silently regenerated: %v", e)
	}
}

func TestSpaceExportDecryptEscapeHatch(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	vault := encryptedSpace(t, h, "Vault")
	c, _ := vaultAgent(t, h, vault.ID)
	one, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "docs/one.txt"}, "export-1", bytes.NewReader([]byte("first file")), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Upload(context.Background(), sdk.UploadRequest{SpaceID: vault.ID, Path: "docs/two.txt"}, "export-2", bytes.NewReader([]byte("second file")), nil); e != nil {
		t.Fatal(e)
	}
	h.call(t, "POST", "/files/"+one.FileID+"/trash", map[string]any{}, nil)
	out := filepath.Join(t.TempDir(), "export")
	report, e := f.Service.ExportSpacePlaintext(context.Background(), vault.ID, out)
	if e != nil {
		t.Fatal(e)
	}
	if report["files_exported"] != 1 || report["files_skipped"] != 1 {
		t.Fatalf("export report unexpected: %v", report)
	}
	body, e := os.ReadFile(filepath.Join(out, "docs", "two.txt"))
	if e != nil || string(body) != "second file" {
		t.Fatalf("exported plaintext differs: %q %v", body, e)
	}
	// The escape hatch only applies to encrypted spaces.
	var plain service.Space
	f.Service.DB.First(&plain, "id = ?", h.space.ID)
	if _, e = f.Service.ExportSpacePlaintext(context.Background(), plain.ID, out); e == nil || e.Error() != "space_not_encrypted" {
		t.Fatalf("plaintext space export not refused: %v", e)
	}
}
