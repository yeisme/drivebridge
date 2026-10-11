package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// A deterministic storage barrier, not a production cancellation hook. Both
// variants wait until Cancel actually interrupts Complete before proceeding.
type batchCancelBarrierStore struct {
	*memoryStore
	entered, interrupted, release chan struct{}
	honorCancellation             bool
}

func (b *batchCancelBarrierStore) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	close(b.entered)
	<-ctx.Done()
	close(b.interrupted)
	<-b.release
	if b.honorCancellation {
		return ctx.Err()
	}
	// A provider can finish work despite cancellation; metadata publication is
	// then an original-operation side effect, never a rolled-back file.
	return b.memoryStore.Put(ctx, key, reader, size)
}

func TestUploadBatchCancelCompletionInterleavings(t *testing.T) {
	for _, honor := range []bool{true, false} {
		name := "provider_finishes_after_interrupt"
		if honor {
			name = "provider_honors_interrupt"
		}
		t.Run(name, func(t *testing.T) {
			s, mem, p := blobRig(t)
			var space Space
			if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
				t.Fatal(err)
			}
			batch, err := s.CreateUploadBatch(p, UploadBatchRequest{SpaceID: space.ID}, "interleaving-batch")
			if err != nil {
				t.Fatal(err)
			}
			inputs := []UploadBatchItemInput{{RelativePath: "active.txt", Size: 1, SHA256: Hash("a")}, {RelativePath: "pending.txt", Size: 1, SHA256: Hash("b")}}
			if _, err = s.AddUploadBatchItems(p, batch.BatchID, inputs); err != nil {
				t.Fatal(err)
			}
			if _, err = s.SealUploadBatch(p, batch.BatchID); err != nil {
				t.Fatal(err)
			}
			create := func(i int, key string) (Operation, error) {
				item := inputs[i]
				return s.CreateUpload(p, UploadRequest{SpaceID: space.ID, Path: item.RelativePath, Size: item.Size, SHA256: item.SHA256, BatchID: batch.BatchID, ItemKey: item.RelativePath + "\x00" + item.SHA256}, key)
			}
			active, err := create(0, "active")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.PutPart(p, active.ID, 0, bytes.NewBufferString("a")); err != nil {
				t.Fatal(err)
			}
			barrier := &batchCancelBarrierStore{memoryStore: mem, entered: make(chan struct{}), interrupted: make(chan struct{}), release: make(chan struct{}), honorCancellation: honor}
			s.storage = func(S3Config, bool) (Storage, error) { return barrier, nil }
			completeCtx, cancelComplete := context.WithCancel(t.Context())
			var release sync.Once
			type result struct {
				operation Operation
				err       error
			}
			completed := make(chan result, 1)
			completeExited := make(chan struct{})
			go func() {
				defer close(completeExited)
				op, err := s.Complete(completeCtx, p, active.ID)
				completed <- result{op, err}
			}()
			// Join workers before fixture teardown, including failed assertions. The
			// go test process timeout bounds a real lock-order regression.
			defer func() { cancelComplete(); release.Do(func() { close(barrier.release) }); <-completeExited }()
			<-barrier.entered
			cancelled := make(chan error, 1)
			cancelExited := make(chan struct{})
			go func() {
				defer close(cancelExited)
				_, err := s.ControlUploadBatch(p, batch.BatchID, "cancel")
				cancelled <- err
			}()
			defer func() { cancelComplete(); release.Do(func() { close(barrier.release) }); <-cancelExited }()
			<-barrier.interrupted
			// Complete still owns the operation lock while Cancel waits. Admission
			// must already be durable, and unrelated scheduling cannot wait on it.
			var observed UploadBatch
			if err = s.DB.First(&observed, "id = ?", batch.BatchID).Error; err != nil {
				t.Fatal(err)
			}
			if observed.Phase != "cancelled" {
				t.Fatal("cleanup started before admission closed")
			}
			if _, err = create(1, "late-item"); err == nil || err.Error() != "operation_closed" {
				t.Fatal("new scheduling admitted while cancellation waits")
			}
			select {
			case <-cancelled:
				t.Fatal("cancel returned before child settled")
			default:
			}
			release.Do(func() { close(barrier.release) })
			done := <-completed
			if err = <-cancelled; err != nil {
				t.Fatal(err)
			}
			var stored Operation
			if err = s.DB.First(&stored, "id = ?", active.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err = s.DB.First(&space, "id = ?", space.ID).Error; err != nil {
				t.Fatal(err)
			}
			if space.Reserved != 0 {
				t.Fatal("reservation was not settled exactly once")
			}
			var files, versions int64
			if err = s.DB.Model(&File{}).Where("space_id = ?", space.ID).Count(&files).Error; err != nil {
				t.Fatal(err)
			}
			if err = s.DB.Model(&Version{}).Where("id = ?", active.VersionID).Count(&versions).Error; err != nil {
				t.Fatal(err)
			}
			if honor {
				if !errors.Is(done.err, context.Canceled) || stored.Status != "cancelled" || files != 0 || versions != 0 || space.Used != 0 {
					t.Fatal("interrupted provider published a file or lost cancellation")
				}
			} else {
				if done.err != nil || stored.Status != "completed" || files != 1 || versions != 1 || space.Used != 1 || stored.VersionID != active.VersionID {
					t.Fatal("successful publication was duplicated or overwritten by cancel")
				}
				_, _, body, err := s.Download(t.Context(), p, stored.FileID, stored.VersionID)
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(body)
				body.Close()
				if readErr != nil || string(data) != "a" {
					t.Fatal("completed side effect not readable")
				}
			}
			if _, err = os.Stat(filepath.Join(s.Config.State, "uploads", active.ID)); !os.IsNotExist(err) {
				t.Fatal("staged parts remain after terminal settlement")
			}
			final, err := s.GetUploadBatch(p, batch.BatchID, "", 50)
			if err != nil {
				t.Fatal(err)
			}
			wantCompleted := int64(0)
			if !honor {
				wantCompleted = 1
			}
			if final.Phase != "cancelled" || final.ItemsWaiting != 0 || final.ItemsCompleted != wantCompleted {
				t.Fatal("batch concealed settled child outcome")
			}
			again, err := s.ControlUploadBatch(p, batch.BatchID, "cancel")
			if err != nil || again.BatchRevision != final.BatchRevision {
				t.Fatal("retry changed original cancellation identity")
			}
		})
	}
}
