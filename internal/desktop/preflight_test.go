package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

func selected(t *testing.T, p *Preflight, root string) desktopcontract.NativeSelection {
	t.Helper()
	ticket, err := p.Select(root)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}
func existingRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func confirmation(view desktopcontract.BindingPreview) desktopcontract.ConfirmBinding {
	return desktopcontract.ConfirmBinding{PreviewID: view.PreviewID, PreviewDigest: view.PreviewDigest, ExpectedGuardRevision: view.ExpectedGuardRevision, Confirmed: true}
}
func TestDesktopSessionPreflightBlockedWithoutVaultWrites(t *testing.T) {
	root := existingRoot(t)
	note := filepath.Join(root, "private-note.md")
	if err := os.WriteFile(note, []byte("private fixture content"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(note)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPreflight()
	defer p.Close()
	ticket := selected(t, p, root)
	view, err := p.Preview(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if view.Validate() != nil || view.Decision != "blocked" || view.Reason != "managed_scope_unverifiable" || len(view.Scopes) != 1 || view.Scopes[0] != desktopcontract.ScopeMetadata {
		t.Fatal("unsafe preview")
	}
	if err := p.ConfirmBlocked(confirmation(view)); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatal("confirmation bypassed admission")
	}
	wire, err := json.Marshal([]any{ticket, view, p})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{root, "private-note", "private fixture content"} {
		if strings.Contains(string(wire), secret) {
			t.Fatal("private selection leaked")
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("preflight wrote vault")
	}
	after, err := os.Stat(note)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("preflight changed note")
	}
	if _, err = p.Preview(ticket); err == nil {
		t.Fatal("ticket replay accepted")
	}
}
func TestDesktopSessionSelectionIsolationExpiryAndCancel(t *testing.T) {
	root := existingRoot(t)
	p, q := NewPreflight(), NewPreflight()
	defer p.Close()
	defer q.Close()
	now := time.Now()
	p.now = func() time.Time { return now }
	ticket := selected(t, p, root)
	if _, err := q.Preview(ticket); err == nil {
		t.Fatal("cross session accepted")
	}
	now = now.Add(selectionLifetime)
	if _, err := p.Preview(ticket); err == nil {
		t.Fatal("expired ticket accepted")
	}
	view, err := p.Preview(selected(t, p, root))
	if err != nil {
		t.Fatal(err)
	}
	tampered := confirmation(view)
	tampered.PreviewDigest = "sha256:" + strings.Repeat("a", 64)
	if err := p.ConfirmBlocked(tampered); err == nil || err.Error() != "binding_stale" {
		t.Fatal("tampering not rejected")
	}
	view.Scopes[0] = desktopcontract.ScopeBody
	if p.previews[view.PreviewID].view.Scopes[0] != desktopcontract.ScopeMetadata {
		t.Fatal("caller mutated stored scopes")
	}
	p.Cancel(view.PreviewID)
	p.Cancel(view.PreviewID)
	if err := p.ConfirmBlocked(confirmation(view)); err == nil || err.Error() != "binding_preview_expired" {
		t.Fatal("cancelled preview retained")
	}
	view, err = p.Preview(selected(t, p, root))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(selectionLifetime)
	if err := p.ConfirmBlocked(confirmation(view)); err == nil || err.Error() != "binding_preview_expired" {
		t.Fatal("expired preview accepted")
	}
	p.Close()
	if _, err := p.Select(root); err == nil {
		t.Fatal("closed session accepted selection")
	}
}
func TestDesktopSessionSelectionReplacementAndSymlink(t *testing.T) {
	base := existingRoot(t)
	root := filepath.Join(base, "vault")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	p := NewPreflight()
	defer p.Close()
	ticket := selected(t, p, root)
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Preview(ticket); err == nil || err.Error() != "native_selection_changed" {
		t.Fatal("replacement accepted")
	}
	view, err := p.Preview(selected(t, p, root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-second"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := p.ConfirmBlocked(confirmation(view)); err == nil || err.Error() != "native_selection_changed" {
		t.Fatal("confirmation ignored replaced directory")
	}
	for _, bad := range []string{strings.Repeat("x", 32769), "relative", root + string(os.PathSeparator) + "..", root + "\x00", filepath.Join(root, "missing")} {
		if _, err := p.Select(bad); err == nil {
			t.Fatal("unsafe path accepted")
		}
	}
	link := filepath.Join(base, "alias")
	if err := os.Symlink(root, link); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if _, err := p.Select(link); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := os.Mkdir(filepath.Join(root, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Select(filepath.Join(link, "child")); err == nil {
		t.Fatal("symlink ancestor accepted")
	}
}
func TestDesktopSessionSelectionBoundAndSingleConsumption(t *testing.T) {
	p := NewPreflight()
	defer p.Close()
	root := existingRoot(t)
	ticket := selected(t, p, root)
	var wg sync.WaitGroup
	results := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := p.Preview(ticket); results <- err == nil }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("ticket consumed more than once")
	}
	for i := 1; i < maxSelections; i++ {
		selected(t, p, root)
	}
	if _, err := p.Select(root); err == nil || err.Error() != "selection_capacity_exceeded" {
		t.Fatal("unbounded selections")
	}
	p.now = func() time.Time { return time.Now().Add(2 * selectionLifetime) }
	selected(t, p, root)
	p.Close()
	if len(p.selections) != 0 || len(p.previews) != 0 {
		t.Fatal("closed session retains private state")
	}
}
