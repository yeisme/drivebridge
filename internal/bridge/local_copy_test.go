package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDuplicateFileRenamesInSameDirectory(t *testing.T) {
	s, p, local, _ := setup(t)
	source := fixture(t, local)
	op, err := s.Upload(context.Background(), "creative", "", source, "reference.png", "dup-src")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	copied, err := s.DuplicateFile(context.Background(), op.FileRef, "", "reference.png")
	if err != nil || copied.Ref == "" || copied.Ref == op.FileRef {
		t.Fatalf("duplicate %+v %v uploads=%d", copied, err, p.uploads)
	}
	if p.uploads < 2 && p.opens < 1 {
		t.Fatalf("expected byte copy fallback, uploads=%d opens=%d", p.uploads, p.opens)
	}
}

func TestDuplicateDirectoryCopiesTree(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := s.Mkdir(ctx, "nas", "项目/资料"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(local, "note.txt"), []byte("hello"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(ctx, "nas", "项目/资料", filepath.Join(local, "note.txt"), "note.txt", "dir-src")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, op.ID)
	out, err := s.DuplicateDirectory(ctx, "nas", "项目/资料", "备份/资料")
	if err != nil || out["copied"].(int) != 1 {
		t.Fatalf("copy dir %+v %v", out, err)
	}
	files, err := s.List(ctx, "nas", "备份/资料")
	if err != nil || len(files) != 1 {
		t.Fatalf("copied list %v %+v", err, files)
	}
	src, err := s.List(ctx, "nas", "项目/资料")
	if err != nil || len(src) != 1 {
		t.Fatalf("source kept %v %+v", err, src)
	}
}

func TestRelocateDirectoryRejectsIntoSelf(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := s.Mkdir(ctx, "nas", "项目/资料"); e != nil {
		t.Fatal(e)
	}
	if _, err := s.RelocateDirectory(ctx, "nas", "项目", "项目/资料/nested"); err == nil || err.Error() != "invalid_directory" {
		t.Fatalf("into self %v", err)
	}
}

func TestTrashDirectoryRemovesFiles(t *testing.T) {
	s, local, _ := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := s.Mkdir(ctx, "nas", "项目/资料"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(local, "gone.txt"), []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	op, e := s.Upload(ctx, "nas", "项目/资料", filepath.Join(local, "gone.txt"), "gone.txt", "trash-src")
	if e != nil {
		t.Fatal(e)
	}
	waitCompleted(t, s, op.ID)
	out, err := s.TrashDirectory(ctx, "nas", "项目/资料")
	if err != nil || out["trashed"].(int) != 1 {
		t.Fatalf("trash %+v %v", out, err)
	}
	files, err := s.List(ctx, "nas", "项目/资料")
	if err != nil && err.Error() != "remote_not_found" {
		t.Fatalf("live list %v %+v", err, files)
	}
	if err == nil && len(files) != 0 {
		t.Fatalf("folder still listed %+v", files)
	}
}

func TestTrashDirectoryRejectsUnsupportedFiles(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if e := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 32}); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e := s.Mkdir(ctx, "nas", "项目/资料"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(remoteRoot, "项目", "资料", "empty.txt"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if _, err := s.TrashDirectory(ctx, "nas", "项目/资料"); err == nil || err.Error() != "directory_contains_unsupported_files" {
		t.Fatalf("empty file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(remoteRoot, "项目", "资料", "empty.txt")); err != nil {
		t.Fatalf("empty file must remain: %v", err)
	}
	if e := os.Remove(filepath.Join(remoteRoot, "项目", "资料", "empty.txt")); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(remoteRoot, "项目", "资料", "big.bin"), make([]byte, 64), 0600); e != nil {
		t.Fatal(e)
	}
	if _, err := s.TrashDirectory(ctx, "nas", "项目/资料"); err == nil || err.Error() != "directory_contains_unsupported_files" {
		t.Fatalf("oversize file: %v", err)
	}
}

func TestRelocateDirectoryRollsBackOnProtectedFile(t *testing.T) {
	s, _, _ := genericFixture(t)
	vault := t.TempDir()
	if err := os.Mkdir(filepath.Join(vault, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "a.md"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "docs", "keep.token"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAliasRemote("db-local-docs", vault); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "docs-vault",
		Root: vault, Remote: "db-local-docs", LocalRoot: vault,
		IdempotencyKey: "k-docs", MaxBytes: 1 << 20,
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.RelocateDirectory(ctx, "docs-vault", "docs", "docs2"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("protected relocate: %v", err)
	}
	src, err := s.List(ctx, "docs-vault", "docs")
	if err != nil || len(src) != 2 {
		t.Fatalf("source kept %v %+v", err, src)
	}
	dest, err := s.List(ctx, "docs-vault", "docs2")
	if err == nil {
		for _, item := range dest {
			if !item.Directory {
				t.Fatalf("dest split leftover %+v", dest)
			}
		}
	}
}
