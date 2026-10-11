package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestEmptyFileUsesUploadAndEncryptedVerification(t *testing.T) {
	f := rig(t)
	human, _ := login(t, f, "empty")
	digest := sha256.Sum256(nil)
	for _, encrypted := range []bool{false, true} {
		space := human.space
		if encrypted {
			space = encryptedSpace(t, human, "Empty encrypted")
		}
		principal := service.Principal{UserID: human.user.ID}
		op, err := f.Service.CreateUpload(principal, service.UploadRequest{SpaceID: space.ID, Path: "empty.txt", Size: 0, SHA256: hex.EncodeToString(digest[:])}, service.ID())
		if err != nil {
			t.Fatal("empty upload creation", err)
		}
		op, err = f.Service.Complete(context.Background(), principal, op.ID)
		if err != nil || op.Status != "completed" {
			t.Fatal("empty upload completion", err)
		}
		var file service.File
		f.Service.DB.First(&file, "id = ?", op.FileID)
		if file.Size != 0 {
			t.Fatal("empty file was padded")
		}
		response, err := human.http.Get(human.base + "/api/v1/files/" + file.ID + "/content?version=" + file.VersionID)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || len(body) != 0 {
			t.Fatal("empty download", response.StatusCode, len(body))
		}
		response, err = human.http.Get(human.base + "/api/v1/files/" + file.ID + "/media?version=" + file.VersionID)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || len(body) != 0 {
			t.Fatal("empty media preview", response.StatusCode, len(body))
		}
		if _, err = f.Service.CreateUpload(principal, service.UploadRequest{SpaceID: space.ID, Path: "empty.txt", Size: 0, SHA256: hex.EncodeToString(digest[:])}, service.ID()); err == nil {
			t.Fatal("duplicate creation silently overwrote a file")
		}
	}
}
