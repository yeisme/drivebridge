package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func downloadedFile(t *testing.T, s *Service, local, name string) Operation {
	t.Helper()
	ref := uploadFixtureRef(t, s, local)
	op, err := s.Download(context.Background(), ref, name, "dl-"+name)
	if err != nil {
		t.Fatal(err)
	}
	return waitOp(t, s, op.ID)
}

func uploadFixtureRef(t *testing.T, s *Service, local string) string {
	t.Helper()
	fixture(t, local)
	op, err := s.Upload(context.Background(), "creative", "", "reference.png", "reference.png", "up-"+filepath.Base(local))
	if err != nil {
		t.Fatal(err)
	}
	done := waitOp(t, s, op.ID)
	if done.Status != "completed" {
		t.Fatalf("upload fixture: %s %s", done.Status, done.Error)
	}
	return done.FileRef
}

func TestDownloadCASDedupeSecondCopy(t *testing.T) {
	s, p, local, _ := setup(t)
	first := downloadedFile(t, s, local, "first.png")
	if first.Status != "completed" || first.CacheHit {
		t.Fatalf("first download: %s cache=%v", first.Status, first.CacheHit)
	}
	opensBefore := p.opens
	second := downloadedFile(t, s, local, "second.png")
	if second.Status != "completed" || !second.CacheHit || second.Transferred != 0 {
		t.Fatalf("second download: %s cache=%v transferred=%d", second.Status, second.CacheHit, second.Transferred)
	}
	if p.opens != opensBefore {
		t.Fatal("cache hit pulled remote bytes")
	}
	a, _ := os.ReadFile(filepath.Join(local, "first.png"))
	b, _ := os.ReadFile(filepath.Join(local, "second.png"))
	if string(a) != string(b) {
		t.Fatal("cache-published bytes diverge")
	}
	if _, err := os.Stat(filepath.Join(local, casRoot)); err != nil {
		t.Fatal("cache not populated on miss")
	}
}

func TestDownloadCASRejectsStaleVersion(t *testing.T) {
	s, p, local, _ := setup(t)
	first := downloadedFile(t, s, local, "first.png")
	p.mu.Lock()
	for id, f := range p.files {
		f.Version = "version2"
		p.files[id] = f
	}
	p.mu.Unlock()
	// The pinned version is re-verified at enqueue: a changed version refuses
	// before any cached byte can be published.
	if _, err := s.Download(context.Background(), first.FileRef, "stale.png", "dl-stale"); err == nil || err.Error() != "file_version_changed" {
		t.Fatalf("stale download: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "stale.png")); !os.IsNotExist(err) {
		t.Fatal("stale cache bytes published")
	}
}

func TestCASPathHiddenFromUserTree(t *testing.T) {
	s, _, local, _ := setup(t)
	downloadedFile(t, s, local, "first.png")
	files, err := s.List(context.Background(), "creative", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name == casRoot {
			t.Fatal("CAS directory leaked into the user tree")
		}
	}
	vfs := NewVFS(s, []string{"creative"}, false)
	nodes, err := vfs.Readdir(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Name == casRoot {
			t.Fatal("CAS directory leaked into the mount tree")
		}
	}
}
