package service_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceTaskPublicationProgress(t *testing.T) {
	for _, payload := range []string{"", "synthetic task progress"} {
		t.Run(fmt.Sprintf("bytes_%d", len(payload)), func(t *testing.T) {
			f := rig(t)
			h, _ := login(t, f, "alice")
			p := service.Principal{UserID: h.user.ID}
			op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "progress.md", Size: int64(len(payload)), SHA256: service.Hash(payload)}, "task-progress")
			if err != nil {
				t.Fatal(err)
			}
			read := func() contract.TaskProjection {
				t.Helper()
				var page contract.Page[contract.TaskProjection]
				if status := h.call(t, "GET", "/workspace/tasks?kind=operation", nil, &page); status != 200 {
					t.Fatalf("read status %d", status)
				}
				for _, item := range page.Items {
					if item.OwnerID == op.ID {
						return item
					}
				}
				t.Fatal("missing original operation")
				return contract.TaskProjection{}
			}
			before := read()
			if before.Progress.Total == nil || *before.Progress.Total != 1 || before.Progress.Pending == nil || *before.Progress.Pending != 1 || before.Progress.Succeeded != nil {
				t.Fatal("unpublished counters incorrect")
			}
			if len(payload) > 0 {
				if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString(payload)); err != nil {
					t.Fatal(err)
				}
			}
			staged := read()
			if staged.Progress.VerifiedBytes != nil || staged.Progress.DurableBytes != nil || staged.Progress.BytesSent != nil || staged.UIState == "completed" {
				t.Fatal("staged bytes implied publication")
			}
			done, err := f.Service.Complete(t.Context(), p, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			published := read()
			if published.Progress.Succeeded == nil || *published.Progress.Succeeded != 1 || published.Progress.Failed == nil || *published.Progress.Failed != 0 || published.Progress.Pending == nil || *published.Progress.Pending != 0 || published.UIState != "completed" {
				t.Fatal("publication evidence not projected")
			}
			if published.Progress.VerifiedBytes != nil || published.Progress.DurableBytes != nil || published.Progress.BytesSent != nil {
				t.Fatal("metadata read invented physical byte facts")
			}
			// A new head must not replace the fixed version that proves this operation.
			next, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "progress.md", FileID: done.FileID, ExpectedVersion: done.VersionID, Size: 1, SHA256: service.Hash("z")}, "task-progress-next")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.PutPart(p, next.ID, 0, bytes.NewBufferString("z")); err != nil {
				t.Fatal(err)
			}
			if _, err = f.Service.Complete(t.Context(), p, next.ID); err != nil {
				t.Fatal(err)
			}
			afterHead := read()
			if afterHead.Revision != published.Revision || afterHead.Progress.Succeeded == nil {
				t.Fatal("latest head replaced original proof")
			}
			// Corrupt only disposable metadata to check each fixed-output binding.
			var originalVersion service.Version
			if err = f.Service.DB.First(&originalVersion, "id = ?", done.VersionID).Error; err != nil {
				t.Fatal(err)
			}
			for _, patch := range []map[string]any{{"owner": "foreign"}, {"file_id": "foreign"}, {"size": originalVersion.Size + 1}, {"sha256": "different"}, {"connection_id": "different"}, {"object_key": ""}} {
				if err = f.Service.DB.Model(&service.Version{}).Where("id = ?", done.VersionID).Updates(patch).Error; err != nil {
					t.Fatal(err)
				}
				if changed := read(); changed.Progress.Succeeded != nil {
					t.Fatal("mismatched fixed output asserted success")
				}
				if err = f.Service.DB.Save(&originalVersion).Error; err != nil {
					t.Fatal(err)
				}
			}
			var first contract.Page[contract.TaskProjection]
			if status := h.call(t, "GET", "/workspace/tasks?kind=operation&limit=1", nil, &first); status != 200 || first.NextCursor == "" {
				t.Fatal("missing cursor before retention")
			}
			// Disposable fixture only: simulate original-version retention removal.
			if err = f.Service.DB.Delete(&service.Version{}, "id = ?", done.VersionID).Error; err != nil {
				t.Fatal(err)
			}
			if status := h.call(t, "GET", "/workspace/tasks?kind=operation&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, nil); status != 410 {
				t.Fatalf("changed proof cursor status %d", status)
			}
			missing := read()
			if missing.Progress.Succeeded != nil || missing.Progress.Pending != nil || missing.Revision == published.Revision || missing.OwnerStatus != "completed" {
				t.Fatal("lost historical proof fabricated counters or owner failure")
			}
			if err = f.Service.DB.Migrator().DropTable(&service.Version{}); err != nil {
				t.Fatal(err)
			}
			if status := h.call(t, "GET", "/workspace/tasks?kind=operation", nil, nil); status != 500 {
				t.Fatalf("version DB failure status %d", status)
			}
		})
	}
}
