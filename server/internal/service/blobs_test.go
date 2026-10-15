package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"
	"testing"
)

type missingAwareStore struct {
	*memoryStore
}

func (m missingAwareStore) Get(ctx context.Context, k string) (io.ReadCloser, error) {
	if e := m.memoryStore.Stat(ctx, k); e != nil {
		return nil, code("storage_read_failed")
	}
	return m.memoryStore.Get(ctx, k)
}

func blobRig(t *testing.T) (*Service, *memoryStore, Principal) {
	t.Helper()
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true, MaxBytes: 1 << 30})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	mem := &memoryStore{data: map[string][]byte{}}
	s.storage = func(S3Config, bool) (Storage, error) { return mem, nil }
	if e = s.SetPlatform(context.Background(), S3Config{}); e != nil {
		t.Fatal(e)
	}
	user, e := s.ensureUser("fixture", "blob-user", "Blob")
	if e != nil {
		t.Fatal(e)
	}
	sp := Space{ID: ID(), Owner: user.ID, Name: "main", ConnectionID: "platform", MaxBytes: 1 << 30}
	if e = s.DB.Create(&sp).Error; e != nil {
		t.Fatal(e)
	}
	return s, mem, Principal{UserID: user.ID}
}

func uploadBytes(t *testing.T, s *Service, p Principal, spaceID, path string, body []byte) Operation {
	t.Helper()
	sum := sha256.Sum256(body)
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: spaceID, Path: path, Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}, ID())
	if e != nil {
		t.Fatal(e)
	}
	if op.Status != "completed" {
		if _, e = s.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
			t.Fatal(e)
		}
		if op, e = s.Complete(context.Background(), p, op.ID); e != nil {
			t.Fatal(e)
		}
	}
	return op
}

func TestBlobInstantUploadSharesOneObject(t *testing.T) {
	s, mem, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("shared blob payload")
	first := uploadBytes(t, s, p, sp.ID, "a.bin", body)
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])

	second, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: "b.bin", Size: int64(len(body)), SHA256: sha}, ID())
	if e != nil {
		t.Fatal(e)
	}
	if second.Status != "completed" || second.Transferred != 0 || !second.BlobReused {
		t.Fatalf("instant upload must complete with zero bytes: %+v", second)
	}
	var blob Blob
	if e := s.DB.First(&blob, "sha256 = ?", sha).Error; e != nil || blob.Refs != 2 {
		t.Fatalf("expected one blob with refs=2: %v %+v", e, blob)
	}
	if len(mem.data) != 1 {
		t.Fatalf("expected exactly one object, got %d", len(mem.data))
	}
	if _, ok := mem.data[s.Instance+"/blobs/"+Hash(p.UserID)+"/"+sha]; !ok {
		t.Fatalf("plaintext blob key must be owner-scoped: %v", mem.data)
	}
	// Both versions read the same bytes through their original URIs.
	for _, op := range []Operation{first, second} {
		_, _, r, e := s.Download(context.Background(), p, op.FileID, op.VersionID)
		if e != nil {
			t.Fatal(e)
		}
		got, _ := io.ReadAll(r)
		r.Close()
		if !bytes.Equal(got, body) {
			t.Fatalf("version %s read back different bytes", op.VersionID)
		}
	}
	// Quota: logical usage grew, no reservation leaked.
	s.DB.First(&sp, "id = ?", sp.ID)
	if sp.Used != 2*int64(len(body)) || sp.Reserved != 0 {
		t.Fatalf("quota state after instant upload: used=%d reserved=%d", sp.Used, sp.Reserved)
	}
	// An idempotent replay of the hit returns the same operation.
	replay, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: "b.bin", Size: int64(len(body)), SHA256: sha}, opKeyOf(t, s, second.ID))
	if e != nil || replay.ID != second.ID {
		t.Fatalf("replay: %v %+v", e, replay)
	}
	// GET blob metadata is authorized for the owner.
	info, e := s.BlobInfo(p, sha)
	if e != nil || info["ref_count"] != int64(2) {
		t.Fatalf("blob info: %v %+v", e, info)
	}
}

func opKeyOf(t *testing.T, s *Service, id string) string {
	t.Helper()
	var op Operation
	if e := s.DB.First(&op, "id = ?", id).Error; e != nil {
		t.Fatal(e)
	}
	return op.Key
}

func TestBlobInstantUploadScopeBoundaries(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("scope boundary payload")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	uploadBytes(t, s, p, sp.ID, "a.bin", body)

	// Another data owner with the same digest does not hit.
	other, e := s.ensureUser("fixture", "other-user", "Other")
	if e != nil {
		t.Fatal(e)
	}
	osp := Space{ID: ID(), Owner: other.ID, Name: "other", ConnectionID: "platform", MaxBytes: 1 << 30}
	if e = s.DB.Create(&osp).Error; e != nil {
		t.Fatal(e)
	}
	op, e := s.CreateUpload(Principal{UserID: other.ID}, UploadRequest{SpaceID: osp.ID, Path: "x.bin", Size: int64(len(body)), SHA256: sha}, ID())
	if e != nil || op.Status == "completed" {
		t.Fatalf("cross-owner digest must not hit: %v %+v", e, op)
	}
	// An encrypted space never reuses a plaintext-scope blob.
	esp := Space{ID: ID(), Owner: p.UserID, Name: "enc", ConnectionID: "platform", MaxBytes: 1 << 30, Encrypted: true}
	if e = s.DB.Create(&esp).Error; e != nil {
		t.Fatal(e)
	}
	op, e = s.CreateUpload(p, UploadRequest{SpaceID: esp.ID, Path: "x.bin", Size: int64(len(body)), SHA256: sha}, ID())
	if e != nil || op.Status == "completed" || len(op.Nonce) != 12 {
		t.Fatalf("plaintext/encrypted boundary must miss: %v %+v", e, op)
	}
	// An encrypted blob in one space is invisible to another encrypted space.
	if e = s.DB.Create(&Blob{ID: blobID(p.UserID, esp.ID, sha), SHA256: sha, Owner: p.UserID, SpaceID: esp.ID, ConnectionID: "platform", ObjectKey: s.Instance + "/blobs/" + esp.ID + "/" + sha, Size: int64(len(body)), Refs: 1, Encrypted: true}).Error; e != nil {
		t.Fatal(e)
	}
	esp2 := Space{ID: ID(), Owner: p.UserID, Name: "enc2", ConnectionID: "platform", MaxBytes: 1 << 30, Encrypted: true}
	if e = s.DB.Create(&esp2).Error; e != nil {
		t.Fatal(e)
	}
	op, e = s.CreateUpload(p, UploadRequest{SpaceID: esp2.ID, Path: "x.bin", Size: int64(len(body)), SHA256: sha}, ID())
	if e != nil || op.Status == "completed" {
		t.Fatalf("cross-space encrypted digest must not hit: %v %+v", e, op)
	}
	// Unauthorized principal gets not_found, never a ref count.
	if _, e = s.BlobInfo(Principal{UserID: other.ID}, sha); e == nil || e.Error() != "not_found" {
		t.Fatalf("unauthorized blob stat must be not_found: %v", e)
	}
}

func TestBlobDriftIsNotAHit(t *testing.T) {
	s, _, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("drift payload")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	// Metadata claims a blob whose object is missing from storage.
	if e := s.DB.Create(&Blob{ID: blobID(p.UserID, "", sha), SHA256: sha, Owner: p.UserID, ConnectionID: "platform", ObjectKey: s.Instance + "/blobs/" + sha, Size: int64(len(body)), Refs: 1}).Error; e != nil {
		t.Fatal(e)
	}
	if _, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: "d.bin", Size: int64(len(body)), SHA256: sha}, ID()); e == nil || e.Error() != "blob_unavailable" {
		t.Fatalf("drift must report blob_unavailable, got %v", e)
	}
	// A version pointing at a missing blob object reports blob_unavailable on
	// download instead of publishing empty bytes.
	op := uploadBytes(t, s, p, sp.ID, "ok.bin", []byte("drift download payload"))
	s.storage = func(S3Config, bool) (Storage, error) {
		return missingAwareStore{&memoryStore{data: map[string][]byte{}}}, nil
	}
	if _, _, _, dlErr := s.Download(context.Background(), p, op.FileID, op.VersionID); dlErr == nil || dlErr.Error() != "blob_unavailable" {
		t.Fatalf("download drift must report blob_unavailable, got %v", dlErr)
	}
}

func TestBlobConcurrentCompleteDedupes(t *testing.T) {
	s, mem, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("concurrent dedupe payload")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	mk := func(path string) Operation {
		op, e := s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: path, Size: int64(len(body)), SHA256: sha}, ID())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
			t.Fatal(e)
		}
		return op
	}
	one, two := mk("one.bin"), mk("two.bin")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, op := range []Operation{one, two} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, e := s.Complete(context.Background(), p, id)
			errs <- e
		}(op.ID)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var blob Blob
	if e := s.DB.First(&blob, "sha256 = ?", sha).Error; e != nil || blob.Refs != 2 {
		t.Fatalf("concurrent completes must leave refs=2: %v %+v", e, blob)
	}
	if len(mem.data) != 1 {
		t.Fatalf("concurrent completes must leave one object, got %d", len(mem.data))
	}
}

func TestBlobPurgeReleasesReferenceAndGCDryRun(t *testing.T) {
	s, mem, p := blobRig(t)
	var sp Space
	s.DB.First(&sp, "owner = ?", p.UserID)
	body := []byte("purge payload")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	first := uploadBytes(t, s, p, sp.ID, "a.bin", body)
	second := uploadBytes(t, s, p, sp.ID, "b.bin", body)
	if !second.BlobReused {
		t.Fatal("second upload must be an instant hit")
	}
	if e := s.Trash(p, first.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Purge(context.Background(), p, first.FileID); e != nil {
		t.Fatal(e)
	}
	var blob Blob
	if e := s.DB.First(&blob, "sha256 = ?", sha).Error; e != nil || blob.Refs != 1 {
		t.Fatalf("purge must release one reference: %v %+v", e, blob)
	}
	if len(mem.data) != 1 {
		t.Fatal("purge must not delete the shared object")
	}
	// The surviving version still downloads.
	_, _, r, e := s.Download(context.Background(), p, second.FileID, second.VersionID)
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
	// Purge the second file: refs reach zero, the object stays, gc dry-run
	// reports it and deletes nothing without confirmation.
	if e := s.Trash(p, second.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e := s.Purge(context.Background(), p, second.FileID); e != nil {
		t.Fatal(e)
	}
	if len(mem.data) != 1 {
		t.Fatal("zero refs must not delete the object by itself")
	}
	report, e := s.BlobGC(context.Background(), true)
	if e != nil || report["reclaimable_blobs"] != 1 || report["removed"] != 0 {
		t.Fatalf("gc dry run: %v %+v", e, report)
	}
	if len(mem.data) != 1 {
		t.Fatal("dry run deleted an object")
	}
	report, e = s.BlobGC(context.Background(), false)
	if e != nil || report["removed"] != 1 {
		t.Fatalf("gc confirmed run: %v %+v", e, report)
	}
	if len(mem.data) != 0 {
		t.Fatal("confirmed gc must delete the unreferenced object")
	}
}

func TestBlobPurgeDoesNotCrossOwnerDigestBoundary(t *testing.T) {
	s, mem, alice := blobRig(t)
	body := []byte("same bytes, separate owners")
	var aliceSpace Space
	if e := s.DB.First(&aliceSpace, "owner = ?", alice.UserID).Error; e != nil {
		t.Fatal(e)
	}
	bobUser, e := s.ensureUser("fixture", "blob-bob", "Blob Bob")
	if e != nil {
		t.Fatal(e)
	}
	bobSpace := Space{ID: ID(), Owner: bobUser.ID, Name: "bob", ConnectionID: "platform", MaxBytes: 1 << 30}
	if e = s.DB.Create(&bobSpace).Error; e != nil {
		t.Fatal(e)
	}
	aliceOp := uploadBytes(t, s, alice, aliceSpace.ID, "same.txt", body)
	bob := Principal{UserID: bobUser.ID}
	bobOp := uploadBytes(t, s, bob, bobSpace.ID, "same.txt", body)
	if len(mem.data) != 2 {
		t.Fatalf("same digest must use owner-scoped objects, got %d", len(mem.data))
	}
	if e = s.Trash(alice, aliceOp.FileID, false); e != nil {
		t.Fatal(e)
	}
	if e = s.Purge(context.Background(), alice, aliceOp.FileID); e != nil {
		t.Fatal(e)
	}
	_, _, reader, e := s.Download(context.Background(), bob, bobOp.FileID, bobOp.VersionID)
	if e != nil {
		t.Fatal(e)
	}
	got, e := io.ReadAll(reader)
	reader.Close()
	if e != nil || !bytes.Equal(got, body) {
		t.Fatalf("other owner content lost after purge: %v %q", e, got)
	}
}
