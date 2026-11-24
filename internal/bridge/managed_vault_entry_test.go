package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The WebDAV adapter must refuse an owner-protected target before it creates any
// staging bytes. Rejecting only at the final commit would still leave a staging
// artifact inside the vault for every denied request.
func TestManagedVaultWebDAVPutAndMoveRejectBeforeStaging(t *testing.T) {
	s, local, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	osRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer osRoot.Close()
	handler := &WebDAVHandler{VFS: NewVFS(s, []string{"vault"}, true), Token: "fixture-token", Root: osRoot}
	put := func(target string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPut, target, strings.NewReader("content"))
		request.Header.Set("Authorization", "Bearer fixture-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := put("/vault/.pinax/note.md")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "owner_protected") {
		t.Fatalf("protected PUT: %d %s", response.Code, response.Body.String())
	}
	move := httptest.NewRequest("MOVE", "/vault/notes/a.md", nil)
	move.Header.Set("Authorization", "Bearer fixture-token")
	move.Header.Set("Destination", "/vault/.pinax/notes/a.md")
	moveResponse := httptest.NewRecorder()
	handler.ServeHTTP(moveResponse, move)
	if moveResponse.Code != http.StatusBadRequest || !strings.Contains(moveResponse.Body.String(), "owner_protected") {
		t.Fatalf("protected MOVE destination: %d %s", moveResponse.Code, moveResponse.Body.String())
	}
	for _, dir := range []string{root, local} {
		if _, err := os.Stat(filepath.Join(dir, ".drivebridge-staging")); !os.IsNotExist(err) {
			t.Fatal("denied WebDAV write created staging bytes")
		}
	}
}

// A state directory inside an adopted root makes the preview cache a vault
// write. It must be refused instead of silently writing owner state into the
// user's tree.
func TestManagedVaultPreviewCacheRefusesStateInsideVault(t *testing.T) {
	base := t.TempDir()
	vault := filepath.Join(base, "vault")
	if err := os.MkdirAll(vault, 0700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(vault, "state")
	s, err := New(state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.rejectStateRootAlias(); err != nil {
		t.Fatalf("an instance with no adoption must stay usable: %v", err)
	}
	if err := s.DB.Create(&AdoptRecord{ID: "adopt-1", Consumer: "pinax", Kind: "local", SpaceID: "space-1", Location: vault}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.rejectStateRootAlias(); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("state inside the vault was accepted: %v", err)
	}
	// The refusal is path-based, not instance-based: an outside state keeps
	// working for the same adoption.
	outside := filepath.Join(base, "state-outside")
	other, err := New(outside, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := other.DB.Create(&AdoptRecord{ID: "adopt-2", Consumer: "pinax", Kind: "local", SpaceID: "space-1", Location: vault}).Error; err != nil {
		t.Fatal(err)
	}
	if err := other.rejectStateRootAlias(); err != nil {
		t.Fatalf("state outside the vault must stay usable: %v", err)
	}
}

// The remote-surface adapter reads local bytes with its own scoped root, so it
// must refuse owner metadata as an upload source before any byte is read. The
// rule is the same path predicate the application service uses; this adapter
// deliberately has no Service binding.
func TestManagedVaultRemoteSurfaceRefusesOwnerMetadataSource(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{".pinax/index.json", "notes/state.sqlite", "notes/token"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	osRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer osRoot.Close()
	surface := &RemoteSurfaces{Root: osRoot}
	for _, rel := range []string{".pinax/index.json", "notes/state.sqlite", "/notes/token"} {
		if _, err := surface.UploadLocalFile(context.Background(), "space", rel, "", "key-"+strings.ReplaceAll(rel, "/", "-")); err == nil || err.Error() != "owner_protected" {
			t.Fatalf("owner metadata source %q: %v", rel, err)
		}
	}
	// An ordinary note is not blocked by this predicate; it proceeds to the
	// (absent) transport instead, which is a different failure.
	if _, err := surface.UploadLocalFile(context.Background(), "space", "notes/plain.md", "", "key-plain"); err != nil && err.Error() == "owner_protected" {
		t.Fatal("an ordinary note was classified as owner metadata")
	}
}
