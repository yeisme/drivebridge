package bridge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"
)

func TestManagedVaultResumePreservesOriginalOperation(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "original-operation", Key: "original-key", SpaceID: "vault", Kind: "upload", Status: "interrupted", Target: ".pinax/original-note", LocalPath: "source.txt"}
	if err := s.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	got, err := s.Resume(op.ID)
	if err == nil || err.Error() != "owner_protected" || got.ID != op.ID {
		t.Fatalf("resume must refuse with original identity: %#v %v", got, err)
	}
	persisted, err := s.Get(op.ID)
	if err != nil || persisted.Status != "interrupted" || persisted.Key != op.Key {
		t.Fatalf("refusal changed original operation: %#v %v", persisted, err)
	}
	var count int64
	if err = s.DB.Model(&Operation{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("refusal created another operation")
	}
}

func TestManagedVaultWorkerRechecksQueuedUpload(t *testing.T) {
	s, _, root := genericFixture(t)
	if err := s.AddSpace(Space{ID: "vault", Remote: "nas", Backend: "generic", LocalRoot: root, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	// The original queue entry predates protection registration. Execute its
	// real worker path only after adoption, without enqueueing a replacement.
	op := Operation{ID: "queued-original", Key: "queued-key", SpaceID: "vault", Kind: "upload", Status: "queued", Target: ".pinax/original-note", LocalPath: "source.txt"}
	if err := s.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	s.run(context.Background(), op)
	got, err := s.Get(op.ID)
	if err != nil || got.Status != "failed" || got.Error != "owner_protected" || got.ID != op.ID {
		t.Fatalf("queued protection result: %#v %v", got, err)
	}
	if _, err = os.Stat(filepath.Join(root, ".pinax", "original-note")); !os.IsNotExist(err) {
		t.Fatal("queued upload reached protected provider")
	}
}

func TestManagedVaultHTTPRejectsBeforeStaging(t *testing.T) {
	s, local, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	for _, broken := range []bool{false, true} {
		if broken {
			if err := s.DB.Migrator().DropTable(&AdoptRecord{}); err != nil {
				t.Fatal(err)
			}
		}
		request := httptest.NewRequest(http.MethodPut, "/files/upload?space_id=vault&dir=.pinax&name=note.md", strings.NewReader("content"))
		request.Header.Set("Idempotency-Key", "original-http-key")
		response := httptest.NewRecorder()
		mux := http.NewServeMux()
		s.localFileRoutes(mux)
		mux.ServeHTTP(response, request)
		want := "owner_protected"
		if broken {
			want = "guard_unavailable"
		}
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), want) {
			t.Fatalf("pre-staging refusal: %d %s", response.Code, response.Body.String())
		}
		for _, dir := range []string{root, local} {
			if _, err := os.Stat(filepath.Join(dir, ".drivebridge-staging")); !os.IsNotExist(err) {
				t.Fatal("denied HTTP upload staged bytes")
			}
		}
	}
}

func TestManagedVaultSyncRefusesAdoptedWholeTree(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sync(context.Background(), "vault", SyncOptions{Resync: true, Workdir: t.TempDir()}); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("whole tree sync must not mutate owner metadata: %v", err)
	}
	var count int64
	if err := s.DB.Model(&Operation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("denied sync created operation")
	}
}

func TestManagedVaultLocalParentAndAliasAdmission(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".pinax"), 0700); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(root)
	if err := s.AddSpace(Space{ID: "parent", Remote: "nas", Backend: "generic", LocalRoot: parent, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if err := s.rejectLocalOwnerProtected("parent", ""); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("ancestor replacement: %v", err)
	}
	alias := filepath.Join(parent, "metadata-alias")
	if err := os.Symlink(filepath.Join(root, ".pinax"), alias); err != nil {
		t.Skip(err)
	}
	op := Operation{ID: "download-original", Key: "download-original-key", Kind: "download", SpaceID: "parent", LocalPath: "metadata-alias/new-note.md", Status: "queued"}
	if got, err := s.enqueue(op); err == nil || err.Error() != "owner_protected" || got.ID != op.ID {
		t.Fatalf("local symlink alias: %#v %v", got, err)
	}
	if err := s.rejectLocalOwnerProtected("vault", "notes/ordinary.md"); err != nil {
		t.Fatalf("historical adoption must not claim all content protection: %v", err)
	}
}

func TestManagedVaultSyncPlanPreservesOriginalIdentity(t *testing.T) {
	s, _, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	plan := SyncPlan{ID: "original-plan", SpaceID: "vault", Phase: planPhaseInterrupted, Revision: 1, OperationID: "original-sync-operation"}
	if err := s.DB.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplySyncPlan(context.Background(), plan.ID, ApplySyncPlanOptions{ExpectedRevision: plan.Revision})
	if err == nil || err.Error() != "owner_protected" || result.Plan.ID != plan.ID || result.Plan.OperationID != plan.OperationID {
		t.Fatalf("guard must retain plan and operation identity: %#v %v", result, err)
	}
}

// Interleave registry installation with a real rclone-backed read. All bytes
// and filesystem effects still go through the production provider.
type managedPublicationReader struct {
	Provider
	once       sync.Once
	beforeRead func()
}

func (p *managedPublicationReader) Open(ctx context.Context, space Space, file File) (io.ReadCloser, error) {
	body, err := p.Provider.Open(ctx, space, file)
	if err == nil {
		p.once.Do(p.beforeRead)
	}
	return body, err
}

func TestManagedVaultDownloadPublicationRechecksAfterRead(t *testing.T) {
	s, _, root := genericFixture(t)
	if err := s.AddSpace(Space{ID: "vault", Remote: "nas", Backend: "generic", LocalRoot: root, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".pinax"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.md"), []byte("real local content"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := s.List(context.Background(), "vault", "")
	if err != nil || len(files) != 1 {
		t.Fatalf("real source listing: %#v %v", files, err)
	}
	s.Generic = &managedPublicationReader{Provider: s.Generic, beforeRead: func() {
		if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
			t.Error(err)
		}
	}}
	op := Operation{ID: "download-before-adoption", Key: "download-key", SpaceID: "vault", Kind: "download", LocalPath: ".pinax/published.md", FileRef: files[0].Ref}
	if err = s.transfer(context.Background(), &op); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("publication after adoption: %v", err)
	}
	if _, err = os.Stat(filepath.Join(root, ".pinax", "published.md")); !os.IsNotExist(err) {
		t.Fatal("protected content was published")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".pinax"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("adoption must precede partial-file creation: %v %v", entries, err)
	}
}

func TestManagedVaultSyncRejectsAliasAndUnknownWriter(t *testing.T) {
	s, local, root := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: root, Remote: "nas", LocalRoot: root}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id     string
		remote string
		want   string
	}{
		{"alias", "nas", "owner_protected"},
		{"unknown", "unregistered-provider", "managed_scope_unverifiable"},
	} {
		if err := s.AddSpace(Space{ID: item.id, Remote: item.remote, Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Sync(context.Background(), item.id, SyncOptions{Resync: true, Workdir: t.TempDir()}); err == nil || err.Error() != item.want {
			t.Fatalf("%s: %v", item.id, err)
		}
	}
}

type managedOutputReader struct {
	open func() (io.ReadCloser, int64, string, error)
}

func (p managedOutputReader) OpenOutput(context.Context, Space, string) (io.ReadCloser, int64, string, error) {
	return p.open()
}

func TestManagedVaultOutputAdmissionBeforeStaging(t *testing.T) {
	s, _, vault := genericFixture(t)
	metadata := filepath.Join(vault, ".pinax")
	if err := os.Mkdir(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSpace(Space{ID: "output", Remote: "nas", Backend: "generic", LocalRoot: metadata, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	s.Output = managedOutputReader{open: func() (io.ReadCloser, int64, string, error) {
		if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: vault, Remote: "nas", LocalRoot: vault}); err != nil {
			t.Fatal(err)
		}
		body, err := os.Open(filepath.Join(vault, "source.mp4"))
		return body, 7, "", err
	}}
	if err := os.WriteFile(filepath.Join(vault, "source.mp4"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "original-output", SpaceID: "output", Kind: "save_output", LocalPath: ".drivebridge-output/original.mp4", Target: "output.mp4"}
	if err := s.saveOutput(context.Background(), &op); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("output admission: %v", err)
	}
	entries, err := os.ReadDir(metadata)
	if err != nil || len(entries) != 0 {
		t.Fatalf("protected output staging was mutated: %v %v", entries, err)
	}
}

func TestManagedVaultDownloadSkipsProtectedCacheAlias(t *testing.T) {
	s, _, vault := genericFixture(t)
	if _, _, err := s.AdoptStorage(AdoptRequest{Consumer: "pinax", Kind: "local", Space: "vault", Root: vault, Remote: "nas", LocalRoot: vault}); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(vault, ".pinax")
	if err := os.Mkdir(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(metadata, filepath.Join(vault, casRoot)); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "source.md"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := s.List(context.Background(), "vault", "")
	if err != nil || len(files) != 1 {
		t.Fatalf("source: %v %v", files, err)
	}
	op := Operation{ID: "cache-alias", SpaceID: "vault", Kind: "download", LocalPath: "destination.md", FileRef: files[0].Ref}
	if err := s.transfer(context.Background(), &op); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(vault, "destination.md")); err != nil || string(body) != "content" {
		t.Fatalf("ordinary destination must remain usable: %q %v", body, err)
	}
	entries, err := os.ReadDir(metadata)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cache wrote through metadata alias: %v %v", entries, err)
	}
}

func TestManagedVaultWaitingSyncApplyReloadsPlan(t *testing.T) {
	s, local, _, workdir := planFixture(t)
	syncWrite(t, local, "source.md", "real sync content", 0)
	plan, err := s.PlanSync(context.Background(), "paired", PlanSyncOptions{Resync: true, Workdir: workdir})
	if err != nil {
		t.Fatal(err)
	}
	// The persisted plan must never be queried outside writer admission,
	// independently of which concurrent caller the scheduler runs first.
	var unlockedPlanRead atomic.Bool
	const callback = "managed-vault-plan-admission"
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "SyncPlan" && s.guardMu.TryLock() {
			unlockedPlanRead.Store(true)
			s.guardMu.Unlock()
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Callback().Query().Remove(callback) })
	type outcome struct {
		view SyncPlanResult
		err error
	}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			ready <- struct{}{}
			<-start
			view, err := s.ApplySyncPlan(context.Background(), plan.Plan.ID, ApplySyncPlanOptions{ExpectedRevision: plan.Plan.Revision})
			results <- outcome{view, err}
		}()
	}
	<-ready
	<-ready
	close(start)
	first, second := <-results, <-results
	if unlockedPlanRead.Load() {
		t.Fatal("apply read the plan before acquiring writer admission")
	}
	if err := s.DB.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if first.err != nil {
		first, second = second, first
	}
	if first.err != nil || first.view.Plan.Phase != planPhaseCompleted || first.view.Plan.OperationID == "" {
		t.Fatalf("first apply did not complete: %#v %v", first.view, first.err)
	}
	if second.err == nil || second.err.Error() != "plan_not_applicable" || second.view.Plan.OperationID != first.view.Plan.OperationID {
		t.Fatalf("waiting apply lost completed identity: %#v %v", second.view, second.err)
	}
	var stored SyncPlan
	if err := s.DB.First(&stored, "id = ?", plan.Plan.ID).Error; err != nil || stored.Phase != planPhaseCompleted || stored.OperationID != first.view.Plan.OperationID {
		t.Fatalf("waiting apply overwrote completed plan: %#v %v", stored, err)
	}
}
