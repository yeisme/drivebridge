package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// D08 acceptance: one bound protection domain must be refused identically by the
// application service, the loopback HTTP surface and the MCP dispatcher. A UI or
// adapter-level check is not evidence; every entry has to reach the same owner
// adjudication in `internal/bridge`.
func TestManagedVaultIntegrationCrossEntryRefusal(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(source, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	refused := func(name string, err error) {
		t.Helper()
		if err == nil || err.Error() != "owner_protected" {
			t.Fatalf("%s must be refused by the owner adjudication: %v", name, err)
		}
	}

	// Go application service.
	refused("service.Mkdir", s.Mkdir(ctx, "vault", ".pinax"))
	_, uploadErr := s.Upload(ctx, "vault", ".pinax", source, "note.md", "key-service-upload")
	refused("service.Upload", uploadErr)
	_, syncErr := s.Sync(ctx, "vault", SyncOptions{Resync: true, Workdir: t.TempDir()})
	refused("service.Sync", syncErr)
	_, actionErr := s.RunLocalFileAction(ctx, LocalFileAction{Action: "mkdir", Space: "vault", Path: ".pinax"})
	refused("action.mkdir", actionErr)
	_, actionUploadErr := s.RunLocalFileAction(ctx, LocalFileAction{Action: "upload_path", Space: "vault", Path: source, Name: "state.sqlite", Key: "key-action-upload"})
	refused("action.upload_path", actionUploadErr)

	// Loopback HTTP surface.
	mux := http.NewServeMux()
	s.localFileRoutes(mux)
	request := httptest.NewRequest(http.MethodPut, "/files/upload?space_id=vault&dir=.pinax&name=note.md", strings.NewReader("payload"))
	request.Header.Set("Idempotency-Key", "key-http-upload")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "owner_protected") {
		t.Fatalf("http upload: %d %s", response.Code, response.Body.String())
	}
	actionBody := strings.NewReader(`{"action":"mkdir","space_id":"vault","path":".pinax"}`)
	actionRequest := httptest.NewRequest(http.MethodPost, "/files/action", actionBody)
	actionResponse := httptest.NewRecorder()
	mux.ServeHTTP(actionResponse, actionRequest)
	if actionResponse.Code != http.StatusBadRequest || !strings.Contains(actionResponse.Body.String(), "owner_protected") {
		t.Fatalf("http action: %d %s", actionResponse.Code, actionResponse.Body.String())
	}

	// MCP dispatcher.
	_, mcpMkdirErr := s.Call(ctx, "mkdir", map[string]any{"space_id": "vault", "path": ".pinax"})
	refused("mcp.mkdir", mcpMkdirErr)
	_, mcpUploadErr := s.Call(ctx, "upload", map[string]any{"space_id": "vault", "local_path": source, "name": "state.sqlite", "idempotency_key": "key-mcp-upload"})
	refused("mcp.upload", mcpUploadErr)

	// A refused request must not leave an operation, a staged byte or a
	// partially created directory behind.
	var operations int64
	if err := s.DB.Model(&Operation{}).Count(&operations).Error; err != nil {
		t.Fatal(err)
	}
	if operations != 0 {
		t.Fatalf("denied entries created %d operations", operations)
	}
	for _, dir := range []string{root} {
		if _, err := os.Stat(filepath.Join(dir, ".drivebridge-staging")); !os.IsNotExist(err) {
			t.Fatal("denied entries created staging bytes")
		}
		if _, err := os.Stat(filepath.Join(dir, ".pinax")); !os.IsNotExist(err) {
			t.Fatal("denied entries created the owner directory")
		}
	}
}

// D08 acceptance: an existing mount/adoption must not migrate the state layout,
// and a legacy layout must keep guard admission closed instead of inventing the
// missing rejection condition.
func TestManagedVaultIntegrationAdoptionDoesNotMigrateState(t *testing.T) {
	f := newGuardFixture(t)
	f.open()
	defer f.svc.Close()
	layout, err := managedstate.Inspect(f.state, "cli")
	if err != nil || layout.Layout != "legacy" {
		t.Fatalf("fixture must start legacy: %#v %v", layout, err)
	}
	if err := f.svc.DB.Where("id = ?", "guard-owner").Delete(&AdoptRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DB.Create(&AdoptRecord{ID: "adopt-legacy", Consumer: "pinax", Kind: "local", SpaceID: f.space, Location: f.root}).Error; err != nil {
		t.Fatal(err)
	}
	after, err := managedstate.Inspect(f.state, "cli")
	if err != nil || after.Layout != "legacy" {
		t.Fatalf("adoption migrated the state layout: %#v %v", after, err)
	}
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("legacy state admitted a guard preview: %v", err)
	}
}

// D08 acceptance: a new writable connection that can reach the same physical root
// is a new connection without a bypass — binding is refused until it is inside
// the same protection domain.
func TestManagedVaultIntegrationNewConnectionNeedsSameDomain(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	if err := f.svc.DB.Create(&Connection{ID: "conn-alias", Label: "alias", BaseURL: "http://127.0.0.1:1", InstanceID: "instance-1", SpaceID: f.space, CredentialSource: "env"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err != nil {
		t.Fatalf("same-space connection must not block its own binding: %v", err)
	}
	if err := f.svc.DB.Create(&Connection{ID: "conn-other", Label: "other", BaseURL: "http://127.0.0.1:2", InstanceID: "instance-1", SpaceID: "space-2", CredentialSource: "env"}).Error; err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(f.root, "nested")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DB.Create(&Space{ID: "space-2", Remote: "remote2", LocalRoot: inner, MaxBytes: 1 << 20}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("a connection in another overlapping space bypassed the domain: %v", err)
	}
}
