package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

// Pause after the complete byte stream has been consumed. The test controls
// the final Close boundary, not production code or a scheduler timing guess.
type integrityCloseBarrier struct {
	*memoryStore
	entered, release chan struct{}
	closed           atomic.Bool
}
type integrityBarrierBody struct {
	io.ReadCloser
	barrier *integrityCloseBarrier
}

func (s *integrityCloseBarrier) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := s.memoryStore.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return &integrityBarrierBody{ReadCloser: body, barrier: s}, nil
}
func (b *integrityBarrierBody) Close() error {
	err := b.ReadCloser.Close()
	b.barrier.closed.Store(true)
	close(b.barrier.entered)
	<-b.barrier.release
	return err
}

func TestResourceIntegrityPostReadBoundaries(t *testing.T) {
	for _, scenario := range []string{"disabled", "trashed", "descriptor_changed", "path_changed", "cancelled", "unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			s, mem, p := blobRig(t)
			var space Space
			if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
				t.Fatal(err)
			}
			op := uploadBytes(t, s, p, space.ID, "check/original.txt", []byte("synthetic bounded content"))
			descriptor, err := s.RegisterWorkspaceResource(p, RegisterWorkspaceResourceRequest{SpaceID: space.ID, FileRef: s.FileRef(File{ID: op.FileID, VersionID: op.VersionID}), Kind: "template", DisplayName: "Boundary"}, "boundary-register")
			if err != nil {
				t.Fatal(err)
			}
			barrier := &integrityCloseBarrier{memoryStore: mem, entered: make(chan struct{}), release: make(chan struct{})}
			s.storage = func(S3Config, bool) (Storage, error) { return barrier, nil }
			ctx, cancel := context.WithCancel(t.Context())
			var once sync.Once
			type result struct {
				receipt ResourceIntegrityReceipt
				err     error
			}
			done := make(chan result, 1)
			exited := make(chan struct{})
			go func() {
				defer close(exited)
				receipt, err := s.CheckResourceIntegrity(ctx, p, descriptor.DescriptorID, 1)
				done <- result{receipt, err}
			}()
			defer func() { cancel(); once.Do(func() { close(barrier.release) }); <-exited }()
			select {
			case <-barrier.entered:
			case early := <-done:
				t.Fatal("check did not reach close barrier", early.err)
			}
			if !barrier.closed.Load() {
				t.Fatal("underlying stream was not closed")
			}
			switch scenario {
			case "disabled":
				err = s.DB.Model(&User{}).Where("id = ?", p.UserID).Update("disabled", true).Error
			case "trashed":
				err = s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("trashed", true).Error
			case "descriptor_changed":
				err = s.DB.Model(&WorkspaceResourceDescriptor{}).Where("id = ?", descriptor.DescriptorID).Update("display_name", "Changed").Error
			case "path_changed":
				err = s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("path", "check/changed.txt").Error
			case "cancelled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			once.Do(func() { close(barrier.release) })
			observed := <-done
			if scenario == "unchanged" {
				if observed.err != nil || observed.receipt.State != "passed" {
					t.Fatal("unchanged check failed", observed.err)
				}
			} else {
				if observed.err == nil || observed.receipt.CheckID != "" {
					t.Fatal("invalidated check returned a receipt")
				}
				if scenario == "cancelled" && !errors.Is(observed.err, context.Canceled) {
					t.Fatal("cancellation was lost", observed.err)
				}
				if (scenario == "descriptor_changed" || scenario == "path_changed") && observed.err.Error() != "query_changed" {
					t.Fatal("drift was not detected", observed.err)
				}
			}
			want := int64(0)
			if scenario == "unchanged" {
				want = 1
			}
			for _, model := range []any{&WorkspaceResourceCheck{}, &Audit{}} {
				query := s.DB.Model(model)
				if _, ok := model.(*Audit); ok {
					query = query.Where("action = ?", "workspace.resource.check")
				}
				var count int64
				if err = query.Count(&count).Error; err != nil || count != want {
					t.Fatal("post-read boundary left incorrect state", scenario, count, err)
				}
			}
		})
	}
}

func TestResourceIntegrityFileCountBoundary(t *testing.T) {
	s, mem, p := blobRig(t)
	var space Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}
	op := uploadBytes(t, s, p, space.ID, "budget/source.txt", []byte("count fixture"))
	var source File
	var version Version
	if err := s.DB.First(&source, "id = ?", op.FileID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.First(&version, "id = ?", op.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	collection, err := s.CreateFileCollection(p, CreateFileCollectionRequest{SpaceID: space.ID, Name: "Count fixture", Kind: "manual", Scope: "private"}, "count-collection")
	if err != nil {
		t.Fatal(err)
	}
	// Disposable metadata aliases share real fixture bytes. Each entry still
	// has its own file/version identity, and no production data is mutated.
	snapshot := CollectionSnapshot{ID: ID(), Owner: p.UserID, CollectionID: collection.CollectionID, Revision: 1, EntriesTotal: 101, EntriesVisible: 101}
	if err = s.DB.Create(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	var last CollectionSnapshotEntry
	for i := range 101 {
		file := source
		v := version
		file.ID = ID()
		v.ID = ID()
		v.FileID = file.ID
		file.VersionID = v.ID
		file.Path = fmt.Sprintf("budget/member-%03d.txt", i)
		entry := CollectionSnapshotEntry{ID: ID(), SnapshotID: snapshot.ID, FileID: file.ID, VersionID: v.ID, Path: file.Path, SHA256: v.SHA256, Visible: true}
		for _, row := range []any{&file, &v, &entry} {
			if err = s.DB.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
		last = entry
	}
	descriptor, err := s.RegisterWorkspaceResource(p, RegisterWorkspaceResourceRequest{SpaceID: space.ID, SnapshotID: snapshot.ID, Kind: "reference_pack", DisplayName: "Count pack"}, "count-resource")
	if err != nil {
		t.Fatal(err)
	}
	reads := &integrityCountingStore{memoryStore: mem}
	s.storage = func(S3Config, bool) (Storage, error) { return reads, nil }
	if _, err = s.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1); err == nil || err.Error() != "query_budget_exceeded" {
		t.Fatal("101 targets admitted", err)
	}
	if reads.gets.Load() != 0 {
		t.Fatal("over-budget pack read provider bytes")
	}
	var count int64
	if err = s.DB.Model(&WorkspaceResourceCheck{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("over-budget pack persisted a result", err)
	}
	if err = s.DB.Delete(&last).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Model(&snapshot).Updates(map[string]any{"entries_total": 100, "entries_visible": 100}).Error; err != nil {
		t.Fatal(err)
	}
	receipt, err := s.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || receipt.State != "passed" || reads.gets.Load() != 100 {
		t.Fatal("100-target boundary did not read every member", err, reads.gets.Load())
	}
	lookup, err := s.LookupResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || lookup != receipt || reads.gets.Load() != 100 {
		t.Fatal("lookup read provider bytes", err)
	}
	replay, err := s.CheckResourceIntegrity(t.Context(), p, descriptor.DescriptorID, 1)
	if err != nil || replay != receipt || reads.gets.Load() != 100 {
		t.Fatal("receipt replay unexpectedly read storage", err)
	}
}

type integrityCountingStore struct {
	*memoryStore
	gets atomic.Int64
}

func (s *integrityCountingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.gets.Add(1)
	return s.memoryStore.Get(ctx, key)
}
