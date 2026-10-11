package service_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

// A trash committing while a move is in flight must survive it: the move
// writes only path columns and refuses once the file is trashed, so a
// successful trash is never silently undone.
func TestMoveDoesNotResurrectTrashedFile(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	for i := 0; i < 30; i++ {
		body := []byte(fmt.Sprintf("move-race-%d", i))
		sum := sha256.Sum256(body)
		op, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: fmt.Sprintf("race-%d.bin", i), Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}, fmt.Sprintf("move-race-%d", i))
		if e != nil {
			t.Fatal(e)
		}
		if op.Status != "completed" {
			if _, e = f.Service.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
				t.Fatal(e)
			}
			if op, e = f.Service.Complete(t.Context(), p, op.ID); e != nil {
				t.Fatal(e)
			}
		}
		move := func() bool {
			payload, _ := json.Marshal(map[string]any{"path": fmt.Sprintf("moved-%d.bin", i), "expected_version": op.VersionID})
			req, _ := http.NewRequest("POST", h.base+"/api/v1/files/"+op.FileID+"/move", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", h.base)
			req.Header.Set("X-CSRF-Token", h.csrf)
			res, e := h.http.Do(req)
			if e != nil {
				return false
			}
			res.Body.Close()
			return res.StatusCode == 200
		}
		stop := make(chan struct{})
		var wg sync.WaitGroup
		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						move()
					}
				}
			}()
		}
		trashErr := f.Service.Trash(p, op.FileID, false)
		close(stop)
		wg.Wait()
		if trashErr != nil {
			continue
		}
		var row service.File
		if e := f.Service.DB.First(&row, "id = ?", op.FileID).Error; e != nil {
			t.Fatalf("iteration %d: trashed file vanished: %v", i, e)
		}
		if !row.Trashed {
			t.Fatalf("iteration %d: move resurrected the trashed file", i)
		}
	}
}
