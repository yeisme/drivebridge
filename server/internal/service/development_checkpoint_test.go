package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
	"net/http/httptest"
	"testing"
)

func TestDevelopmentCheckpointVerifiesAndRestoresAllVersions(t *testing.T) {
	f := rig(t)
	owner, _ := login(t, f, "owner")
	body := []byte("retained object")
	hash := sha256.Sum256(body)
	p := service.Principal{UserID: owner.user.ID}
	op, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: owner.space.ID, Path: "preserved.txt", Size: int64(len(body)), SHA256: hex.EncodeToString(hash[:])}, service.ID())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Service.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
		t.Fatal(e)
	}
	op, e = f.Service.Complete(context.Background(), p, op.ID)
	if e != nil {
		t.Fatal(e)
	}
	handler, e := fixtures.DiskStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	disk := httptest.NewServer(handler)
	defer disk.Close()
	cfg := service.S3Config{Endpoint: disk.URL, Region: "us-east-1", Bucket: "fixture", Prefix: "managed", AccessKey: "fixture-key", SecretKey: "fixture-secret"}
	if n, e := f.Service.CloneDevelopmentPlatform(context.Background(), cfg, false); e != nil || n != 1 {
		t.Fatalf("clone n=%d error=%v", n, e)
	}
	f.S3.Close()
	if n, e := f.Service.CloneDevelopmentPlatform(context.Background(), cfg, true); e != nil || n != 1 {
		t.Fatalf("restore n=%d error=%v", n, e)
	}
	_, _, r, e := f.Service.Download(context.Background(), p, op.FileID, op.VersionID)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	b, e := io.ReadAll(r)
	if e != nil || !bytes.Equal(b, body) {
		t.Fatal("content changed")
	}
}
