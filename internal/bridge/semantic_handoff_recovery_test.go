package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Keep production List's basename contract; only inject cancellation at the
// readback boundary after the real rclone alias has committed the nested file.
type semanticAliasCommitProvider struct {
	Provider
	committed chan struct{}
	lostResponse bool
}

func (p semanticAliasCommitProvider) Upload(ctx context.Context, space Space, target string, source io.Reader, size int64) (File, error) {
	file, err := p.Provider.Upload(ctx, space, target, source, size)
	close(p.committed)
	<-ctx.Done()
	if p.lostResponse && err == nil {
		return File{}, ctx.Err()
	}
	return file, err
}

func TestSemanticHandoffNestedAliasCommitResumeAndReconciliation(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "known_commit", true: "lost_response"}[lost], func(t *testing.T) {
			s, local, remoteRoot := genericFixture(t)
			if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(remoteRoot, "nested"), 0700); err != nil {
				t.Fatal(err)
			}
			provider := s.Generic
			committed := make(chan struct{})
			s.Generic = semanticAliasCommitProvider{Provider: provider, committed: committed, lostResponse: lost}
			op, err := s.Upload(context.Background(), "nas", "nested", fixture(t, local), "reference.png", "nested-commit")
			if err != nil {
				t.Fatal(err)
			}
			<-committed
			if _, err := s.Cancel(op.ID); err != nil {
				t.Fatal(err)
			}
			op = waitOp(t, s, op.ID)
			if op.Status != "interrupted" || op.Error != "resume_required" || op.FileRef == "" {
				t.Fatalf("nested committed target treated as absent: %+v", op)
			}
			s.wg.Wait()
			s.Generic = provider
			if _, err := s.Resume(op.ID); err != nil {
				t.Fatal(err)
			}
			recovered := waitOp(t, s, op.ID)
			if recovered.Status != "completed" || recovered.ID != op.ID || recovered.FileRef != op.FileRef || recovered.ContentSHA256 == "" {
				t.Fatalf("nested immutable commit failed recovery: %+v", recovered)
			}
			entries, err := os.ReadDir(filepath.Join(remoteRoot, "nested"))
			if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(op.Target) {
				t.Fatal("nested recovery changed or duplicated target", err)
			}
		})
	}
}

func (p semanticAliasCommitProvider) Open(ctx context.Context, space Space, file File) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.Provider.Open(ctx, space, file)
}

type semanticResumeGateProvider struct {
	*fakeProvider
	entered chan struct{}
}

func (p semanticResumeGateProvider) List(ctx context.Context, space Space, dir string) ([]File, error) {
	close(p.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSemanticHandoffUnknownResumeRecancelPreservesUncertainty(t *testing.T) {
	s, p, local, _ := setup(t)
	source := fixture(t, local)
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	digest, size, err := digestReader(input)
	input.Close()
	if err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: newID(), Key: "unknown-recancel", SpaceID: "creative", Kind: "upload", Status: "interrupted", Error: "resume_required", LocalPath: "reference.png", Fingerprint: digest + ":reference.png", Bytes: size}
	op.Target = op.ID + "-reference.png"
	if err := s.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	s.Provider = semanticResumeGateProvider{fakeProvider: p, entered: entered}
	queued, err := s.Resume(op.ID)
	if err != nil || queued.Error != "resume_required" || queued.FileRef != "" {
		t.Fatalf("Resume erased unresolved write: %+v %v", queued, err)
	}
	<-entered
	if _, err := s.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	terminal := waitOp(t, s, op.ID)
	if terminal.Status != "interrupted" || terminal.Error != "resume_required" || terminal.FileRef != "" {
		t.Fatalf("recancel before reconciliation claimed rollback: %+v", terminal)
	}
	s.wg.Wait()
	// Exercise the pre-worker branch deterministically with the exact queued
	// snapshot returned by Resume, before any provider reconciliation can run.
	if err := s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "cancelling", "error": queued.Error}).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.run(ctx, queued)
	terminal, err = s.Get(op.ID)
	if err != nil || terminal.Status != "interrupted" || terminal.Error != "resume_required" || terminal.FileRef != "" {
		t.Fatalf("cancel before worker start claimed rollback: %+v %v", terminal, err)
	}
}
