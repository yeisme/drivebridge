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
)

func TestManagedVaultInstalledGuardProtectsOrdinaryContentAcrossEntries(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	f.svc.Enabled = true
	defer f.svc.Close()
	preview := f.preview()
	if _, err := f.install(preview, "confirm"); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"active", "hidden"} {
		if state == "hidden" {
			if _, err := f.svc.SetGuardVisibility(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 1, Hidden: true, IdempotencyKey: "hide"}); err != nil {
				t.Fatal(err)
			}
		}
		// 这里故意写普通正文，而非 .pinax；仅元数据过滤无法通过本反例。
		if err := f.svc.Mkdir(context.Background(), f.space, "notes/new"); err == nil || err.Error() != "owner_protected" {
			t.Fatalf("%s service content write: %v", state, err)
		}
		if _, err := f.svc.Call(context.Background(), "mkdir", map[string]any{"space_id": f.space, "path": "notes/new"}); err == nil || err.Error() != "owner_protected" {
			t.Fatalf("%s MCP content write: %v", state, err)
		}
		mux := http.NewServeMux()
		f.svc.localFileRoutes(mux)
		request := httptest.NewRequest(http.MethodPost, "/files/action", strings.NewReader(`{"action":"mkdir","space_id":"`+f.space+`","path":"notes/new"}`))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "owner_protected") {
			t.Fatalf("%s HTTP content write: %d %s", state, response.Code, response.Body.String())
		}
		if err := f.svc.rejectOperationMutation(Operation{Kind: "download", SpaceID: f.space, LocalPath: "notes/new.md"}); err == nil || err.Error() != "owner_protected" {
			t.Fatalf("%s persisted worker: %v", state, err)
		}
	}
	if _, err := f.svc.ReleaseGuard(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 2, Confirmed: true, IdempotencyKey: "release"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.rejectOwnerProtected(f.space, "notes/ordinary.md"); err != nil {
		t.Fatalf("released content retained the binding write gate: %v", err)
	}
	if err := f.svc.rejectOwnerProtected(f.space, ".pinax/config.yaml"); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("release removed the adoption metadata shield: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "notes")); !os.IsNotExist(err) {
		t.Fatal("denied content entry created files")
	}
}

func TestManagedVaultInstalledGuardRejectsAliasesAndAncestors(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	if _, err := f.install(f.preview(), "confirm"); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(f.root)
	if err := f.svc.DB.Create(&Space{ID: "parent", Remote: "remote", LocalRoot: parent}).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f.root, filepath.Join(parent, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", "vault", "vault/notes/a.md", "alias/notes/a.md"} {
		if err := f.svc.rejectLocalOwnerProtected("parent", target); err == nil || err.Error() != "owner_protected" {
			t.Fatalf("ancestor/alias %q: %v", target, err)
		}
	}
	if err := f.svc.rejectLocalOwnerProtected("parent", "sibling/a.md"); err != nil {
		t.Fatalf("unrelated sibling was blocked: %v", err)
	}
	if err := f.svc.DB.Migrator().DropTable(&ManagedGuard{}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.rejectLocalOwnerProtected("parent", "sibling/a.md"); err == nil || err.Error() != "guard_unavailable" {
		t.Fatalf("unreadable guard registry failed open: %v", err)
	}
}

func TestGuardPreviewRequiresExplicitOwnerAdoption(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	if err := f.svc.DB.Where("id = ?", "guard-owner").Delete(&AdoptRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	// A fenced file-service state is insufficient on its own: it has not
	// declared this root as owner-managed or installed the metadata shield.
	if _, err := f.svc.PreviewGuard(GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: f.space, Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}); err == nil || err.Error() != "managed_scope_unverifiable" {
		t.Fatalf("unadopted file root became a ready binding: %v", err)
	}
}

func TestGuardCompletedOperationsDoNotBlockBinding(t *testing.T) {
	f := newGuardFixture(t)
	f.fence()
	defer f.svc.Close()
	if err := f.svc.DB.Create(&Operation{ID: "completed", Key: "completed", SpaceID: f.space, Kind: "upload", Status: "completed"}).Error; err != nil {
		t.Fatal(err)
	}
	preview := f.preview()
	if preview.Decision != "ready" || preview.PendingMutations != 0 {
		t.Fatalf("terminal operation treated as in-flight: %#v", preview)
	}
	if _, err := f.install(preview, "confirm"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReleaseGuard(GuardRequest{BindingID: preview.BindingID, ExpectedRevision: 1, Confirmed: true, IdempotencyKey: "release"}); err != nil {
		t.Fatalf("terminal operation blocked release: %v", err)
	}
}
