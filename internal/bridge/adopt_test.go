package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerProtectedRel(t *testing.T) {
	protected := []string{
		".pinax", ".pinax/config.yaml", ".pinax/nested/x",
		"notes/.pinax/config.yaml",
		".scaena", ".scaena/config.yaml", ".scaena/objects", ".scaena/objects/meta",
		"state.sqlite", "vault.sqlite-wal", "app.db", "app.db-shm",
		"token", "user.token", "dir/token",
	}
	allowed := []string{
		"", ".", "notes", "notes/a.md", "output/img.png",
		".scaena/objects/sha256", ".scaena/objects/sha256/ab",
		".scaena/objects/sha256/ab/abc",
		"readme.md", "config.yaml",
	}
	for _, p := range protected {
		if !OwnerProtectedRel(p) {
			t.Fatalf("%q should be owner_protected", p)
		}
	}
	for _, p := range allowed {
		if OwnerProtectedRel(p) {
			t.Fatalf("%q should be allowed", p)
		}
	}
}

func TestAdoptLocalZeroCopyAndFence(t *testing.T) {
	s, local, _ := genericFixture(t)
	vault := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(filepath.Join(vault, ".pinax"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "note.md"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, ".pinax", "config.yaml"), []byte("cfg"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAliasRemote("db-local-vault", vault); err != nil {
		t.Fatal(err)
	}
	rec, reused, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "pinax-vault",
		Root: vault, Remote: "db-local-vault", LocalRoot: vault,
		IdempotencyKey: "k-local", MaxBytes: 1 << 20,
	})
	if err != nil || reused || rec.Copied || rec.Location != filepath.Clean(vault) {
		t.Fatalf("adopt: rec=%+v reused=%v err=%v", rec, reused, err)
	}
	again, reused, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "pinax-vault",
		Root: vault, Remote: "db-local-vault", LocalRoot: vault,
		IdempotencyKey: "k-local", MaxBytes: 1 << 20,
	})
	if err != nil || !reused || again.ID != rec.ID {
		t.Fatalf("reuse: rec=%+v reused=%v err=%v", again, reused, err)
	}
	files, err := s.List(context.Background(), "pinax-vault", "")
	if err != nil || len(files) != 1 || files[0].Name != "note.md" {
		t.Fatalf("list root: %v %+v", err, files)
	}
	pinax, err := s.List(context.Background(), "pinax-vault", ".pinax")
	if err != nil || len(pinax) != 1 || pinax[0].Name != "config.yaml" {
		t.Fatalf("list .pinax should stay allowed: %v %+v", err, pinax)
	}
	if err = s.Mkdir(context.Background(), "pinax-vault", ".pinax"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("mkdir .pinax: %v", err)
	}
	if err = s.Mkdir(context.Background(), "pinax-vault", ".pinax/nested"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("mkdir .pinax/nested: %v", err)
	}
	if err = s.Mkdir(context.Background(), "pinax-vault", ".scaena"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("mkdir .scaena: %v", err)
	}
	if err = s.Mkdir(context.Background(), "pinax-vault", ".scaena/objects/sha256/ab"); err != nil {
		t.Fatalf("cas mkdir should be allowed: %v", err)
	}
	if err = s.Mkdir(context.Background(), "pinax-vault", "notes"); err != nil {
		t.Fatalf("user mkdir: %v", err)
	}
	src := filepath.Join(vault, "upload.bin")
	if err = os.WriteFile(src, []byte("payload-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upload(context.Background(), "pinax-vault", ".pinax", src, "upload.bin", "up-pinax"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("upload .pinax: %v", err)
	}
	if _, err = s.Upload(context.Background(), "pinax-vault", "", src, "state.sqlite", "up-sqlite"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("upload sqlite: %v", err)
	}
	other := t.TempDir()
	if err = EnsureAliasRemote("db-local-other", other); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "other-vault",
		Root: other, Remote: "db-local-other", LocalRoot: other,
		IdempotencyKey: "k-local", MaxBytes: 1 << 20,
	}); err == nil || err.Error() != "adopt_mismatch" {
		t.Fatalf("mismatch: %v", err)
	}
	if _, err = os.Stat(filepath.Join(local, "note.md")); err == nil {
		t.Fatal("adopt must not copy vault bytes into local-root")
	}
}

func TestAdoptS3SamePrefixNoCopy(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	prefix := filepath.Join(remoteRoot, "bucket", "prefix")
	if err := os.MkdirAll(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(prefix, "obj.txt")
	if err := os.WriteFile(obj, []byte("blob"), 0600); err != nil {
		t.Fatal(err)
	}
	rec, reused, err := s.AdoptStorage(AdoptRequest{
		Consumer: "eikona", Kind: "s3", Space: "eikona-exports",
		Remote: "nas", RemotePath: "bucket/prefix", LocalRoot: local,
		IdempotencyKey: "k-s3", MaxBytes: 1 << 20,
	})
	if err != nil || reused || rec.Copied || rec.Location != "bucket/prefix" {
		t.Fatalf("s3 adopt: %+v reused=%v err=%v", rec, reused, err)
	}
	files, err := s.List(context.Background(), "eikona-exports", "")
	if err != nil || len(files) != 1 || files[0].Name != "obj.txt" {
		t.Fatalf("s3 list: %v %+v", err, files)
	}
	if _, err = os.Stat(obj); err != nil {
		t.Fatal("s3 adopt must keep the original object")
	}
	matches, _ := filepath.Glob(filepath.Join(local, "*obj.txt"))
	if len(matches) != 0 {
		t.Fatalf("s3 adopt copied bytes into local-root: %v", matches)
	}
}

func TestAdoptFenceSkipsNonAdoptedSpace(t *testing.T) {
	s, local, _ := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if err := s.Mkdir(context.Background(), "nas", ".pinax"); err != nil {
		t.Fatalf("non-adopted mkdir .pinax should stay allowed: %v", err)
	}
}

func TestAdoptLocalRootMustBeSameOrParent(t *testing.T) {
	s, _, _ := genericFixture(t)
	vault := t.TempDir()
	sibling := t.TempDir()
	if err := EnsureAliasRemote("db-local-parent", vault); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "bad-root",
		Root: vault, Remote: "db-local-parent", LocalRoot: sibling, MaxBytes: 1 << 20,
	}); err == nil || err.Error() != "invalid_space" {
		t.Fatalf("sibling local-root: %v", err)
	}
	parent := filepath.Dir(vault)
	if _, _, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "parent-root",
		Root: vault, Remote: "db-local-parent", LocalRoot: parent, MaxBytes: 1 << 20,
	}); err != nil {
		t.Fatalf("parent local-root should be allowed: %v", err)
	}
}
