package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPinaxOverlayLinksContentAndKeepsControlPlane(t *testing.T) {
	content := t.TempDir()
	if err := os.Mkdir(filepath.Join(content, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "notes", "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(t.TempDir(), "vault")
	receipt, err := ApplyPinaxOverlay(vault, content, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Preset != "pinax-vault" || len(receipt.Links) != 1 || receipt.Links[0] != "notes" {
		t.Fatalf("receipt: %+v", receipt)
	}
	got, err := os.ReadFile(filepath.Join(vault, "notes", "a.md"))
	if err != nil || string(got) != "a" {
		t.Fatalf("read through overlay: %s %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(vault, ".pinax"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, ".pinax", "config.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(content, ".pinax", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("control plane leaked into content root")
	}
	RemovePinaxOverlay(receipt)
	if _, err := os.Lstat(filepath.Join(vault, "notes")); !os.IsNotExist(err) {
		t.Fatal("content link not removed")
	}
	if _, err := os.Stat(filepath.Join(vault, ".pinax", "config.yaml")); err != nil {
		t.Fatal("control plane removed by stop")
	}
	if _, err := os.Stat(filepath.Join(content, "notes", "a.md")); err != nil {
		t.Fatal("content bytes removed by stop")
	}
}

func TestApplyPinaxOverlayCreatesNotesWhenEmpty(t *testing.T) {
	content := t.TempDir()
	vault := filepath.Join(t.TempDir(), "vault")
	receipt, err := ApplyPinaxOverlay(vault, content, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Links) != 1 || receipt.Links[0] != "notes" {
		t.Fatalf("expected notes link, got %+v", receipt.Links)
	}
	info, err := os.Stat(filepath.Join(content, "notes"))
	if err != nil || !info.IsDir() {
		t.Fatalf("notes dir: %v", err)
	}
}

func TestApplyPinaxOverlayRejectsCapsaPrefix(t *testing.T) {
	content := t.TempDir()
	if err := os.WriteFile(filepath.Join(content, "protocol.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(content, "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyPinaxOverlay(filepath.Join(t.TempDir(), "vault"), content, "docs")
	if err == nil || err.Error() != "preset_prefix_is_capsa" {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyPinaxOverlayConflictOnExistingFile(t *testing.T) {
	content := t.TempDir()
	if err := os.Mkdir(filepath.Join(content, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(filepath.Join(vault, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyPinaxOverlay(vault, content, "docs")
	if err == nil || err.Error() != "mount_overlay_conflict" {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyPinaxOverlayColocatedLocal(t *testing.T) {
	root := t.TempDir()
	receipt, err := ApplyPinaxOverlay(root, root, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Adapter != "pinax-vault-colocated" {
		t.Fatalf("adapter=%s", receipt.Adapter)
	}
	if _, err := os.Stat(filepath.Join(root, "notes")); err != nil {
		t.Fatal(err)
	}
}
